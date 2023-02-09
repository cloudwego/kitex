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

// Package detection protocol detection, it is used for a scenario that switching KitexProtobuf to gRPC.
// No matter KitexProtobuf or gRPC the server side can handle with this detection handler.
package detection

import (
	"bytes"
	"context"
	"net"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
)

// NewSvrTransHandlerFactory detection factory construction
func NewSvrTransHandlerFactory(nonHttp2, http2 remote.ServerTransHandlerFactory) remote.ServerTransHandlerFactory {
	return &svrTransHandlerFactory{nonHttp2, http2}
}

type svrTransHandlerFactory struct {
	nonHttp2 remote.ServerTransHandlerFactory
	http2    remote.ServerTransHandlerFactory
}

func (f *svrTransHandlerFactory) MuxEnabled() bool {
	return false
}

func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	t := &svrTransHandler{}
	var err error
	if t.http2, err = f.http2.NewTransHandler(opt); err != nil {
		return nil, err
	}
	if t.nonHttp2, err = f.nonHttp2.NewTransHandler(opt); err != nil {
		return nil, err
	}
	return t, nil
}

type svrTransHandler struct {
	http2    remote.ServerTransHandler
	nonHttp2 remote.ServerTransHandler
}

func (t *svrTransHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) (nctx context.Context, err error) {
	return t.which(ctx).Write(ctx, conn, send)
}

func (t *svrTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) (nctx context.Context, err error) {
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
	r := ctx.Value(handlerKey{}).(*handlerWrapper)
	if r.handler != nil {
		return r.handler.OnRead(r.ctx, conn)
	}
	// Check the validity of client preface.
	var (
		preface []byte
		err     error
	)
	npReader := conn.(interface{ Reader() netpoll.Reader }).Reader()
	if preface, err = npReader.Peek(prefaceReadAtMost); err != nil {
		return err
	}
	// compare preface one by one
	which := t.nonHttp2
	if bytes.Equal(preface[:prefaceReadAtMost], grpc.ClientPreface[:prefaceReadAtMost]) {
		which = t.http2
		ctx, err = which.OnActive(ctx, conn)
		if err != nil {
			return err
		}
	}
	r.ctx, r.handler = ctx, which
	return which.OnRead(ctx, conn)
}

func (t *svrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// t.http2 should use the ctx returned by OnActive in r.ctx
	if r, ok := ctx.Value(handlerKey{}).(*handlerWrapper); ok && r.ctx != nil {
		ctx = r.ctx
	}
	t.which(ctx).OnInactive(ctx, conn)
}

func (t *svrTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	t.which(ctx).OnError(ctx, err, conn)
}

func (t *svrTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	return t.which(ctx).OnMessage(ctx, args, result)
}

func (t *svrTransHandler) which(ctx context.Context) remote.ServerTransHandler {
	if r, ok := ctx.Value(handlerKey{}).(*handlerWrapper); ok && r.handler != nil {
		return r.handler
	}
	// use noop transHandler
	return noopHandler
}

func (t *svrTransHandler) SetPipeline(pipeline *remote.TransPipeline) {
	t.http2.SetPipeline(pipeline)
	t.nonHttp2.SetPipeline(pipeline)
}

func (t *svrTransHandler) SetInvokeHandleFunc(inkHdlFunc endpoint.Endpoint) {
	if t, ok := t.http2.(remote.InvokeHandleFuncSetter); ok {
		t.SetInvokeHandleFunc(inkHdlFunc)
	}
	if t, ok := t.nonHttp2.(remote.InvokeHandleFuncSetter); ok {
		t.SetInvokeHandleFunc(inkHdlFunc)
	}
}

func (t *svrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	ctx, err := t.nonHttp2.OnActive(ctx, conn)
	if err != nil {
		return nil, err
	}
	// svrTransHandler wraps two kinds of ServerTransHandler: http2, none-http2.
	// We think that one connection only use one type, it doesn't need to do protocol detection for every request.
	// And ctx is initialized with a new connection, so we put a handlerWrapper into ctx, which for recording
	// the actual handler, then the later request don't need to do http2 detection.
	return context.WithValue(ctx, handlerKey{}, &handlerWrapper{}), nil
}

type handlerKey struct{}

type handlerWrapper struct {
	ctx     context.Context
	handler remote.ServerTransHandler
}
