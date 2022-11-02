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

package mocks

import (
	"context"
	"net"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
)

type mockSvrTransHandlerFactory struct {
	hdlr *MockSvrTransHandler
}

// NewMockSvrTransHandlerFactory .
func NewMockSvrTransHandlerFactory(hdrl *MockSvrTransHandler) remote.ServerTransHandlerFactory {
	return &mockSvrTransHandlerFactory{hdrl}
}

func (f *mockSvrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	f.hdlr.Opt = opt
	return f.hdlr, nil
}

// MockSvrTransHandler .
type MockSvrTransHandler struct {
	Opt       *remote.ServerOption
	transPipe *remote.TransPipeline

	OnReadFunc func(ctx context.Context, conn net.Conn) error

	WriteFunc func(ctx context.Context, conn net.Conn, send remote.Message) (nctx context.Context, err error)

	ReadFunc func(ctx context.Context, conn net.Conn, msg remote.Message) (nctx context.Context, err error)
}

// OnRead implements the remote.TransHandler interface.
func (t *MockSvrTransHandler) OnRead(ctx context.Context, conn net.Conn) (err error) {
	if t.OnReadFunc != nil {
		return t.OnReadFunc(ctx, conn)
	}
	return
}

// Write implements the remote.TransHandler interface.
func (t *MockSvrTransHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) (nctx context.Context, err error) {
	if t.WriteFunc != nil {
		return t.WriteFunc(ctx, conn, send)
	}
	return
}

// Read implements the remote.TransHandler interface.
func (t *MockSvrTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) (nctx context.Context, err error) {
	if t.ReadFunc != nil {
		return t.ReadFunc(ctx, conn, msg)
	}
	return
}

// OnMessage implements the remote.TransHandler interface.
func (t *MockSvrTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	// do nothing
	return ctx, nil
}

// OnActive implements the remote.TransHandler interface.
func (t *MockSvrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	// ineffective now and do nothing
	return ctx, nil
}

// OnInactive implements the remote.TransHandler interface.
func (t *MockSvrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// ineffective now and do nothing
}

// OnError implements the remote.TransHandler interface.
func (t *MockSvrTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	if pe, ok := err.(*kerrors.DetailedError); ok {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, error=%s\nstack=%s", conn.RemoteAddr(), err.Error(), pe.Stack())
	} else {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, error=%s", conn.RemoteAddr(), err.Error())
	}
}

// SetPipeline implements the remote.TransHandler interface.
func (t *MockSvrTransHandler) SetPipeline(p *remote.TransPipeline) {
	t.transPipe = p
}

// MockTransServerFactory .
type MockTransServerFactory struct {
	transSvr *MockTransServer
}

// NewMockTransServerFactory .
func NewMockTransServerFactory(transSvr *MockTransServer) remote.TransServerFactory {
	return &MockTransServerFactory{transSvr}
}

// NewTransServer .
func (t *MockTransServerFactory) NewTransServer(opt *remote.ServerOption, transHdlr remote.ServerTransHandler) remote.TransServer {
	t.transSvr.opt = opt
	t.transSvr.transHdlr = transHdlr
	return t.transSvr
}

// MockTransServer .
type MockTransServer struct {
	opt       *remote.ServerOption
	transHdlr remote.ServerTransHandler

	CreateListenerFunc  func(net.Addr) (net.Listener, error)
	BootstrapServerFunc func(net.Listener) (err error)
	ShutdownFunc        func() (err error)
	ConnCountFunc       func() utils.AtomicInt
}

// CreateListener .
func (t *MockTransServer) CreateListener(addr net.Addr) (ln net.Listener, err error) {
	if t.CreateListenerFunc != nil {
		return t.CreateListenerFunc(addr)
	}
	return
}

// BootstrapServer .
func (t *MockTransServer) BootstrapServer(ln net.Listener) (err error) {
	if t.BootstrapServerFunc != nil {
		return t.BootstrapServerFunc(ln)
	}
	return
}

// Shutdown .
func (t *MockTransServer) Shutdown() (err error) {
	if t.ShutdownFunc != nil {
		return t.ShutdownFunc()
	}
	return
}

// ConnCount .
func (t *MockTransServer) ConnCount() (r utils.AtomicInt) {
	if t.ConnCountFunc != nil {
		return t.ConnCountFunc()
	}
	return
}
