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
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/netpoll"
	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	terrors "github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/errors"
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

type muxConnTransList struct {
	L          sync.RWMutex
	size       int
	cursor     uint32
	transports []*transport
	pool       transPool
	sf         singleflight.Group
}

func newMuxConnTransList(size int, pool transPool) *muxConnTransList {
	tl := new(muxConnTransList)
	if size == 0 {
		size = runtime.GOMAXPROCS(0)
	}
	tl.size = size
	tl.transports = make([]*transport, size)
	tl.pool = pool
	return tl
}

func (tl *muxConnTransList) Close() {
	tl.L.Lock()
	for i, t := range tl.transports {
		_ = t.Close(nil)
		tl.transports[i] = nil
	}
	tl.L.Unlock()
}

func (tl *muxConnTransList) Get(sinfo *serviceinfo.ServiceInfo, network, addr string) (*transport, error) {
	idx := atomic.AddUint32(&tl.cursor, 1) % uint32(tl.size)
	tl.L.RLock()
	trans := tl.transports[idx]
	tl.L.RUnlock()
	if trans != nil && trans.IsActive() {
		return trans, nil
	}

	v, err, _ := tl.sf.Do(fmt.Sprintf("%d", idx), func() (interface{}, error) {
		conn, err := dialer.DialConnection(network, addr, time.Second)
		if err != nil {
			return nil, err
		}
		trans := newTransport(clientTransport, sinfo, conn, tl.pool)
		_ = conn.AddCloseCallback(func(connection netpoll.Connection) error {
			// peer close
			_ = trans.Close(terrors.ErrTransport.WithCause(errors.New("connection closed by peer")))
			return nil
		})
		runtime.SetFinalizer(trans, func(trans *transport) {
			// self close when not hold by user
			_ = trans.Close(nil)
		})
		tl.L.Lock()
		tl.transports[idx] = trans
		tl.L.Unlock()
		return trans, nil
	})
	if err != nil {
		return nil, err
	}
	trans = v.(*transport)
	return trans, nil
}

func newMuxConnTransPool(config MuxConnConfig) transPool {
	t := new(muxConnTransPool)
	t.config = config
	return t
}

type muxConnTransPool struct {
	config      MuxConnConfig
	pool        sync.Map // addr:*muxConnTransList
	activity    sync.Map // addr:lastActive
	sflight     singleflight.Group
	cleanerOnce sync.Once
}

func (p *muxConnTransPool) Get(sinfo *serviceinfo.ServiceInfo, network, addr string) (trans *transport, err error) {
	v, ok := p.pool.Load(addr)
	if ok {
		return v.(*muxConnTransList).Get(sinfo, network, addr)
	}

	v, err, _ = p.sflight.Do(addr, func() (interface{}, error) {
		transList := newMuxConnTransList(p.config.PoolSize, p)
		p.pool.Store(addr, transList)
		return transList, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*muxConnTransList).Get(sinfo, network, addr)
}

func (p *muxConnTransPool) Put(trans *transport) {
	p.activity.Store(trans.conn.RemoteAddr().String(), time.Now())
	p.cleanerOnce.Do(func() {
		internal := p.config.MaxIdleTimeout
		if internal == 0 {
			return
		}
		go func() {
			for {
				now := time.Now()
				count := 0
				p.activity.Range(func(addr, value interface{}) bool {
					count++
					lastActive := value.(time.Time)
					if lastActive.IsZero() || now.Sub(lastActive) < p.config.MaxIdleTimeout {
						return true
					}
					v, _ := p.pool.Load(addr)
					if v == nil {
						return true
					}
					transList := v.(*muxConnTransList)
					p.pool.Delete(addr)
					p.activity.Delete(addr)
					transList.Close()
					return true
				})
				time.Sleep(internal)
			}
		}()
	})
}
