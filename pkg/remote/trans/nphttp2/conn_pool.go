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
	"crypto/tls"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
)

var _ remote.LongConnPool = &connPool{}

func poolSize() uint32 {
	// One connection per processor, and need redundancy。
	// Benchmark indicates this setting is the best performance.
	numP := runtime.GOMAXPROCS(0)
	return uint32(numP * 3 / 2)
}

// NewConnPool ...
func NewConnPool(remoteService string, size uint32, connOpts grpc.ConnectOptions) *connPool {
	if size == 0 {
		size = poolSize()
	}
	return &connPool{
		remoteService: remoteService,
		size:          size,
		connOpts:      connOpts,
	}
}

// MuxPool manages a pool of long connections.
type connPool struct {
	size          uint32
	sfg           singleflight.Group
	conns         sync.Map // key: address, value: *transports
	remoteService string   // remote service name
	connOpts      grpc.ConnectOptions
}

type transports struct {
	index         uint32
	size          uint32
	cliTransports []grpc.ClientTransport
}

// get connection from the pool, load balance with round-robin.
func (t *transports) get() grpc.ClientTransport {
	idx := atomic.AddUint32(&t.index, 1)
	return t.cliTransports[idx%t.size]
}

// put find the first empty position to put the connection to the pool.
func (t *transports) put(trans grpc.ClientTransport) {
	for i := 0; i < int(t.size); i++ {
		cliTransport := t.cliTransports[i]
		if cliTransport == nil {
			t.cliTransports[i] = trans
			return
		}
		if !cliTransport.(grpc.IsActive).IsActive() {
			t.cliTransports[i].GracefulClose()
			t.cliTransports[i] = trans
			return
		}
	}
}

// close all connections of the pool.
func (t *transports) close() {
	for i := range t.cliTransports {
		if c := t.cliTransports[i]; c != nil {
			c.GracefulClose()
		}
	}
}

var _ remote.LongConnPool = (*connPool)(nil)

func (p *connPool) newTransport(ctx context.Context, dialer remote.Dialer, network, address string,
	connectTimeout time.Duration, opts grpc.ConnectOptions,
) (grpc.ClientTransport, error) {
	conn, err := dialer.DialTimeout(network, address, connectTimeout)
	if err != nil {
		return nil, err
	}
	if opts.TLSConfig != nil {
		conn = tls.Client(conn, opts.TLSConfig)
	}
	return grpc.NewClientTransport(
		ctx,
		conn,
		opts,
		p.remoteService,
		func(grpc.GoAwayReason) {
			// do nothing
		},
		func() {
			// do nothing
		},
	)
}

// Get pick or generate a net.Conn and return
func (p *connPool) Get(ctx context.Context, network, address string, opt remote.ConnOption) (net.Conn, error) {
	if p.connOpts.ShortConn {
		return p.createShortConn(ctx, network, address, opt)
	}

	var (
		trans *transports
		conn  *clientConn
		err   error
	)

	v, ok := p.conns.Load(address)
	if ok {
		trans = v.(*transports)
		if tr := trans.get(); tr != nil {
			if tr.(grpc.IsActive).IsActive() {
				// Actually new a stream, reuse the connection (grpc.ClientTransport)
				conn, err = newClientConn(ctx, tr, address)
				if err == nil {
					return conn, nil
				}
				klog.CtxDebugf(ctx, "KITEX: New grpc stream failed, network=%s, address=%s, error=%s", network, address, err.Error())
			}
		}
	}
	tr, err, _ := p.sfg.Do(address, func() (i interface{}, e error) {
		// Notice: newTransport means new a connection, the timeout of connection cannot be set,
		// so using context.Background() but not the ctx passed in as the parameter.
		tr, err := p.newTransport(context.Background(), opt.Dialer, network, address, opt.ConnectTimeout, p.connOpts)
		if err != nil {
			return nil, err
		}
		if trans == nil {
			trans = &transports{
				size:          p.size,
				cliTransports: make([]grpc.ClientTransport, p.size),
			}
		}
		trans.put(tr) // the tr (connection) maybe not in the pool, but can be recycled by keepalive.
		p.conns.Store(address, trans)
		return tr, nil
	})
	if err != nil {
		klog.CtxErrorf(ctx, "KITEX: New grpc client connection failed, network=%s, address=%s, error=%s", network, address, err.Error())
		return nil, err
	}
	klog.CtxDebugf(ctx, "KITEX: New grpc client connection succeed, network=%s, address=%s", network, address)
	return newClientConn(ctx, tr.(grpc.ClientTransport), address)
}

// Put implements the ConnPool interface.
func (p *connPool) Put(conn net.Conn) error {
	if p.connOpts.ShortConn {
		return p.release(conn)
	}
	return nil
}

func (p *connPool) release(conn net.Conn) error {
	clientConn := conn.(*clientConn)
	clientConn.tr.GracefulClose()
	return nil
}

func (p *connPool) createShortConn(ctx context.Context, network, address string, opt remote.ConnOption) (net.Conn, error) {
	// Notice: newTransport means new a connection, the timeout of connection cannot be set,
	// so using context.Background() but not the ctx passed in as the parameter.
	tr, err := p.newTransport(context.Background(), opt.Dialer, network, address, opt.ConnectTimeout, p.connOpts)
	if err != nil {
		return nil, err
	}
	return newClientConn(ctx, tr, address)
}

// Discard implements the ConnPool interface.
func (p *connPool) Discard(conn net.Conn) error {
	return nil
}

// Clean implements the LongConnPool interface.
func (p *connPool) Clean(network, address string) {
	if v, ok := p.conns.Load(address); ok {
		p.conns.Delete(address)
		v.(*transports).close()
	}
}

// Close is to release resource of ConnPool, it is executed when client is closed.
func (p *connPool) Close() error {
	p.conns.Range(func(addr, trans interface{}) bool {
		p.conns.Delete(addr)
		trans.(*transports).close()
		return true
	})
	return nil
}
