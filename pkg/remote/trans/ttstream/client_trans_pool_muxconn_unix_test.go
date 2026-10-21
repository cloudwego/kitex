//go:build !windows

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestMuxConnStreamLifecycle(t *testing.T) {
	p, tl, _, _ := newMuxIdleTestPool()
	cconn, sconn := newTestConnectionPipe(t)
	defer sconn.Close()
	trans := newClientTransport(cconn, tl)
	defer trans.Close(nil)
	tl.transports[0] = trans
	ctx, ri := newTestRPCInfoCtx(nil)
	p.pool.Delete("target")
	p.pool.Store(ri.To().Address().String(), tl)
	handler := &clientTransHandler{transPool: p}
	first, err := handler.NewStream(ctx, ri)
	test.Assert(t, err == nil, err)
	second, err := handler.NewStream(ctx, ri)
	test.Assert(t, err == nil, err)
	first.(*clientStream).close(nil, false, "", nil)
	p.cleanIdle(time.Now().Add(2 * time.Second))
	test.Assert(t, trans.IsActive(), "a long stream must survive another stream's idle timeout")
	second.(*clientStream).close(nil, false, "", nil)
	p.cleanIdle(time.Now().Add(2 * time.Second))
	test.Assert(t, !trans.IsActive(), "last completed stream allows transport cleanup")
	trans.WaitClosed()
}

func TestMuxConnFailedStreamReleasesReservation(t *testing.T) {
	p, tl, trans, _ := newMuxIdleTestPool()
	ctx, ri := newTestRPCInfoCtx(nil)
	p.pool.Delete("target")
	p.pool.Store(ri.To().Address().String(), tl)
	// Simulate a peer close after acquisition, before WriteStream registers it.
	pool := &muxCloseAfterGetPool{pool: p}
	handler := &clientTransHandler{transPool: pool}
	_, err := handler.NewStream(ctx, ri)
	test.Assert(t, err != nil, "closed transport must reject stream creation")
	test.Assert(t, atomic.LoadInt32(&tl.inUse) == 0, "failed creation must return its reservation")
	count := 0
	trans.streams.Range(func(_, _ interface{}) bool { count++; return true })
	test.Assert(t, count == 0, "failed stream must be removed")
}

type muxCloseAfterGetPool struct{ pool *muxConnTransPool }

func (p *muxCloseAfterGetPool) Get(network, addr string) (*clientTransport, error) {
	trans, err := p.pool.Get(network, addr)
	if err == nil {
		_ = trans.Close(errTransport.newBuilder())
	}
	return trans, err
}
func (p *muxCloseAfterGetPool) Put(trans *clientTransport) { p.pool.Put(trans) }
