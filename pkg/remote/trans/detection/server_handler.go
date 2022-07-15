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
	"context"
	"net"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"

	"github.com/cloudwego/kitex/pkg/endpoint"
	transNetpoll "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
)

type DetectableServerTransHandler interface {
	remote.ServerTransHandler
	ProtocolMatch(ctx context.Context, conn net.Conn) (err error)
}

var registeredServerTransHandlerFactory []remote.ServerTransHandlerFactory

func RegisterServerTransHandlerFactory(f remote.ServerTransHandlerFactory) {
	registeredServerTransHandlerFactory = append(registeredServerTransHandlerFactory, f)
}

// NewSvrTransHandlerFactory detection factory construction
func NewSvrTransHandlerFactory() remote.ServerTransHandlerFactory {
	return &svrTransHandlerFactory{
		registered: registeredServerTransHandlerFactory,
		netpoll:    transNetpoll.NewSvrTransHandlerFactory(),
	}
}

type svrTransHandlerFactory struct {
	registered []remote.ServerTransHandlerFactory
	netpoll    remote.ServerTransHandlerFactory
}

func (f *svrTransHandlerFactory) MuxEnabled() bool {
	return false
}

func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	t := &svrTransHandler{}
	var err error
	for i := range f.registered {
		h, err := f.registered[i].NewTransHandler(opt)
		if err != nil {
			return nil, err
		}
		handler, ok := h.(DetectableServerTransHandler)
		if !ok {
			klog.Errorf("KITEX: failed to append detection server trans handler: %T", h)
			continue
		}
		t.registered = append(t.registered, handler)
	}
	if t.netpoll, err = f.netpoll.NewTransHandler(opt); err != nil {
		return nil, err
	}
	return t, nil
}

type svrTransHandler struct {
	registered []DetectableServerTransHandler
	netpoll    remote.ServerTransHandler
}

func (t *svrTransHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) error {
	return t.which(ctx).Write(ctx, conn, send)
}

func (t *svrTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) error {
	return t.which(ctx).Read(ctx, conn, msg)
}

func (t *svrTransHandler) OnRead(ctx context.Context, conn net.Conn) (err error) {
	// only need detect once when connection is reused
	r := ctx.Value(handlerKey{}).(*handlerWrapper)
	if r.handler != nil {
		return r.handler.OnRead(r.ctx, conn)
	}
	// compare preface one by one
	which := t.netpoll
	for i := range t.registered {
		if t.registered[i].ProtocolMatch(ctx, conn) == nil {
			which = t.registered[i]
			break
		}
	}
	ctx, err = which.OnActive(ctx, conn)
	if err != nil {
		return err
	}
	r.ctx, r.handler = ctx, which
	return which.OnRead(ctx, conn)
}

func (t *svrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// t.http2 should use the ctx returned by OnActive in r.ctx
	r := ctx.Value(handlerKey{}).(*handlerWrapper)
	if r.ctx != nil {
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
	for i := range t.registered {
		t.registered[i].SetPipeline(pipeline)
	}
	t.netpoll.SetPipeline(pipeline)
}

func (t *svrTransHandler) SetInvokeHandleFunc(inkHdlFunc endpoint.Endpoint) {
	for i := range t.registered {
		if s, ok := t.registered[i].(remote.InvokeHandleFuncSetter); ok {
			s.SetInvokeHandleFunc(inkHdlFunc)
		}
	}
	if s, ok := t.netpoll.(remote.InvokeHandleFuncSetter); ok {
		s.SetInvokeHandleFunc(inkHdlFunc)
	}
}

func (t *svrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
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
