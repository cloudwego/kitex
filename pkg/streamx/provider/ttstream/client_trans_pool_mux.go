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
	"time"

	"github.com/cloudwego/netpoll"
	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var _ transPool = (*muxTransPool)(nil)

func newMuxTransPool() transPool {
	t := new(muxTransPool)
	return t
}

type muxTransPool struct {
	pool    sync.Map // addr:*transport
	sflight singleflight.Group
}

func (m *muxTransPool) Get(sinfo *serviceinfo.ServiceInfo, network string, addr string) (trans *transport, err error) {
	v, ok := m.pool.Load(addr)
	if ok {
		return v.(*transport), nil
	}
	v, err, _ = m.sflight.Do(addr, func() (interface{}, error) {
		conn, err := netpoll.DialConnection(network, addr, time.Second)
		if err != nil {
			return nil, err
		}
		trans = newTransport(clientTransport, sinfo, conn)
		_ = conn.AddCloseCallback(func(connection netpoll.Connection) error {
			// peer close
			_ = trans.Close()
			return nil
		})
		m.pool.Store(addr, trans)
		runtime.SetFinalizer(trans, func(trans *transport) {
			// self close when not hold by user
			_ = trans.Close()
		})
		return trans, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*transport), nil
}

func (m *muxTransPool) Put(trans *transport) {
	// do nothing
}
