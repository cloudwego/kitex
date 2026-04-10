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
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

const (
	poolOpen   int32 = 0
	poolClosed int32 = 1
)

func poolSize() uint32 {
	// One connection per processor, and need redundancy。
	// Benchmark indicates this setting is the best performance.
	numP := runtime.GOMAXPROCS(0)
	return uint32(numP * 3 / 2)
}

// NewConnPool ...
func NewConnPool(remoteService string, size uint32, connOpts grpc.ConnectOptions) *connPool {
	if connOpts.TraceController == nil {
		connOpts.TraceController = &rpcinfo.TraceController{}
	}
	// process Header and Trailer retrieving for unary
	connOpts.TraceController.AppendClientStreamEventHandler(unaryMetaEventHandler)
	if size == 0 {
		size = poolSize()
	}
	return &connPool{
		remoteService: remoteService,
		size:          size,
		connOpts:      connOpts,
	}
}

// connPool manages a pool of gRPC long connections.
type connPool struct {
	size          uint32
	sfg           singleflight.Group
	conns         sync.Map // key: address, value: *transports
	remoteService string   // remote service name
	connOpts      grpc.ConnectOptions
	closed        int32 // 1 means connPool has been closed
}

var (
	_                 remote.LongConnPool = (*connPool)(nil)
	errConnPoolClosed                     = status.Err(codes.Aborted, "connection pool has been closed")
)

// Get pick or generate a net.Conn and return
func (p *connPool) Get(ctx context.Context, network, address string, opt remote.ConnOption) (net.Conn, error) {
	if p.connOpts.ShortConn {
		return p.createShortConn(ctx, network, address, opt)
	}

	var (
		tr   grpc.ClientTransport
		idx  uint32
		conn *clientConn
		err  error
	)

	// there is no need to check whether connPool has been closed
	// because connPool would only be closed when Kitex Client is GCed
	v, ok := p.conns.Load(address)
	if ok {
		trans := v.(*transports)
		tr, idx = trans.getActiveTransport()
		if tr != nil {
			// Actually new a stream, reuse the connection (grpc.ClientTransport)
			conn, err = newClientConn(ctx, tr, address)
			if err == nil {
				return conn, nil
			}

			// when stream creations failed:
			// - gRPC Connection closed or draining: create a new connection
			// - ctx canceled: the request lifecycle has ended, exit immediately
			select {
			// ctx provided by users is canceled, we should not try to create a new gRPC connection
			case <-ctx.Done():
				return nil, err
			default:
			}
			klog.CtxDebugf(ctx, "KITEX: New grpc stream failed, network=%s, address=%s, error=%s", network, address, err.Error())
		}
	}
	rawTr, dErr, _ := p.sfg.Do(address, func() (i interface{}, e error) {
		var trans *transports
		var isNew bool
		// avoid creating duplicate transports
		if existTrans, ok := p.conns.Load(address); ok {
			trans = existTrans.(*transports)
		} else {
			trans = newTransports(p.size)
			isNew = true
		}

		res, cErr := trans.createTransport(idx, p.remoteService, opt.Dialer, network, address, opt.ConnectTimeout, p.connOpts)
		if cErr != nil {
			return nil, cErr
		}

		if isNew {
			// Store first, then recheck closed state to eliminate TOCTOU:
			// if Close() finished its Range between our earlier check and this store,
			// self-clean here to prevent orphaned transports.
			p.conns.LoadOrStore(address, trans)
			if p.isClosed() {
				if recheckV, recheckOK := p.conns.LoadAndDelete(address); recheckOK {
					recheckV.(*transports).close()
				}
				return nil, errConnPoolClosed
			}
		}

		return res, nil
	})
	if dErr != nil {
		klog.CtxErrorf(ctx, "KITEX: New grpc client connection failed, network=%s, address=%s, error=%s", network, address, dErr.Error())
		return nil, dErr
	}
	klog.CtxDebugf(ctx, "KITEX: New grpc client connection succeed, network=%s, address=%s", network, address)
	return newClientConn(ctx, rawTr.(grpc.ClientTransport), address)
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
	tr, err := newTransport(p.remoteService, opt.Dialer, network, address, opt.ConnectTimeout, p.connOpts, nil, nil)
	if err != nil {
		return nil, err
	}
	return newClientConn(ctx, tr, address)
}

