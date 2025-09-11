/*
 * Copyright 2025 CloudWeGo Authors
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

package ttstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"sync"

	"github.com/bytedance/gopkg/cloud/metainfo"

	"github.com/cloudwego/kitex/pkg/consts"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream/ktx"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	ktransport "github.com/cloudwego/kitex/transport"
)

type StreamInfo interface {
	Service() string
	Method() string
	TransportProtocol() ktransport.Protocol
}

/*  trans_server.go only use the following interface in remote.ServerTransHandler:
- OnRead
- OnActive
- OnInactive
- OnError
- GracefulShutdown: used by type assert

Other interface is used by trans pipeline
*/

type svrTransHandlerFactory struct{}

// NewSvrTransHandlerFactory ...
func NewSvrTransHandlerFactory() remote.ServerTransHandlerFactory {
	return &svrTransHandlerFactory{}
}

func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	ttOpts := make([]ServerProviderOption, 0, len(opt.TTHeaderStreamingOptions.TransportOptions))
	for _, o := range opt.TTHeaderStreamingOptions.TransportOptions {
		if opt, ok := o.(ServerProviderOption); ok {
			ttOpts = append(ttOpts, opt)
		}
	}
	return &svrTransHandler{
		opt:      opt,
		provider: newServerProvider(ttOpts...),
	}, nil
}

var (
	_                   remote.ServerTransHandler = &svrTransHandler{}
	errProtocolNotMatch                           = errors.New("protocol not match")
)

type svrTransHandler struct {
	opt        *remote.ServerOption
	provider   *serverProvider
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

// OnRead control the connection level lifecycle.
// only when OnRead return, netpoll can close the connection buffer
func (t *svrTransHandler) OnRead(ctx context.Context, conn net.Conn) (err error) {
	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
		_, nerr := t.provider.OnInactive(ctx, conn)
		if err == nil && nerr != nil {
			err = nerr
		}
		if err != nil {
			klog.CtxErrorf(ctx, "KITEX: stream OnRead return: err=%v", err)
		}
	}()
	// connection level goroutine
	for {
		nctx, st, nerr := t.provider.OnStream(ctx, conn)
		if nerr != nil {
			if errors.Is(nerr, io.EOF) {
				return nil
			}
			klog.CtxErrorf(ctx, "KITEX: OnStream failed: err=%v", nerr)
			return nerr
		}
		wg.Add(1)
		// stream level goroutine
		gofunc.GoFunc(nctx, func() {
			defer wg.Done()
			err := t.OnStream(nctx, conn, st)
			if err != nil && !errors.Is(err, io.EOF) {
				t.OnError(nctx, err, conn)
			}
		})
	}
}

// OnStream
// - create  server stream
// - process server stream
// - close   server stream
func (t *svrTransHandler) OnStream(ctx context.Context, conn net.Conn, st *stream) (err error) {
	ri := t.opt.InitOrResetRPCInfoFunc(nil, conn.RemoteAddr())
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	defer func() {
		if rpcinfo.PoolEnabled() {
			ri = t.opt.InitOrResetRPCInfoFunc(ri, conn.RemoteAddr())
			// TODO: rpcinfo pool
		}
	}()

	ink := ri.Invocation().(rpcinfo.InvocationSetter)
	sinfo := t.opt.SvcSearcher.SearchService(st.Service(), st.Method(), false)
	if sinfo == nil {
		return remote.NewTransErrorWithMsg(remote.UnknownService, fmt.Sprintf("unknown service %s", st.Service()))
	}
	minfo := sinfo.MethodInfo(st.Method())
	if minfo == nil {
		return remote.NewTransErrorWithMsg(remote.UnknownMethod, fmt.Sprintf("unknown method %s", st.Method()))
	}
	ink.SetServiceName(sinfo.ServiceName)
	ink.SetMethodName(st.Method())
	ink.SetStreamingMode(minfo.StreamingMode())
	if mutableTo := rpcinfo.AsMutableEndpointInfo(ri.To()); mutableTo != nil {
		_ = mutableTo.SetMethod(st.Method())
		// this method name will be used as from method if a new RPC call is invoked in this handler.
		// ping-pong relies on transMetaHandler.OnMessage to inject but streaming does not trigger.
		//
		//nolint:staticcheck // SA1029: consts.CtxKeyMethod has been used and we just follow it
		ctx = context.WithValue(ctx, consts.CtxKeyMethod, st.Method())
	}
	rpcinfo.AsMutableRPCConfig(ri.Config()).SetTransportProtocol(st.TransportProtocol())

	// headerHandler return a new stream level ctx
	// it contains rpcinfo modified by HeaderHandler
	if t.provider.headerHandler != nil {
		ctx, err = t.provider.headerHandler.OnReadStream(ctx, st.meta, st.header)
		if err != nil {
			return err
		}
	}
	// register metainfo into ctx
	ctx = metainfo.SetMetaInfoFromMap(ctx, st.header)
	ss := newServerStream(st)

	// cancel ctx when OnStreamFinish
	ctx, cancelFunc := ktx.WithCancel(ctx)
	ctx = context.WithValue(ctx, serverStreamCancelCtxKey{}, cancelFunc)

	ctx = t.startTracer(ctx, ri)
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			if conn != nil {
				klog.CtxErrorf(ctx, "KITEX: ttstream panic happened, close conn, remoteAddress=%s, error=%s\nstack=%s", conn.RemoteAddr(), panicErr, string(debug.Stack()))
			} else {
				klog.CtxErrorf(ctx, "KITEX: ttstream panic happened, error=%v\nstack=%s", panicErr, string(debug.Stack()))
			}
		}
		t.finishTracer(ctx, ri, err, panicErr)
	}()

	args := &streaming.Args{
		ServerStream: ss,
	}
	if err = t.inkHdlFunc(ctx, args, nil); err != nil {
		// treat err thrown by invoking handler as the final err, ignore the err returned by OnStreamFinish
		t.provider.OnStreamFinish(ctx, ss, err)
		return
	}
	if bizErr := ri.Invocation().BizStatusErr(); bizErr != nil {
		// when biz err thrown, treat the err returned by OnStreamFinish as the final err
		ctx, err = t.provider.OnStreamFinish(ctx, ss, bizErr)
		return
	}
	// there is no invoking handler err or biz err, treat the err returned by OnStreamFinish as the final err
	ctx, err = t.provider.OnStreamFinish(ctx, ss, nil)
	return err
}

func (t *svrTransHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) (nctx context.Context, err error) {
	return ctx, nil
}

func (t *svrTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) (nctx context.Context, err error) {
	return ctx, nil
}

func (t *svrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
}

func (t *svrTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	var de *kerrors.DetailedError
	if ok := errors.As(err, &de); ok && de.Stack() != "" {
		klog.CtxErrorf(ctx, "KITEX: processing ttstream request error, remoteAddr=%s, error=%s\nstack=%s", conn.RemoteAddr(), err.Error(), de.Stack())
	} else {
		klog.CtxErrorf(ctx, "KITEX: processing ttstream request error, remoteAddr=%s, error=%s", conn.RemoteAddr(), err.Error())
	}
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
