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
	"time"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/netpoll"
)

func newShortConnTransPool() transPool {
	return &shortConnTransPool{}
}

type shortConnTransPool struct {
}

func (c *shortConnTransPool) Get(sinfo *serviceinfo.ServiceInfo, network string, addr string) (*transport, error) {
	// create new connection
	conn, err := netpoll.DialConnection(network, addr, time.Second)
	if err != nil {
		return nil, err
	}
	// create new transport
	trans := newTransport(clientTransport, sinfo, conn)
	return trans, nil
}

func (c *shortConnTransPool) Put(trans *transport) {
	_ = trans.Close()
}