// Discard implements the ConnPool interface.
func (p *connPool) Discard(conn net.Conn) error {
	if p.connOpts.ShortConn {
		return p.release(conn)
	}
	return nil
}

// Clean implements the LongConnPool interface.
func (p *connPool) Clean(network, address string) {
	if v, ok := p.conns.LoadAndDelete(address); ok {
		v.(*transports).close()
	}
}

// Close is to release resource of ConnPool, it is executed when client is closed.
func (p *connPool) Close() error {
	if !p.casClosed() {
		return nil
	}

	p.conns.Range(func(addr, trans interface{}) bool {
		p.conns.Delete(addr)
		trans.(*transports).close()
		return true
	})
	return nil
}

func (p *connPool) isClosed() bool {
	return atomic.LoadInt32(&p.closed) == poolClosed
}

func (p *connPool) casClosed() bool {
	return atomic.CompareAndSwapInt32(&p.closed, poolOpen, poolClosed)
}

type dumpEntry struct {
	addr string
	tr   grpc.ClientTransport
}

// Dump dumps the connection pool with the details of the underlying transport.
func (p *connPool) Dump() interface{} {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			klog.Errorf("KITEX: dump gRPC client connection pool panic, err=%v, stack=%s", panicErr, string(debug.Stack()))
		}
	}()

	// remoteAddress -> []clientTransport, where each clientTransport is a connection. Distinguish the connection via localAddress.
	// If mesh egress is not enabled, toAddr should be the address of the callee service.
	// Otherwise, toAddr will be the same, so you should check the remoteAddress in each stream, which is read from the header.

	// sync.Map does not expose its length directly.
	// Since dump is a cold-path operation, performance is not a major concern here
	poolDump := make(map[string]interface{}, p.size)
	var cliTransDumps []dumpEntry
	p.conns.Range(func(k, v interface{}) bool {
		addr := k.(string)
		for _, tr := range v.(*transports).loadAll() {
			cliTransDumps = append(cliTransDumps, dumpEntry{addr: addr, tr: tr})
		}
		return true
	})

	for _, cliTransDump := range cliTransDumps {
		dumper, ok := cliTransDump.tr.(interface{ Dump() interface{} })
		if !ok {
			continue
		}
		var curr []interface{}
		if poolDump[cliTransDump.addr] == nil {
			curr = make([]interface{}, 0)
		} else {
			curr = poolDump[cliTransDump.addr].([]interface{})
		}
		curr = append(curr, dumper.Dump())
		poolDump[cliTransDump.addr] = curr
	}
	return poolDump
}

// newTransport creates a gRPC connection
func newTransport(remoteService string,
	dialer remote.Dialer, network, address string, connectTimeout time.Duration, opts grpc.ConnectOptions,
	onGoAway func(context.Context, grpc.ClientTransport, grpc.GoAwayReason),
	onClose func(context.Context, grpc.ClientTransport, error),
) (grpc.ClientTransport, error) {
	conn, err := dialer.DialTimeout(network, address, connectTimeout)
	if err != nil {
		return nil, err
	}
	if opts.TLSConfig != nil {
		tlsConn, tErr := newTLSConn(conn, opts.TLSConfig)
		if tErr != nil {
			// release tls handshake failed connection
			cErr := conn.Close()
			if cErr != nil {
				klog.Warnf("KITEX: Close TLS handshake failed connection, err: %v", cErr)
			}
			return nil, tErr
		}
		conn = tlsConn
	}
	return grpc.NewClientTransportWithConfig(
		context.Background(), // gRPC connection does not need to be bound to a specific ctx
		conn,
		opts,
		grpc.ClientConfig{
			RemoteService: remoteService,
			OnGoAway:      onGoAway,
			OnClose:       onClose,
		},
	)
}

// newTLSConn constructs a client-side TLS connection and performs handshake.
func newTLSConn(conn net.Conn, tlsCfg *tls.Config) (net.Conn, error) {
	tlsConn := tls.Client(conn, tlsCfg)
	if err := tlsConn.Handshake(); err != nil {
		return nil, err
	}
	return tlsConn, nil
}

func checkActive(trans grpc.ClientTransport) bool {
	if trans == nil {
		return false
	}
	// grpc.ClientTransport is implemented by *http2Client in pkg/remote/trans/nphttp2/grpc
	// it implements grpc.IsActive
	return trans.(grpc.IsActive).IsActive()
}
