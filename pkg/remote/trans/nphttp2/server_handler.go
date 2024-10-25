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

package nphttp2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/endpoint"

	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	grpcTransport "github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/transport"
)

type svrTransHandlerFactory struct{}

// NewSvrTransHandlerFactory ...
func NewSvrTransHandlerFactory() remote.ServerTransHandlerFactory {
	return &svrTransHandlerFactory{}
}

func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	return newSvrTransHandler(opt)
}

func newSvrTransHandler(opt *remote.ServerOption) (*svrTransHandler, error) {
	return &svrTransHandler{
		opt:         opt,
		svcSearcher: opt.SvcSearcher,
		codec:       grpc.NewGRPCCodec(grpc.WithThriftCodec(opt.PayloadCodec)),
	}, nil
}

var _ remote.ServerTransHandler = &svrTransHandler{}

type svrTransHandler struct {
	opt         *remote.ServerOption
	svcSearcher remote.ServiceSearcher
	inkHdlFunc  remote.InnerServerEndpoint
	codec       remote.Codec
	transPipe   *remote.ServerTransPipeline
}

var prefaceReadAtMost = func() int {
	// min(len(ClientPreface), len(flagBuf))
	// len(flagBuf) = 2 * codec.Size32
	if 2*codec.Size32 < grpcTransport.ClientPrefaceLen {
		return 2 * codec.Size32
	}
	return grpcTransport.ClientPrefaceLen
}()

func (t *svrTransHandler) ProtocolMatch(ctx context.Context, conn net.Conn) (err error) {
	// Check the validity of client preface.
	npReader := conn.(interface{ Reader() netpoll.Reader }).Reader()
	// read at most avoid block
	preface, err := npReader.Peek(prefaceReadAtMost)
	if err != nil {
		return err
	}
	if bytes.Equal(preface[:prefaceReadAtMost], grpcTransport.ClientPreface[:prefaceReadAtMost]) {
		return nil
	}
	return errors.New("error protocol not match")
}

func (t *svrTransHandler) SetPipeline(p *remote.ServerTransPipeline) {
	t.transPipe = p
}

// 只 return write err
func (t *svrTransHandler) OnRead(ctx context.Context, conn net.Conn) error {
	svrTrans := ctx.Value(ctxKeySvrTransport).(*SvrTrans)
	tr := svrTrans.tr

	tr.HandleStreams(func(s *grpcTransport.Stream) {
		gofunc.GoFunc(ctx, func() {
			t.handleFunc(s, svrTrans, conn)
		})
	}, func(ctx context.Context, method string) context.Context {
		return ctx
	})
	return nil
}

func (t *svrTransHandler) handleFunc(s *grpcTransport.Stream, svrTrans *SvrTrans, conn net.Conn) {
	tr := svrTrans.tr
	ri := svrTrans.pool.Get().(rpcinfo.RPCInfo)
	rCtx := rpcinfo.NewCtxWithRPCInfo(s.Context(), ri)
	defer func() {
		// reset rpcinfo for performance (PR #584)
		if rpcinfo.PoolEnabled() {
			ri = t.opt.InitOrResetRPCInfoFunc(ri, conn.RemoteAddr())
			svrTrans.pool.Put(ri)
		}
	}()

	ink := ri.Invocation().(rpcinfo.InvocationSetter)
	sm := s.Method()
	if sm != "" && sm[0] == '/' {
		sm = sm[1:]
	}
	pos := strings.LastIndex(sm, "/")
	if pos == -1 {
		errDesc := fmt.Sprintf("malformed method name, method=%q", s.Method())
		tr.WriteStatus(s, status.New(codes.Internal, errDesc))
		return
	}
	methodName := sm[pos+1:]
	ink.SetMethodName(methodName)

	if mutableTo := rpcinfo.AsMutableEndpointInfo(ri.To()); mutableTo != nil {
		if err := mutableTo.SetMethod(methodName); err != nil {
			errDesc := fmt.Sprintf("setMethod failed in streaming, method=%s, error=%s", methodName, err.Error())
			_ = tr.WriteStatus(s, status.New(codes.Internal, errDesc))
			return
		}
	}

	var serviceName string
	idx := strings.LastIndex(sm[:pos], ".")
	if idx == -1 {
		ink.SetPackageName("")
		serviceName = sm[0:pos]
	} else {
		ink.SetPackageName(sm[:idx])
		serviceName = sm[idx+1 : pos]
	}
	ink.SetServiceName(serviceName)
	svcInfo := t.svcSearcher.SearchService(serviceName, methodName, true)
	var methodInfo serviceinfo.MethodInfo
	if svcInfo != nil {
		ink.SetServiceInfo(svcInfo)
		methodInfo = svcInfo.MethodInfo(methodName)
		ink.SetMethodInfo(methodInfo)
	}
	// set grpc transport flag before execute metahandler
	cfg := rpcinfo.AsMutableRPCConfig(ri.Config())
	cfg.SetTransportProtocol(transport.GRPC)
	cfg.SetPayloadCodec(getPayloadCodecFromContentType(s))
	var err error
	rCtx = t.startTracer(rCtx, ri)
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			if conn != nil {
				klog.CtxErrorf(rCtx, "KITEX: gRPC panic happened, close conn, remoteAddress=%s, error=%s\nstack=%s", conn.RemoteAddr(), panicErr, string(debug.Stack()))
			} else {
				klog.CtxErrorf(rCtx, "KITEX: gRPC panic happened, error=%v\nstack=%s", panicErr, string(debug.Stack()))
			}
		}
		t.finishTracer(rCtx, ri, err, panicErr)
	}()

	// set recv grpc compressor at server to decode the pack from client
	remote.SetRecvCompressor(ri, s.RecvCompress())
	// set send grpc compressor at server to encode reply pack
	remote.SetSendCompressor(ri, s.SendCompress())

	streamx := newServerStream(rCtx, ri, newServerConn(tr, s), t, conn)
	streamx.pipe.Initialize(streamx, t.transPipe)
	// inject streamx so that GetServerConn only relies on it
	rCtx = context.WithValue(rCtx, serverConnKey{}, streamx)
	// bind stream into ctx, in order to let user set header and trailer by provided api in meta_api.go
	rCtx = streaming.NewCtxWithStream(rCtx, streamx.GetGRPCStream())
	_ = t.transPipe.OnStream(rCtx, streamx)
}

