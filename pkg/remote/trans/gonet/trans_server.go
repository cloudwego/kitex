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
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/remote/trans"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

const defaultShutdownTicker = 100 * time.Millisecond

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
	shutdown  atomic.Bool
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
func (ts *transServer) BootstrapServer(ln net.Listener) error {
	if ln == nil {
		return errors.New("listener is nil in gonet transport server")
	}
	ts.Lock()
	ts.ln = ln
	ts.Unlock()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ts.shutdown.Load() {
				// shutdown
				return nil
			}
			klog.Errorf("KITEX: BootstrapServer accept failed, err=%s", err.Error())
			return err
		}
		go ts.serveConn(context.Background(), conn)
	}
}

func (ts *transServer) serveConn(ctx context.Context, conn net.Conn) (err error) {
	defer transRecover(ctx, conn, "serveConn")

	ts.connCount.Inc()
	bc := newSvrConn(conn)
	defer func() {
		if err != nil {
			ts.onError(ctx, err, bc)
		}
		ts.connCount.Dec()
		bc.Close()
	}()

	ctx, err = ts.transHdlr.OnActive(ctx, bc)
	if err != nil {
		klog.CtxErrorf(ctx, "KITEX: OnActive error=%s", err)
		return err
	}
	for {
		// block to wait for next request
		bc.SetReadDeadline(time.Time{})
		_, err = bc.r.Peek(1)
		if err != nil {
			return err
		}
		ts.refreshDeadline(rpcinfo.GetRPCInfo(ctx), bc)
		err = ts.transHdlr.OnRead(ctx, bc)
		if err != nil {
			return err
		}
		if bc.Done() {
			// if Done returns true, return here.
			// for now, only http2 serverHandler execute `Do`.
			break
		}
	}
	return nil
}

// Shutdown implements the remote.TransServer interface.
func (ts *transServer) Shutdown() (err error) {
	ts.shutdown.Store(true)

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

	shutdownTicker := time.NewTicker(defaultShutdownTicker)
	defer shutdownTicker.Stop()
	for {
		select {
		case <-shutdownTicker.C:
			// check active conn count
			if ts.connCount.Value() == 0 {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// ConnCount implements the remote.TransServer interface.
func (ts *transServer) ConnCount() utils.AtomicInt {
	return utils.AtomicInt(ts.connCount.Value())
}

func (ts *transServer) onError(ctx context.Context, err error, conn net.Conn) {
	ts.transHdlr.OnError(ctx, err, conn)
}

func (ts *transServer) refreshDeadline(ri rpcinfo.RPCInfo, conn net.Conn) {
	readTimeout := ri.Config().ReadWriteTimeout()
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
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
