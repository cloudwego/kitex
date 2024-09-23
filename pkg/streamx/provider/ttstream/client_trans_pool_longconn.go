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
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/container"
	"github.com/cloudwego/netpoll"
	"golang.org/x/sync/singleflight"
)

var DefaultLongConnConfig = LongConnConfig{
	MaxIdlePerAddress: 100,
	MaxIdleTimeout:    time.Minute,
}

type LongConnConfig struct {
	MaxIdlePerAddress int
	MaxIdleTimeout    time.Duration
}

func newLongConnTransPool(config LongConnConfig) transPool {
	tp := new(longConnTransPool)
	if config.MaxIdlePerAddress == 0 {
		config.MaxIdlePerAddress = DefaultLongConnConfig.MaxIdlePerAddress
	}
	if config.MaxIdleTimeout == 0 {
		config.MaxIdleTimeout = DefaultLongConnConfig.MaxIdleTimeout
	}
	tp.scavenger = newScavenger()
	tp.config = config
	return tp
}

type longConnTransPool struct {
	pool      sync.Map // {"addr":*transStack}
	scavenger *scavenger
	sg        singleflight.Group
	config    LongConnConfig
}

const localhost = "localhost"

func (c *longConnTransPool) Get(sinfo *serviceinfo.ServiceInfo, network string, addr string) (*transport, error) {
	if strings.HasPrefix(addr, localhost) {
		addr = "127.0.0.1" + addr[len(localhost):]
	}
	var cstack *transStack
	val, ok := c.pool.Load(addr)
	if ok {
		cstack = val.(*transStack)
	} else {
		v, err, _ := c.sg.Do(addr, func() (interface{}, error) {
			cstack = newTransStack()
			c.pool.Store(addr, cstack)
			return cstack, nil
		})
		if err != nil {
			return nil, err
		}
		cstack = v.(*transStack)
	}
	for {
		tw, _ := cstack.Pop()
		if tw == nil {
			break
		}
		trans := tw.transport
		tw.release()
		if !tw.IsInvalid() {
			return trans, nil
		}
		// continue to find a valid transport
	}

	// create new connection
	conn, err := netpoll.DialConnection(network, addr, time.Second)
	if err != nil {
		return nil, err
	}
	// create new transport
	trans := newTransport(clientTransport, sinfo, conn)
	tw := newTransWrapper(trans, c.config.MaxIdleTimeout)
	// register into scavenger
	c.scavenger.Add(tw)
	return trans, nil
}

func (c *longConnTransPool) Put(trans *transport) {
	var cstack *transStack
	val, ok := c.pool.Load(trans.conn.RemoteAddr().String())
	if !ok {
		return
	}
	cstack = val.(*transStack)
	//if cstack.Size() >= c.config.MaxIdlePerAddress {
	//	// discard transport
	//	_ = trans.Close()
	//	return
	//}
	tw := newTransWrapper(trans, c.config.MaxIdleTimeout)
	cstack.Push(tw)
}

func newTransStack() *transStack {
	return container.NewStack[*transWrapper]()
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
	return !(t.transport.IsActive() && time.Now().Sub(t.lastActive) < t.idleTimeout)
}

type transStack = container.Stack[*transWrapper]
