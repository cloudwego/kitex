/*
 * Copyright 2026 CloudWeGo Authors
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
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/test"
)

type muxIdleTestConn struct {
	netpoll.Connection
	closed int32
}

func (c *muxIdleTestConn) IsActive() bool      { return atomic.LoadInt32(&c.closed) == 0 }
func (c *muxIdleTestConn) Close() error        { atomic.StoreInt32(&c.closed, 1); return nil }
func (c *muxIdleTestConn) LocalAddr() net.Addr { return &net.TCPAddr{Port: 1234} }

func newMuxIdleTestPool() (*muxConnTransPool, *muxConnTransList, *clientTransport, *muxIdleTestConn) {
	p := newMuxConnTransPool(MuxConnConfig{PoolSize: 1, MaxIdleTimeout: time.Second}).(*muxConnTransPool)
	// Exercise cleanup synchronously without leaving a background cleaner in a test.
	p.cleanerOnce = 1
	tl := newMuxConnTransList(1, p)
	conn := &muxIdleTestConn{}
	trans := &clientTransport{conn: conn, pool: tl}
	tl.transports[0] = trans
	p.pool.Store("target", tl)
	return p, tl, trans, conn
}

func TestMuxConnIdleEvictionClosesTransport(t *testing.T) {
	p, tl, trans, conn := newMuxIdleTestPool()
	got, err := p.Get("tcp", "target")
	test.Assert(t, err == nil && got == trans, err)
	trans.pool.Put(trans)
	p.cleanIdle(time.Now().Add(2 * time.Second))
	_, exists := p.pool.Load("target")
	test.Assert(t, !exists, "idle list must be removed")
	test.Assert(t, !conn.IsActive(), "evicted transport must be closed")
	test.Assert(t, trans.state == connStateClosed, "transport cleanup must run")
	_, err = tl.Get("tcp", "target")
	test.Assert(t, err == errMuxConnListClosed, "a stale list must not create an orphan transport")
}

func TestMuxConnIdleEvictionPreservesReservations(t *testing.T) {
	p, _, trans, conn := newMuxIdleTestPool()
	// Keep one acquired transport pending before WriteStream. Another stream's
	// completion must not make the list eligible for eviction.
	_, err := p.Get("tcp", "target")
	test.Assert(t, err == nil, err)
	_, err = p.Get("tcp", "target")
	test.Assert(t, err == nil, err)
	trans.pool.Put(trans)
	p.cleanIdle(time.Now().Add(2 * time.Second))
	_, exists := p.pool.Load("target")
	test.Assert(t, exists && conn.IsActive(), "outstanding Get must protect the list")
	trans.pool.Put(trans)
	p.cleanIdle(time.Now())
	test.Assert(t, conn.IsActive(), "last return restarts the idle interval")
	p.cleanIdle(time.Now().Add(2 * time.Second))
	test.Assert(t, !conn.IsActive(), "all returned transports must eventually close")
}

func TestMuxConnConcurrentGetAndRetirement(t *testing.T) {
	for i := 0; i < 100; i++ {
		p, tl, trans, conn := newMuxIdleTestPool()
		_, err := tl.Get("tcp", "target")
		test.Assert(t, err == nil, err)
		tl.Put(trans)
		now := time.Now().Add(2 * time.Second)
		var wg sync.WaitGroup
		wg.Add(2)
		var acquired bool
		go func() {
			defer wg.Done()
			_, getErr := tl.Get("tcp", "target")
			acquired = getErr == nil
		}()
		go func() { defer wg.Done(); p.cleanIdle(now) }()
		wg.Wait()
		if acquired {
			test.Assert(t, conn.IsActive(), "retirement must not close a concurrent reservation")
			tl.Put(trans)
			p.cleanIdle(time.Now().Add(2 * time.Second))
		}
		test.Assert(t, !conn.IsActive(), "idle transport must be released")
	}
}
