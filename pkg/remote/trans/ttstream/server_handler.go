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
	"strconv"
	"sync"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/netpoll"

	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream/ktx"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/utils"
)

var streamingBidirectionalCtx = igeneric.WithGenericStreamingMode(context.Background(), serviceinfo.StreamingBidirectional)

type (
	serverTransCtxKey        struct{}
	serverStreamCancelCtxKey struct{}
)

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

func (f *svrTransHandlerFactory) NewTransHandler(opts *remote.ServerOption) (remote.ServerTransHandler, error) {
	sp := &svrTransHandler{
		opt: opts,
	}
	for _, o := range opts.TTHeaderStreamingOptions.TransportOptions {
		if opt, ok := o.(ServerHandlerOption); ok {
			opt(sp)
		}
	}
	return sp, nil
}

var (
	_                   remote.ServerTransHandler = &svrTransHandler{}
	errProtocolNotMatch                           = errors.New("protocol not match")
)

type svrTransHandler struct {
	opt           *remote.ServerOption
	inkHdlFunc    endpoint.Endpoint
	headerHandler HeaderFrameReadHandler
	traceCtl      *rpcinfo.TraceController
}

func (t *svrTransHandler) SetInvokeHandleFunc(inkHdlFunc endpoint.Endpoint) {
	t.inkHdlFunc = inkHdlFunc
}

func (t *svrTransHandler) ProtocolMatch(ctx context.Context, conn net.Conn) (err error) {
	nconn, ok := conn.(netpoll.Connection)
	if !ok {
		return errProtocolNotMatch
	}
	data, err := nconn.Reader().Peek(8)
	if err != nil {
		return errProtocolNotMatch
	}
	if !ttheader.IsStreaming(data) {
		return errProtocolNotMatch
	}
	return nil
}

type onDisConnectSetter interface {
	SetOnDisconnect(onDisconnect netpoll.OnDisconnect) error
}

// OnActive will be called when a connection accepted
func (t *svrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	nconn := conn.(netpoll.Connection)
	trans := newServerTransport(nconn, t.traceCtl)
	_ = nconn.(onDisConnectSetter).SetOnDisconnect(func(ctx context.Context, connection netpoll.Connection) {
		// server only close transport when peer connection closed
		_ = trans.Close(nil)
	})
	return context.WithValue(ctx, serverTransCtxKey{}, trans), nil
}

