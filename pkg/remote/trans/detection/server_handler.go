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

// Package detection protocol detection
package detection

import (
	"bytes"
	"context"
	"net"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	transNetpoll "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
)

type detectionKey struct{}

type detectionHandler struct {
	handler remote.ServerTransHandler
}

// NewSvrTransHandlerFactory detection factory construction
func NewSvrTransHandlerFactory() remote.ServerTransHandlerFactory {
	return &svrTransHandlerFactory{
		http2:   nphttp2.NewSvrTransHandlerFactory(),
		netpoll: transNetpoll.NewSvrTransHandlerFactory(),
	}
}

type svrTransHandlerFactory struct {
	http2   remote.ServerTransHandlerFactory
	netpoll remote.ServerTransHandlerFactory
}

func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	t := &svrTransHandler{}
	var err error
	if t.http2, err = f.http2.NewTransHandler(opt); err != nil {
		return nil, err
	}
	if t.netpoll, err = f.netpoll.NewTransHandler(opt); err != nil {
		return nil, err
	}
	return t, nil
}

type svrTransHandler struct {
	http2   remote.ServerTransHandler
	netpoll remote.ServerTransHandler
}

func (t *svrTransHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) error {
	return t.which(ctx).Write(ctx, conn, send)
}

func (t *svrTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) error {
	return t.which(ctx).Read(ctx, conn, msg)
}

var prefaceReadAtMost = func() int {
	// min(len(ClientPreface), len(flagBuf))
	// len(flagBuf) = 2 * codec.Size32
	if 2*codec.Size32 < grpc.ClientPrefaceLen {
		return 2 * codec.Size32
	}
	return grpc.ClientPrefaceLen
}()

func (t *svrTransHandler) OnRead(ctx context.Context, conn net.Conn) error {
	// only need detect once when connection is reused
	r, ok := ctx.Value(detectionKey{}).(*detectionHandler)
	if ok && r != nil && r.handler != nil {
		return r.handler.OnRead(ctx, conn)
	}
	// Check the validity of client preface.
	zr := conn.(netpoll.Connection).Reader()
	// read at most avoid block
	preface, err := zr.Peek(prefaceReadAtMost)
	if err != nil {
		return err
	}
	// compare preface one by one
	which := t.netpoll
	if bytes.Equal(preface[:prefaceReadAtMost], grpc.ClientPreface[:prefaceReadAtMost]) {
		which = t.http2
		ctx, err = which.OnActive(ctx, conn)
		if err != nil {
			return err
		}
	}
	r.handler = which
	return which.OnRead(ctx, conn)
}

func (t *svrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	t.which(ctx).OnInactive(ctx, conn)
}

func (t *svrTransHandler) which(ctx context.Context) remote.ServerTransHandler {
	if r, ok := ctx.Value(detectionKey{}).(*detectionHandler); ok && r != nil && r.handler != nil {
		return r.handler
	}
	// use noop transHandler
	return noopSvrTransHandler{}
}

func (t *svrTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	t.which(ctx).OnError(ctx, err, conn)
}

func (t *svrTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	return t.which(ctx).OnMessage(ctx, args, result)
}

//
func (t *svrTransHandler) SetPipeline(pipeline *remote.TransPipeline) {
	t.http2.SetPipeline(pipeline)
	t.netpoll.SetPipeline(pipeline)
}

func (t *svrTransHandler) SetInvokeHandleFunc(inkHdlFunc endpoint.Endpoint) {
	if t, ok := t.http2.(remote.InvokeHandleFuncSetter); ok {
		t.SetInvokeHandleFunc(inkHdlFunc)
	}
	if t, ok := t.netpoll.(remote.InvokeHandleFuncSetter); ok {
		t.SetInvokeHandleFunc(inkHdlFunc)
	}
}

func (t *svrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	ctx, err := t.netpoll.OnActive(ctx, conn)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, detectionKey{}, &detectionHandler{}), nil
}
