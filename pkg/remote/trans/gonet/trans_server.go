/*
 * Copyright 2022 CloudWeGo Authors
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

// Package gonet contains server and client implementation for go net.
package gonet

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/remote/trans"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

// NewTransServerFactory creates a default go net transport server factory.
func NewTransServerFactory() remote.TransServerFactory {
	return &gonetTransServerFactory{}
}

type gonetTransServerFactory struct{}

// NewTransServer implements the remote.TransServerFactory interface.
func (f *gonetTransServerFactory) NewTransServer(opt *remote.ServerOption, transHdlr remote.ServerTransHandler) remote.TransServer {
	return &transServer{
		opt:       opt,
		transHdlr: transHdlr,
		lncfg:     trans.NewListenConfig(opt),
	}
}

type transServer struct {
	opt       *remote.ServerOption
	transHdlr remote.ServerTransHandler
	ln        net.Listener
	lncfg     net.ListenConfig
	connCount utils.AtomicInt
	sync.Mutex
}

var _ remote.TransServer = &transServer{}

// CreateListener implements the remote.TransServer interface.
// The network must be "tcp", "tcp4", "tcp6" or "unix".
func (ts *transServer) CreateListener(addr net.Addr) (ln net.Listener, err error) {
	ln, err = ts.lncfg.Listen(context.Background(), addr.Network(), addr.String())
	return ln, err
}

// BootstrapServer implements the remote.TransServer interface.
func (ts *transServer) BootstrapServer(ln net.Listener) (err error) {
	if ln == nil {
		return errors.New("listener is nil in gonet transport server")
	}
	ts.ln = ln
	for {
		conn, err := ts.ln.Accept()
		if err != nil {
			klog.Errorf("KITEX: BootstrapServer accept failed, err=%s", err.Error())
			os.Exit(1)
		}
		go func() {
			var (
				ctx = context.Background()
				err error
			)
			defer func() {
				transRecover(ctx, conn, "OnRead")
			}()
			bc := newBufioConn(conn)
			ctx, err = ts.transHdlr.OnActive(ctx, bc)
			if err != nil {
				klog.CtxErrorf(ctx, "KITEX: OnActive error=%s", err)
				return
			}
			for {
				ts.refreshDeadline(rpcinfo.GetRPCInfo(ctx), bc)
				err := ts.transHdlr.OnRead(ctx, bc)
				if err != nil {
					ts.onError(ctx, err, bc)
					_ = bc.Close()
					return
				}
			}
		}()
	}
}

// Shutdown implements the remote.TransServer interface.
func (ts *transServer) Shutdown() (err error) {
	ts.Lock()
	defer ts.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), ts.opt.ExitWaitTime)
	defer cancel()
	if g, ok := ts.transHdlr.(remote.GracefulShutdown); ok {
		if ts.ln != nil {
			// 1. stop listener
			_ = ts.ln.Close()

			// 2. signal all active connections to close gracefully
			_ = g.GracefulShutdown(ctx)
		}
	}
	return
}

// ConnCount implements the remote.TransServer interface.
func (ts *transServer) ConnCount() utils.AtomicInt {
	return ts.connCount
}

func (ts *transServer) onError(ctx context.Context, err error, conn net.Conn) {
	ts.transHdlr.OnError(ctx, err, conn)
}

func (ts *transServer) refreshDeadline(ri rpcinfo.RPCInfo, conn net.Conn) {
	readTimeout := ri.Config().ReadWriteTimeout()
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
}

// bufioConn implements the net.Conn interface.
type bufioConn struct {
	conn net.Conn
	r    netpoll.Reader
}

func newBufioConn(c net.Conn) *bufioConn {
	return &bufioConn{
		conn: c,
		r:    netpoll.NewReader(c),
	}
}

func (bc *bufioConn) RawConn() net.Conn {
	return bc.conn
}

func (bc *bufioConn) Read(b []byte) (int, error) {
	buf, err := bc.r.Next(len(b))
	if err != nil {
		return 0, err
	}
	copy(b, buf)
	return len(b), nil
}

func (bc *bufioConn) Write(b []byte) (int, error) {
	return bc.conn.Write(b)
}

func (bc *bufioConn) Close() error {
	bc.r.Release()
	return bc.conn.Close()
}

func (bc *bufioConn) LocalAddr() net.Addr {
	return bc.conn.LocalAddr()
}

func (bc *bufioConn) RemoteAddr() net.Addr {
	return bc.conn.RemoteAddr()
}

func (bc *bufioConn) SetDeadline(t time.Time) error {
	return bc.conn.SetDeadline(t)
}

func (bc *bufioConn) SetReadDeadline(t time.Time) error {
	return bc.conn.SetReadDeadline(t)
}

func (bc *bufioConn) SetWriteDeadline(t time.Time) error {
	return bc.conn.SetWriteDeadline(t)
}

func (bc *bufioConn) Reader() netpoll.Reader {
	return bc.r
}

func transRecover(ctx context.Context, conn net.Conn, funcName string) {
	panicErr := recover()
	if panicErr != nil {
		if conn != nil {
			klog.CtxErrorf(ctx, "KITEX: panic happened in %s, remoteAddress=%s, error=%v\nstack=%s", funcName, conn.RemoteAddr(), panicErr, string(debug.Stack()))
		} else {
			klog.CtxErrorf(ctx, "KITEX: panic happened in %s, error=%v\nstack=%s", funcName, panicErr, string(debug.Stack()))
		}
	}
}
