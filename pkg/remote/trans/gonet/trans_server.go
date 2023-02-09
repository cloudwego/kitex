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
	"bufio"
	"context"
	"errors"
	"io"
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
			bc := NewBufioConn(conn)
			ctx, err = ts.transHdlr.OnActive(ctx, bc)
			if err != nil {
				//return err
				klog.CtxErrorf(ctx, "KITEX: OnActive error=%s", err)
			}
			for {
				ts.refreshDeadline(rpcinfo.GetRPCInfo(ctx), bc)
				err := ts.transHdlr.OnRead(ctx, bc)
				if err != nil {
					klog.CtxErrorf(ctx, "KITEX: OnRead Error: %s\n", err.Error())
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

// BufioConn implements the net.Conn interface.
type BufioConn struct {
	conn net.Conn
	r    *bufioReader
}

func NewBufioConn(c net.Conn) *BufioConn {
	return &BufioConn{
		conn: c,
		//rw:   &bufioReader{rw: bufio.NewReadWriter(r, w)},
		r: NewBufioReader(c),
	}
}

func (bc *BufioConn) RawConn() net.Conn {
	return bc.conn
}

func (bc *BufioConn) Read(b []byte) (int, error) {
	//return bc.rw.rw.Read(b)
	return bc.r.r.Read(b)
}

func (bc *BufioConn) Write(b []byte) (int, error) {
	return bc.conn.Write(b)
}

func (bc *BufioConn) Close() error {
	bc.r.Release()
	return bc.conn.Close()
}

func (bc *BufioConn) LocalAddr() net.Addr {
	return bc.conn.LocalAddr()
}

func (bc *BufioConn) RemoteAddr() net.Addr {
	return bc.conn.RemoteAddr()
}

func (bc *BufioConn) SetDeadline(t time.Time) error {
	return bc.conn.SetDeadline(t)
}

func (bc *BufioConn) SetReadDeadline(t time.Time) error {
	return bc.conn.SetReadDeadline(t)
}

func (bc *BufioConn) SetWriteDeadline(t time.Time) error {
	return bc.conn.SetWriteDeadline(t)
}

func (bc *BufioConn) Reader() netpoll.Reader {
	return bc.r
}

var _ netpoll.Reader = &bufioReader{}

func NewBufioReader(c net.Conn) *bufioReader {
	return &bufioReader{r: bufio.NewReader(c)}
}

// bufioReader implements the netpoll.Reader interface.
type bufioReader struct {
	r *bufio.Reader
}

func (br bufioReader) Next(n int) (p []byte, err error) {
	p = make([]byte, n)
	if _, err = io.ReadFull(br.r, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (br *bufioReader) Peek(n int) (buf []byte, err error) {
	return br.r.Peek(n)
}

func (br *bufioReader) Skip(n int) (err error) {
	_, err = br.r.Discard(n)
	return
}

func (br *bufioReader) Until(delim byte) (line []byte, err error) {
	panic("unimplemented")
}

func (br *bufioReader) ReadString(n int) (s string, err error) {
	return br.r.ReadString(byte(n))
}

func (br *bufioReader) ReadBinary(n int) (p []byte, err error) {
	return br.r.ReadBytes(byte(n))
}

func (br *bufioReader) ReadByte() (b byte, err error) {
	return br.r.ReadByte()
}

func (br *bufioReader) Slice(n int) (r netpoll.Reader, err error) {
	panic("implement me")
}

func (br *bufioReader) Release() (err error) {
	br.r.Buffered()
	return nil
}

func (br *bufioReader) Len() (length int) {
	return br.r.Buffered()
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