// OnRead control the connection level lifecycle.
// only when OnRead return, netpoll can close the connection buffer
func (t *svrTransHandler) OnRead(ctx context.Context, conn net.Conn) (err error) {
	var wg sync.WaitGroup
	defer func() {
		wg.Wait()
		trans, _ := ctx.Value(serverTransCtxKey{}).(*transport)
		if trans != nil {
			trans.WaitClosed()
		}
		if errors.Is(err, io.EOF) {
			err = nil
		}
	}()
	// connection level goroutine
	for {
		trans, _ := ctx.Value(serverTransCtxKey{}).(*transport)
		if trans == nil {
			err = fmt.Errorf("server transport is nil")
			return
		}
		var st *stream
		// ReadStream will block until a stream coming or conn return error
		st, err = trans.ReadStream(ctx)
		if err != nil {
			return
		}
		wg.Add(1)
		// stream level goroutine
		gofunc.GoFunc(ctx, func() {
			defer wg.Done()
			err := t.OnStream(ctx, conn, st)
			if err != nil && !errors.Is(err, io.EOF) {
				t.OnError(ctx, err, conn)
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
	// TODO: support protobuf codec, and make `strict` true when combine service is not supported.
	sinfo := t.opt.SvcSearcher.SearchService(st.Service(), st.Method(), false, serviceinfo.Thrift)
	if sinfo == nil {
		err = remote.NewTransErrorWithMsg(remote.UnknownService, fmt.Sprintf("unknown service %s", st.Service()))
		return
	}
	// TODO: pass-through grpc streaming mode.
	minfo := sinfo.MethodInfo(streamingBidirectionalCtx, st.Method())
	if minfo == nil {
		err = remote.NewTransErrorWithMsg(remote.UnknownMethod, fmt.Sprintf("unknown method %s", st.Method()))
		return
	}
	ink.SetServiceName(sinfo.ServiceName)
	ink.SetMethodName(st.Method())
	ink.SetMethodInfo(minfo)
	ink.SetStreamingMode(minfo.StreamingMode())
	if mutableTo := rpcinfo.AsMutableEndpointInfo(ri.To()); mutableTo != nil {
		_ = mutableTo.SetMethod(st.Method())
	}
	rpcinfo.AsMutableRPCConfig(ri.Config()).SetTransportProtocol(st.TransportProtocol())

	// headerHandler return a new stream level ctx
	// it contains rpcinfo modified by HeaderHandler
	if t.headerHandler != nil {
		ctx, err = t.headerHandler.OnReadStream(ctx, st.meta, st.header)
		if err != nil {
			return
		}
	}
	// register metainfo into ctx
	ctx = metainfo.SetMetaInfoFromMap(ctx, st.header)
	ss := newServerStream(st)

	// cancel ctx when OnStreamFinish
	ctx, cancelFunc := ktx.WithCancel(ctx)
	ctx = context.WithValue(ctx, serverStreamCancelCtxKey{}, cancelFunc)

	ctx = t.startTracer(ctx, ri)
	st.setTraceContext(ctx)
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			if conn != nil {
				klog.CtxErrorf(ctx, "KITEX: ttstream panic happened, close conn, remoteAddress=%s, error=%s\nstack=%s", conn.RemoteAddr(), panicErr, string(debug.Stack()))
			} else {
				klog.CtxErrorf(ctx, "KITEX: ttstream panic happened, error=%v\nstack=%s", panicErr, string(debug.Stack()))
			}
		}
		st.finishTrace()
		t.finishTracer(ctx, ri, err, panicErr)
	}()
	args := &streaming.Args{
		ServerStream: ss,
	}
	if err = t.inkHdlFunc(ctx, args, nil); err != nil {
		// treat err thrown by invoking handler as the final err, ignore the err returned by OnStreamFinish
		_, _ = t.OnStreamFinish(ctx, ss, err)
		return
	}
	if bizErr := ri.Invocation().BizStatusErr(); bizErr != nil {
		// when biz err thrown, treat the err returned by OnStreamFinish as the final err
		ctx, err = t.OnStreamFinish(ctx, ss, bizErr)
		return
	}
	// there is no invoking handler err or biz err, treat the err returned by OnStreamFinish as the final err
	ctx, err = t.OnStreamFinish(ctx, ss, nil)
	return
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

func (t *svrTransHandler) OnStreamFinish(ctx context.Context, ss streaming.ServerStream, err error) (context.Context, error) {
	sst := ss.(*serverStream)
	var exception error
	if err != nil {
		switch terr := err.(type) {
		case kerrors.BizStatusErrorIface:
			bizStatus := strconv.Itoa(int(terr.BizStatusCode()))
			bizMsg := terr.BizMessage()
			if terr.BizExtra() == nil {
				err = sst.writeTrailer(streaming.Trailer{
					"biz-status":  bizStatus,
					"biz-message": bizMsg,
				})
			} else {
				bizExtra, _ := utils.Map2JSONStr(terr.BizExtra())
				err = sst.writeTrailer(streaming.Trailer{
					"biz-status":  bizStatus,
					"biz-message": bizMsg,
					"biz-extra":   bizExtra,
				})
			}
			if err != nil {
				return nil, err
			}
			exception = nil
		case *thrift.ApplicationException:
			exception = terr
		case tException:
			exception = thrift.NewApplicationException(terr.TypeId(), terr.Error())
		default:
			exception = thrift.NewApplicationException(remote.InternalError, terr.Error())
		}
	}
	// server stream CloseSend will send the trailer with payload
	if err = sst.CloseSend(exception); err != nil {
		return nil, err
	}

	cancelFunc, _ := ctx.Value(serverStreamCancelCtxKey{}).(context.CancelFunc)
	if cancelFunc != nil {
		cancelFunc()
	}

	return ctx, nil
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
