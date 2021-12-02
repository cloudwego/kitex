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
)

type mockCliTransHandlerFactory struct {
	hdlr *MockCliTransHandler
}

// NewMockCliTransHandlerFactory .
func NewMockCliTransHandlerFactory(hdrl *MockCliTransHandler) remote.ClientTransHandlerFactory {
	return &mockCliTransHandlerFactory{hdrl}
}

func (f *mockCliTransHandlerFactory) NewTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	f.hdlr.opt = opt
	return f.hdlr, nil
}

// MockCliTransHandler .
type MockCliTransHandler struct {
	opt       *remote.ClientOption
	transPipe *remote.TransPipeline

	WriteFunc func(ctx context.Context, conn net.Conn, send remote.Message) error

	// 调用decode
	ReadFunc func(ctx context.Context, conn net.Conn, msg remote.Message) error
}

func (t *MockCliTransHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) (err error) {
	if t.WriteFunc != nil {
		return t.WriteFunc(ctx, conn, send)
	}
	return
}

// Read 阻塞等待
func (t *MockCliTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) (err error) {
	if t.ReadFunc != nil {
		return t.ReadFunc(ctx, conn, msg)
	}
	return
}

// OnMessage .
func (t *MockCliTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	// do nothing
	return ctx, nil
}

// OnActive 新连接建立时触发，主要用于服务端，对用netpoll onPrepare
func (t *MockCliTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	// ineffective now and do nothing
	return ctx, nil
}

// OnInactive 连接关闭时回调
func (t *MockCliTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// ineffective now and do nothing
}

// OnError 传输层扩展中panic 回调
func (t *MockCliTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	if pe, ok := err.(*kerrors.DetailedError); ok {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, err=%s\n%s", conn.RemoteAddr(), err.Error(), pe.Stack())
	} else {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, err=%s", conn.RemoteAddr(), err.Error())
	}
}

// SetPipeline .
func (t *MockCliTransHandler) SetPipeline(p *remote.TransPipeline) {
	t.transPipe = p
}
