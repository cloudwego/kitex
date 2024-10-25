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
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/netpoll"
	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var _ transPool = (*muxTransPool)(nil)

type muxTransList struct {
	L          sync.RWMutex
	size       int
	cursor     uint32
	transports []*transport
}

func newMuxTransList(size int) *muxTransList {
	tl := new(muxTransList)
	tl.size = size
	tl.transports = make([]*transport, size)
	return tl
}

func (tl *muxTransList) Get(sinfo *serviceinfo.ServiceInfo, network string, addr string) (*transport, error) {
	idx := atomic.AddUint32(&tl.cursor, 1) % uint32(tl.size)
	tl.L.RLock()
	trans := tl.transports[idx]
	tl.L.RUnlock()
	if trans != nil && trans.IsActive() {
		return trans, nil
	}

	conn, err := netpoll.DialConnection(network, addr, time.Second)
	if err != nil {
		return nil, err
	}
	klog.Infof("Dial connection, addr: %s", addr)
	trans = newTransport(clientTransport, sinfo, conn)
	_ = conn.AddCloseCallback(func(connection netpoll.Connection) error {
		// peer close
		_ = trans.Close(nil)
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
}

func newMuxTransPool() transPool {
	t := new(muxTransPool)
	t.poolSize = runtime.GOMAXPROCS(0)
	return t
}

type muxTransPool struct {
	poolSize int
	pool     sync.Map // addr:*muxTransList
	sflight  singleflight.Group
}

func (m *muxTransPool) Get(sinfo *serviceinfo.ServiceInfo, network string, addr string) (trans *transport, err error) {
	v, ok := m.pool.Load(addr)
	if ok {
		return v.(*muxTransList).Get(sinfo, network, addr)
	}

	v, err, _ = m.sflight.Do(addr, func() (interface{}, error) {
		transList := newMuxTransList(m.poolSize)
		m.pool.Store(addr, transList)
		return transList, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*muxTransList).Get(sinfo, network, addr)
}

func (m *muxTransPool) Put(trans *transport) {
	// do nothing
}
