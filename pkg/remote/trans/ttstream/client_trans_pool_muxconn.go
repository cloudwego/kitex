/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ttstream

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/gofunc"
)

var DefaultMuxConnConfig = MuxConnConfig{
	PoolSize:       runtime.GOMAXPROCS(0),
	MaxIdleTimeout: time.Minute,
}

type MuxConnConfig struct {
	PoolSize       int
	MaxIdleTimeout time.Duration
}

var _ transPool = (*muxConnTransPool)(nil)

var errMuxConnListClosed = errors.New("mux connection list is closed")

type muxConnTransList struct {
	L          sync.RWMutex
	size       int
	cursor     uint32
	transports []*clientTransport
	pool       *muxConnTransPool
	// A reservation spans Get through CloseStream, including the interval before
	// WriteStream registers the stream on its transport.
	inUse      int32
	lastActive time.Time
	closed     bool
}

func newMuxConnTransList(size int, pool *muxConnTransPool) *muxConnTransList {
	if size <= 0 {
		size = runtime.GOMAXPROCS(0)
	}
	return &muxConnTransList{size: size, transports: make([]*clientTransport, size), pool: pool}
}

func (tl *muxConnTransList) Get(network, addr string) (*clientTransport, error) {
	tl.L.RLock()
	if tl.closed {
		tl.L.RUnlock()
		return nil, errMuxConnListClosed
	}
	idx := atomic.AddUint32(&tl.cursor, 1) % uint32(tl.size)
	atomic.AddInt32(&tl.inUse, 1)
	trans := tl.transports[idx]
	if trans != nil && trans.IsActive() {
		tl.L.RUnlock()
		return trans, nil
	}
	tl.L.RUnlock()

	// Keep the reservation while upgrading to the write lock so retirement
	// cannot close a list between selection and transport acquisition.
	tl.L.Lock()
	if tl.closed {
		atomic.AddInt32(&tl.inUse, -1)
		tl.L.Unlock()
		return nil, errMuxConnListClosed
	}
	trans = tl.transports[idx]
	if trans != nil && trans.IsActive() {
		tl.L.Unlock()
		return trans, nil
	}
	discard := trans
	conn, err := dialer.DialConnection(network, addr, time.Second)
	if err != nil {
		atomic.AddInt32(&tl.inUse, -1)
		tl.L.Unlock()
		if discard != nil {
			_ = discard.Close(nil)
		}
		return nil, err
	}
	trans = newClientTransport(conn, tl)
	_ = conn.AddCloseCallback(func(connection netpoll.Connection) error {
		_ = trans.Close(errTransport.newBuilder().withCause(errors.New("connection closed by peer")))
		return nil
	})
	tl.transports[idx] = trans
	tl.L.Unlock()
	if discard != nil {
		_ = discard.Close(nil)
	}
	return trans, nil
}

func (tl *muxConnTransList) Put(trans *clientTransport) {
	tl.L.Lock()
	atomic.AddInt32(&tl.inUse, -1)
	tl.lastActive = time.Now()
	tl.L.Unlock()
	tl.pool.startCleaner(trans)
}

// retireIfIdle excludes both active streams and concurrent Get reservations.
// A caller that already loaded this list must retry against the pool after retirement.
func (tl *muxConnTransList) retireIfIdle(now time.Time, timeout time.Duration) ([]*clientTransport, bool) {
	tl.L.Lock()
	defer tl.L.Unlock()
	if tl.closed || atomic.LoadInt32(&tl.inUse) != 0 || tl.lastActive.IsZero() || now.Sub(tl.lastActive) < timeout {
		return nil, false
	}
	tl.closed = true
	transports := tl.transports
	tl.transports = nil
	return transports, true
}

func newMuxConnTransPool(config MuxConnConfig) transPool {
	return &muxConnTransPool{config: config}
}

type muxConnTransPool struct {
	config      MuxConnConfig
	pool        sync.Map // addr:*muxConnTransList
	cleanerOnce int32
}

func (p *muxConnTransPool) Get(network, addr string) (trans *clientTransport, err error) {
	for {
		v, ok := p.pool.Load(addr)
		if !ok {
			v, _ = p.pool.LoadOrStore(addr, newMuxConnTransList(p.config.PoolSize, p))
		}
		trans, err = v.(*muxConnTransList).Get(network, addr)
		if err != errMuxConnListClosed {
			return trans, err
		}
	}
}

func (p *muxConnTransPool) Put(trans *clientTransport) {
	trans.pool.(*muxConnTransList).Put(trans)
}

func (p *muxConnTransPool) cleanIdle(now time.Time) {
	p.pool.Range(func(key, value interface{}) bool {
		transports, retired := value.(*muxConnTransList).retireIfIdle(now, p.config.MaxIdleTimeout)
		if !retired {
			return true
		}
		p.pool.Delete(key)
		// Do not hold the list lock while closing: transport cleanup can return streams.
		for _, trans := range transports {
			if trans != nil {
				_ = trans.Close(nil)
			}
		}
		return true
	})
}

func (p *muxConnTransPool) startCleaner(trans *clientTransport) {
	idleTimeout := p.config.MaxIdleTimeout
	if idleTimeout == 0 || !atomic.CompareAndSwapInt32(&p.cleanerOnce, 0, 1) {
		return
	}
	gofunc.RecoverGoFuncWithInfo(context.Background(), func() {
		for {
			p.cleanIdle(time.Now())
			time.Sleep(idleTimeout)
		}
	}, gofunc.NewBasicInfo("", trans.Addr().String()))
}
