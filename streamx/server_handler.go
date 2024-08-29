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

package streamx

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
	"net"
	"runtime/debug"
	"sync"
)

/* 实际上 remote.ServerTransHandler 真正被 trans_server.go 使用的接口只有：
- OnRead
- OnActive
- OnInactive
- OnError
- GracefulShutdown: assert 方式使用

其他接口实际上最终是用来去组装了 transpipeline ....
*/

type svrTransHandlerFactory struct{}

// NewSvrTransHandlerFactory ...
func NewSvrTransHandlerFactory() remote.ServerTransHandlerFactory {
	return &svrTransHandlerFactory{}
}

func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	return newSvrTransHandler(opt)
}

func newSvrTransHandler(opt *remote.ServerOption) (*svrTransHandler, error) {
	sp, ok := opt.Provider.(ServerProvider)
	if !ok {
		return nil, nil
	}
	sp = NewServerProvider(sp) // wrapped server provider
	return &svrTransHandler{
		opt:      opt,
		provider: sp,
	}, nil
}

var _ remote.ServerTransHandler = &svrTransHandler{}
var errProtocolNotMatch = errors.New("protocol not match")

type svrTransHandler struct {
	opt         *remote.ServerOption
	provider    ServerProvider
	inkHdlFunc  endpoint.Endpoint
	svcSearcher remote.ServiceSearcher
	streamMap   sync.Map
}

func (t *svrTransHandler) SetInvokeHandleFunc(inkHdlFunc endpoint.Endpoint) {
	t.inkHdlFunc = inkHdlFunc
}

func (t *svrTransHandler) ProtocolMatch(ctx context.Context, conn net.Conn) (err error) {
	if t.provider.Available(ctx, conn) {
		return nil
	}
	return errProtocolNotMatch
}

func (t *svrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	var err error
	ctx, err = t.provider.OnActive(ctx, conn)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			nctx, ss, nerr := t.provider.OnStream(ctx, conn)
			if nerr != nil {
				klog.CtxErrorf(ctx, "KITEX: OnStream failed: err=%v", nerr)
				return
			}
			go func() {
				nerr = t.OnStream(nctx, conn, ss)
				if nerr != nil {
					klog.CtxErrorf(ctx, "KITEX: stream ReadStream failed: err=%v", err)
					return
				}
			}()
		}
	}()
	return ctx, nil
}

func (t *svrTransHandler) OnRead(ctx context.Context, conn net.Conn) error {
	return nil
}

// OnStream
// - create  server stream
// - process server stream
// - close   server stream
func (t *svrTransHandler) OnStream(ctx context.Context, conn net.Conn, ss ServerStream) (err error) {
	// inkHdlFunc 包含了所有中间件 + 用户 serviceInfo.methodHandler
	// 这里 streamx 依然会复用原本的 server endpoint.Endpoint 中间件，因为他们都不会单独去取 req/res 的值
	// 无法在保留现有 streaming 功能的情况下，彻底弃用 endpoint.Endpoint , 所以这里依然使用 endpoint 接口
	// 但是对用户 API ，做了单独的封装。把这部分脏逻辑仅暴露在框架中。
	method := ss.Method()
	//minfo := t.opt.TargetSvcInfo.Methods[method]
	//args := minfo.NewArgs()
	//result := minfo.NewResult()
	sargs := NewStreamArgs(ss)
	ctx = WithStreamArgsContext(ctx, sargs)

	ri := t.opt.InitOrResetRPCInfoFunc(nil, conn.RemoteAddr())
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	defer func() {
		if rpcinfo.PoolEnabled() {
			ri = t.opt.InitOrResetRPCInfoFunc(ri, conn.RemoteAddr())
			// TODO: rpcinfo pool
		}
	}()

	ink := ri.Invocation().(rpcinfo.InvocationSetter)
	ink.SetServiceName(t.opt.TargetSvcInfo.ServiceName)
	ink.SetMethodName(method)
	if mutableTo := rpcinfo.AsMutableEndpointInfo(ri.To()); mutableTo != nil {
		_ = mutableTo.SetMethod(method)
	}

	// set grpc transport flag before execute metahandler
	_ = rpcinfo.AsMutableRPCConfig(ri.Config()).SetTransportProtocol(transport.JSONRPC)

	ctx = t.startTracer(ctx, ri)
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			if conn != nil {
				klog.CtxErrorf(ctx, "KITEX: jsonRPC panic happened, close conn, remoteAddress=%s, error=%s\nstack=%s", conn.RemoteAddr(), panicErr, string(debug.Stack()))
			} else {
				klog.CtxErrorf(ctx, "KITEX: jsonRPC panic happened, error=%v\nstack=%s", panicErr, string(debug.Stack()))
			}
		}
		t.finishTracer(ctx, ri, err, panicErr)
	}()

	reqArgs := NewStreamReqArgs(nil)
	resArgs := NewStreamResArgs(nil)
	serr := t.inkHdlFunc(ctx, reqArgs, resArgs)
	ctx, err = t.provider.OnStreamFinish(ctx, ss)
	if err == nil && serr != nil {
		err = serr
	}
	return err
}

func (t *svrTransHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) (nctx context.Context, err error) {
	return ctx, nil
}

func (t *svrTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) (nctx context.Context, err error) {
	return ctx, nil
}

func (t *svrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	return
}

func (t *svrTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	return
}

func (t *svrTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	return ctx, nil
}

func (t *svrTransHandler) SetPipeline(pipeline *remote.TransPipeline) {
	return
}

func (t *svrTransHandler) startTracer(ctx context.Context, ri rpcinfo.RPCInfo) context.Context {
	c := t.opt.TracerCtl.DoStart(ctx, ri)
	return c
}

func (t *svrTransHandler) finishTracer(ctx context.Context, ri rpcinfo.RPCInfo, err error, panicErr interface{}) {
	rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats())
	if rpcStats == nil {
		return
	}
	if panicErr != nil {
		rpcStats.SetPanicked(panicErr)
	}
	t.opt.TracerCtl.DoFinish(ctx, ri, err)
	rpcStats.Reset()
}
