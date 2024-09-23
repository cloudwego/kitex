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
	tp.scavenger = newScavenger(config.MaxIdleTimeout)
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

func (c *longConnTransPool) Get(sinfo *serviceinfo.ServiceInfo, network string, addr string) (trans *transport, err error) {
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
	trans = cstack.Pop()
	if trans != nil {
		return trans, nil
	}
	conn, err := netpoll.DialConnection(network, addr, time.Second)
	if err != nil {
		return nil, err
	}
	trans = newTransport(clientTransport, sinfo, conn)
	_ = conn.AddCloseCallback(func(connection netpoll.Connection) error {
		_ = trans.Close()
		return nil
	})
	c.scavenger.Add(trans)
	return trans, nil
}

func (c *longConnTransPool) Put(trans *transport) {
	var cstack *transStack
	val, ok := c.pool.Load(trans.conn.RemoteAddr().String())
	if !ok {
		return
	}
	cstack = val.(*transStack)
	if cstack.Size() >= c.config.MaxIdlePerAddress {
		return
	}
	cstack.Push(trans)
}

func newTransStack() *transStack {
	return &transStack{stack: container.NewStack[*transport]()}
}

// FILO
type transStack struct {
	stack *container.Stack[*transport]
}

func (s *transStack) Size() int {
	return s.stack.Size()
}

func (s *transStack) Pop() *transport {
	trans, _ := s.stack.Pop()
	return trans
}

func (s *transStack) PopBottom() *transport {
	trans, _ := s.stack.PopBottom()
	return trans
}

func (s *transStack) Push(trans *transport) {
	s.stack.Push(trans)
}