func (t *svrTransHandler) OnStream(ctx context.Context, st streaming.ServerStream) error {
	streamx := st.(*serverStreamX)
	streamx.ctx = ctx
	ri := streamx.ri
	methodName := ri.Invocation().MethodName()
	serviceName := ri.Invocation().ServiceName()
	var err error
	if ri.Invocation().MethodInfo() == nil {
		unknownServiceHandlerFunc := t.opt.GRPCUnknownServiceHandler
		if unknownServiceHandlerFunc != nil {
			rpcinfo.Record(ctx, ri, stats.ServerHandleStart, nil)
			err = unknownServiceHandlerFunc(ctx, methodName, streamx.GetGRPCStream())
			if err != nil {
				err = kerrors.ErrBiz.WithCause(err)
			}
		} else {
			if ri.Invocation().ServiceInfo() == nil {
				err = remote.NewTransErrorWithMsg(remote.UnknownService, fmt.Sprintf("unknown service %s", serviceName))
			} else {
				err = remote.NewTransErrorWithMsg(remote.UnknownMethod, fmt.Sprintf("unknown method %s", methodName))
			}
		}
	} else {
		err = t.inkHdlFunc(ctx, streamx)
	}
	if err != nil {
		streamx.conn.tr.WriteStatus(streamx.conn.s, convertStatus(err))
		t.OnError(ctx, err, streamx.rawConn)
		return nil
	}
	if bizStatusErr := streamx.ri.Invocation().BizStatusErr(); bizStatusErr != nil {
		var st *status.Status
		if sterr, ok := bizStatusErr.(status.Iface); ok {
			st = sterr.GRPCStatus()
		} else {
			st = status.New(codes.Internal, bizStatusErr.BizMessage())
		}
		grpcStream := streamx.conn.s
		grpcStream.SetBizStatusErr(bizStatusErr)
		streamx.conn.tr.WriteStatus(grpcStream, st)
		return nil
	}
	streamx.conn.tr.WriteStatus(streamx.conn.s, status.New(codes.OK, ""))
	return nil
}

type svrTransKey int

const ctxKeySvrTransport svrTransKey = 1

type SvrTrans struct {
	tr   grpcTransport.ServerTransport
	pool *sync.Pool // value is rpcInfo
}

// 新连接建立时触发，主要用于服务端，对应 netpoll onPrepare
func (t *svrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	// set readTimeout to infinity to avoid streaming break
	// use keepalive to check the health of connection
	if npConn, ok := conn.(netpoll.Connection); ok {
		npConn.SetReadTimeout(grpcTransport.Infinity)
	} else {
		conn.SetReadDeadline(time.Now().Add(grpcTransport.Infinity))
	}

	tr, err := grpcTransport.NewServerTransport(ctx, conn, t.opt.GRPCCfg)
	if err != nil {
		return nil, err
	}
	pool := &sync.Pool{
		New: func() interface{} {
			// init rpcinfo
			ri := t.opt.InitOrResetRPCInfoFunc(nil, conn.RemoteAddr())
			return ri
		},
	}
	ctx = context.WithValue(ctx, ctxKeySvrTransport, &SvrTrans{tr: tr, pool: pool})
	return ctx, nil
}

// 连接关闭时回调
func (t *svrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	tr := ctx.Value(ctxKeySvrTransport).(*SvrTrans).tr
	tr.Close()
}

// 传输层 error 回调
func (t *svrTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	var de *kerrors.DetailedError
	if ok := errors.As(err, &de); ok && de.Stack() != "" {
		klog.CtxErrorf(ctx, "KITEX: processing gRPC request error, remoteAddr=%s, error=%s\nstack=%s", conn.RemoteAddr(), err.Error(), de.Stack())
	} else {
		klog.CtxErrorf(ctx, "KITEX: processing gRPC request error, remoteAddr=%s, error=%s", conn.RemoteAddr(), err.Error())
	}
}

func (t *svrTransHandler) SetInvokeHandleFunc(streamHdlFunc remote.InnerServerEndpoint, unaryHdlFunc endpoint.Endpoint) {
	t.inkHdlFunc = streamHdlFunc
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

func getPayloadCodecFromContentType(s *grpcTransport.Stream) serviceinfo.PayloadCodec {
	// TODO: handle other protocols in the future. currently only supports grpc
	subType := s.ContentSubtype()
	switch subType {
	case contentSubTypeThrift:
		return serviceinfo.Thrift
	default:
		return serviceinfo.Protobuf
	}
}
