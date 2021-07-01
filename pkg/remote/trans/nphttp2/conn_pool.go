/*
 * Copyright 2021 CloudWeGo Authors
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

package nphttp2

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"

	"golang.org/x/sync/singleflight"
)

// NewConnPool ...
func NewConnPool() remote.LongConnPool {
	return &connPool{}
}

// MuxPool manages a pool of long connections.
type connPool struct {
	sfg   singleflight.Group
	conns sync.Map // key address, value *clientConn
}

var _ remote.LongConnPool = (*connPool)(nil)

// Get pick or generate a net.Conn and return
func (p *connPool) Get(ctx context.Context, network, address string, opt *remote.ConnOption) (net.Conn, error) {
	v, ok := p.conns.Load(address)
	if ok {
		tr := v.(grpc.ClientTransport)
		conn, err := newClientConn(ctx, tr, address)
		if err != nil {
			p.conns.Delete(address)
			return nil, err
		}
		return conn, nil
	}
	tr, err, _ := p.sfg.Do(address, func() (i interface{}, e error) {
		conn, err := opt.Dialer.DialTimeout(network, address, opt.ConnectTimeout)
		if err != nil {
			return nil, err
		}
		tr, err := grpc.NewClientTransport(context.Background(), conn.(netpoll.Connection), func(r grpc.GoAwayReason) {
			p.conns.Delete(address)
		}, func() {
			// delete from conn pool
			p.conns.Delete(address)
		})
		if err != nil {
			return nil, err
		}
		p.conns.Store(address, tr)
		return tr, nil
	})
	if err != nil {
		return nil, err
	}
	conn, err := newClientConn(ctx, tr.(grpc.ClientTransport), address)
	if err != nil {
		p.conns.Delete(address)
		return nil, err
	}
	return conn, nil
}

// Put implements the ConnPool interface.
func (p *connPool) Put(conn net.Conn) error {
	return nil
}

// Discard implements the ConnPool interface.
func (p *connPool) Discard(conn net.Conn) error {
	return nil
}

// Clean implements the LongConnPool interface.
func (p *connPool) Clean(network, address string) {
	if v, ok := p.conns.Load(address); ok {
		p.conns.Delete(address)
		v.(io.Closer).Close()
	}
}

// Close is to release resource of ConnPool, it is executed when client is closed.
func (p *connPool) Close() error {
	p.conns.Range(func(addr, clientConn interface{}) bool {
		p.conns.Delete(addr)
		clientConn.(io.Closer).Close()
		return true
	})
	return nil
}
