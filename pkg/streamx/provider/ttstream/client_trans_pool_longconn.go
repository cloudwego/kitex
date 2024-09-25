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
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
	"github.com/cloudwego/netpoll"
)

var DefaultLongConnConfig = LongConnConfig{
	MaxIdleTimeout: time.Minute,
}

type LongConnConfig struct {
	MaxIdleTimeout time.Duration
}

func newLongConnTransPool(config LongConnConfig) transPool {
	tp := new(longConnTransPool)
	if config.MaxIdleTimeout == 0 {
		config.MaxIdleTimeout = DefaultLongConnConfig.MaxIdleTimeout
	}
	tp.transPool = container.NewObjectPool(time.Second * 10)
	tp.config = config
	return tp
}

type longConnTransPool struct {
	transPool *container.ObjectPool
	config    LongConnConfig
}

func (c *longConnTransPool) Get(sinfo *serviceinfo.ServiceInfo, network string, addr string) (trans *transport, err error) {
	o := c.transPool.Pop(addr)
	if o != nil {
		tw := o.(*transWrapper)
		trans = tw.transport
		tw.release()
		return trans, nil
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
	tw := newTransWrapper(trans, c.config.MaxIdleTimeout)
	c.transPool.Push(addr, tw)
}

var transWrapperPool sync.Pool

func newTransWrapper(t *transport, idleTimeout time.Duration) (tw *transWrapper) {
	v := transWrapperPool.Get()
	if v == nil {
		tw = new(transWrapper)
	} else {
		tw = v.(*transWrapper)
	}
	tw.transport = t
	tw.lastActive = time.Now()
	tw.idleTimeout = idleTimeout
	return tw
}

type transWrapper struct {
	*transport
	lastActive  time.Time
	idleTimeout time.Duration
}

func (t *transWrapper) release() {
	t.transport = nil
	transWrapperPool.Put(t)
}

func (t *transWrapper) IsInvalid() bool {
	return time.Now().Sub(t.lastActive) > t.idleTimeout
}
