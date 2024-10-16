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
	"io"
	"net"
	"runtime/debug"
	"time"

	"github.com/cloudwego/kitex/internal/wpool"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
)

/* 实际上 remote.ServerTransHandler 真正被 trans_server.go 使用的接口只有：
- OnRead
- OnActive
- OnInactive
- OnError
- GracefulShutdown: assert 方式使用

其他接口实际上最终是用来去组装了 transpipeline ....
*/

var streamWorkerPool = wpool.New(128, time.Second)

type svrTransHandlerFactory struct {
	provider streamx.ServerProvider
}

// NewSvrTransHandlerFactory ...
func NewSvrTransHandlerFactory(provider streamx.ServerProvider) remote.ServerTransHandlerFactory {
	sp := streamx.NewServerProvider(provider) // wrapped server provider
	return &svrTransHandlerFactory{provider: sp}
}

func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	return &svrTransHandler{
		opt:      opt,
		provider: f.provider,
	}, nil
}

var (
	_                   remote.ServerTransHandler = &svrTransHandler{}
	errProtocolNotMatch                           = errors.New("protocol not match")
)

type svrTransHandler struct {
	opt        *remote.ServerOption
	provider   streamx.ServerProvider
	inkHdlFunc endpoint.Endpoint
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
	return ctx, nil
}

func (t *svrTransHandler) OnRead(ctx context.Context, conn net.Conn) error {
	// connection level goroutine
	for {
		nctx, ss, nerr := t.provider.OnStream(ctx, conn)
		if nerr != nil {
			if errors.Is(nerr, io.EOF) {
				return nil
			}
			klog.CtxErrorf(ctx, "KITEX: OnStream failed: err=%v", nerr)
			return nerr
		}
		// stream level goroutine
		streamWorkerPool.GoCtx(ctx, func() {
			err := t.OnStream(nctx, conn, ss)
			if err != nil && !errors.Is(err, io.EOF) {
				klog.CtxErrorf(ctx, "KITEX: stream ReadStream failed: err=%v", nerr)
			}
		})
	}
}

// OnStream
// - create  server stream
// - process server stream
// - close   server stream
func (t *svrTransHandler) OnStream(ctx context.Context, conn net.Conn, ss streamx.ServerStream) (err error) {
	// inkHdlFunc 包含了所有中间件 + 用户 serviceInfo.methodHandler
	// 这里 streamx 依然会复用原本的 server endpoint.Endpoint 中间件，因为他们都不会单独去取 req/res 的值
	// 无法在保留现有 streaming 功能的情况下，彻底弃用 endpoint.Endpoint , 所以这里依然使用 endpoint 接口
	// 但是对用户 API ，做了单独的封装。把这部分脏逻辑仅暴露在框架中。
	sargs := streamx.NewStreamArgs(ss)
	ctx = streamx.WithStreamArgsContext(ctx, sargs)

	ri := t.opt.InitOrResetRPCInfoFunc(nil, conn.RemoteAddr())
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	defer func() {
		if rpcinfo.PoolEnabled() {
			ri = t.opt.InitOrResetRPCInfoFunc(ri, conn.RemoteAddr())
			// TODO: rpcinfo pool
		}
	}()

	ink := ri.Invocation().(rpcinfo.InvocationSetter)
	ink.SetServiceName(ss.Service())
	ink.SetMethodName(ss.Method())
	if mutableTo := rpcinfo.AsMutableEndpointInfo(ri.To()); mutableTo != nil {
		_ = mutableTo.SetMethod(ss.Method())
	}

	ctx = t.startTracer(ctx, ri)
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			if conn != nil {
				klog.CtxErrorf(ctx, "KITEX: streamx panic happened, close conn, remoteAddress=%s, error=%s\nstack=%s", conn.RemoteAddr(), panicErr, string(debug.Stack()))
			} else {
				klog.CtxErrorf(ctx, "KITEX: streamx panic happened, error=%v\nstack=%s", panicErr, string(debug.Stack()))
			}
		}
		t.finishTracer(ctx, ri, err, panicErr)
	}()

	reqArgs := streamx.NewStreamReqArgs(nil)
	resArgs := streamx.NewStreamResArgs(nil)
	serr := t.inkHdlFunc(ctx, reqArgs, resArgs)
	if serr == nil {
		if bizErr := ri.Invocation().BizStatusErr(); bizErr != nil {
			serr = bizErr
		}
	}
	ctx, err = t.provider.OnStreamFinish(ctx, ss, serr)
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
	_, _ = t.provider.OnInactive(ctx, conn)
}

func (t *svrTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
}

func (t *svrTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	return ctx, nil
}

func (t *svrTransHandler) SetPipeline(pipeline *remote.TransPipeline) {
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
