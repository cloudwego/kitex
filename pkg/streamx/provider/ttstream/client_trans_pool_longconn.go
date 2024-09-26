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
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
	"github.com/cloudwego/netpoll"
)

const idleCheckInternal = time.Second * 10

var DefaultLongConnConfig = LongConnConfig{
	MaxIdleTimeout: time.Minute,
}

type LongConnConfig struct {
	MaxIdleTimeout time.Duration
}

func newLongConnTransPool(config LongConnConfig) transPool {
	tp := new(longConnTransPool)
	tp.config = DefaultLongConnConfig
	if config.MaxIdleTimeout > 0 {
		tp.config.MaxIdleTimeout = config.MaxIdleTimeout
	}
	tp.transPool = container.NewObjectPool(tp.config.MaxIdleTimeout, idleCheckInternal)
	return tp
}

type longConnTransPool struct {
	transPool *container.ObjectPool
	config    LongConnConfig
}

func (c *longConnTransPool) Get(sinfo *serviceinfo.ServiceInfo, network string, addr string) (trans *transport, err error) {
	o := c.transPool.Pop(addr)
	if o != nil {
		return o.(*transport), nil
	}

	// create new connection
	conn, err := netpoll.DialConnection(network, addr, time.Second)
	if err != nil {
		return nil, err
	}
	// create new transport
	trans = newTransport(clientTransport, sinfo, conn)
	return trans, nil
}

func (c *longConnTransPool) Put(trans *transport) {
	addr := trans.conn.RemoteAddr().String()
	c.transPool.Push(addr, trans)
}
