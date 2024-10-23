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

package trans

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/debug"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
)

var errHeartbeatMessage = errors.New("heartbeat message")

// NewDefaultSvrTransHandler to provide default impl of serverTransHandler, it can be reused in netpoll, shm-ipc, framework-sdk extensions
func NewDefaultSvrTransHandler(opt *remote.ServerOption, ext Extension) (remote.ServerTransHandler, error) {
	svrHdlr := &serverTransHandler{
		opt:           opt,
		codec:         opt.Codec,
		svcSearcher:   opt.SvcSearcher,
		targetSvcInfo: opt.TargetSvcInfo,
		ext:           ext,
	}
	if svrHdlr.opt.TracerCtl == nil {
		// init TraceCtl when it is nil, or it will lead some unit tests panic
		svrHdlr.opt.TracerCtl = &rpcinfo.TraceController{}
	}
	return svrHdlr, nil
}

type serverTransHandler struct {
	opt           *remote.ServerOption
	svcSearcher   remote.ServiceSearcher
	targetSvcInfo *serviceinfo.ServiceInfo
	inkHdlFunc    endpoint.Endpoint
	codec         remote.Codec
	transPipe     *remote.ServerTransPipeline
	ext           Extension
}

func (t *serverTransHandler) newCtxWithRPCInfo(ctx context.Context, conn net.Conn) (context.Context, rpcinfo.RPCInfo) {
	if rpcinfo.PoolEnabled() { // reuse per-connection rpcinfo
		return ctx, rpcinfo.GetRPCInfo(ctx)
		// delayed reinitialize for faster response
	}
	// new rpcinfo if reuse is disabled
	ri := t.opt.InitOrResetRPCInfoFunc(nil, conn.RemoteAddr())
	if t.targetSvcInfo != nil {
		ri.Invocation().(rpcinfo.InvocationSetter).SetServiceInfo(t.targetSvcInfo)
	} else {
		ri = &PingPongRPCInfo{RPCInfo: ri, svcSearcher: t.svcSearcher}
	}
	return rpcinfo.NewCtxWithRPCInfo(ctx, ri), ri
}

// OnRead implements the remote.ServerTransHandler interface.
// The connection should be closed after returning error.
func (t *serverTransHandler) OnRead(ctx context.Context, conn net.Conn) (err error) {
	ctx, ri := t.newCtxWithRPCInfo(ctx, conn)
	st := newServerStream(ctx, conn, t, ri)
	streamPipe := remote.NewServerStreamPipeline(st, t.transPipe)
	st.SetPipeline(streamPipe)
	return t.transPipe.OnStream(ctx, st)
}

func (t *serverTransHandler) OnStream(ctx context.Context, stream streaming.ServerStream) (err error) {
	st := stream.(*svrStream)
	conn := st.conn
	ri := st.ri
	t.ext.SetReadTimeout(ctx, conn, ri.Config(), remote.Server)
	closeConnOutsideIfErr := true
	defer func() {
		panicErr := recover()
		var wrapErr error
		if panicErr != nil {
			stack := string(debug.Stack())
			if conn != nil {
				ri := rpcinfo.GetRPCInfo(ctx)
				rService, rAddr := getRemoteInfo(ri, conn)
				klog.CtxErrorf(ctx, "KITEX: panic happened, remoteAddress=%s, remoteService=%s, error=%v\nstack=%s", rAddr, rService, panicErr, stack)
			} else {
				klog.CtxErrorf(ctx, "KITEX: panic happened, error=%v\nstack=%s", panicErr, stack)
			}
			if err != nil {
				wrapErr = kerrors.ErrPanic.WithCauseAndStack(fmt.Errorf("[happened in OnRead] %s, last error=%s", panicErr, err.Error()), stack)
			} else {
				wrapErr = kerrors.ErrPanic.WithCauseAndStack(fmt.Errorf("[happened in OnRead] %s", panicErr), stack)
			}
		}
		t.finishTracer(ctx, ri, err, panicErr)
		t.finishProfiler(ctx)
		// reset rpcinfo for reuse
		if rpcinfo.PoolEnabled() {
			t.opt.InitOrResetRPCInfoFunc(ri, conn.RemoteAddr())
		}
		if wrapErr != nil {
			err = wrapErr
		}
		if err != nil && !closeConnOutsideIfErr {
			err = nil
		}
	}()
	ctx = t.startTracer(ctx, ri)
	ctx = t.startProfiler(ctx)
	args := &serviceinfo.PingPongCompatibleMethodArgs{}
	err = st.RecvMsg(args)
	if err != nil {
		if errors.Is(err, errHeartbeatMessage) {
			sendMsg := remote.NewMessage(nil, nil, ri, remote.Heartbeat, remote.Server)
			ctx, err = st.pipe.Write(ctx, sendMsg)
			return err
		}
		t.writeErrorReplyIfNeeded(ctx, conn, st, err, ri)
		// t.OnError(ctx, err, conn) will be executed at outer function when transServer close the conn
		return
	}
	methodInfo := ri.Invocation().MethodInfo()
	var res interface{}
	if !methodInfo.OneWay() {
		res = methodInfo.NewResult()
	}
	err = t.inkHdlFunc(ctx, args.Data, res)
	if err != nil {
		// error cannot be wrapped to print here, so it must exec before NewTransError
		t.OnError(ctx, err, conn)
		err = remote.NewTransError(remote.InternalError, err)
		if closeConn := t.writeErrorReplyIfNeeded(ctx, conn, st, err, ri); closeConn {
			return err
		}
		// connection don't need to be closed when the error is return by the server handler
		closeConnOutsideIfErr = false
		return
	}
	if err = st.SendMsg(res); err != nil {
		// t.OnError(ctx, err, conn) will be executed at outer function when transServer close the conn
		return err
	}
	return
}

// OnActive implements the remote.ServerTransHandler interface.
func (t *serverTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	// init rpcinfo
	ri := t.opt.InitOrResetRPCInfoFunc(nil, conn.RemoteAddr())
	return rpcinfo.NewCtxWithRPCInfo(ctx, ri), nil
}

// OnInactive implements the remote.ServerTransHandler interface.
func (t *serverTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// recycle rpcinfo
	rpcinfo.PutRPCInfo(rpcinfo.GetRPCInfo(ctx))
}

// OnError implements the remote.ServerTransHandler interface.
func (t *serverTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	ri := rpcinfo.GetRPCInfo(ctx)
	rService, rAddr := getRemoteInfo(ri, conn)
	if t.ext.IsRemoteClosedErr(err) {
		// it should not regard error which cause by remote connection closed as server error
		if ri == nil {
			return
		}
		remote := rpcinfo.AsMutableEndpointInfo(ri.From())
		remote.SetTag(rpcinfo.RemoteClosedTag, "1")
	} else {
		var de *kerrors.DetailedError
		if ok := errors.As(err, &de); ok && de.Stack() != "" {
			klog.CtxErrorf(ctx, "KITEX: processing request error, remoteService=%s, remoteAddr=%v, error=%s\nstack=%s", rService, rAddr, err.Error(), de.Stack())
		} else {
			klog.CtxErrorf(ctx, "KITEX: processing request error, remoteService=%s, remoteAddr=%v, error=%s", rService, rAddr, err.Error())
		}
	}
}

// SetInvokeHandleFunc implements the remote.InvokeHandleFuncSetter interface.
func (t *serverTransHandler) SetInvokeHandleFunc(streamHdlFunc remote.InnerServerEndpoint, inkHdlFunc endpoint.Endpoint) {
	t.inkHdlFunc = inkHdlFunc
}

// SetPipeline implements the remote.ServerTransHandler interface.
func (t *serverTransHandler) SetPipeline(p *remote.ServerTransPipeline) {
	t.transPipe = p
}

func (t *serverTransHandler) writeErrorReplyIfNeeded(
	ctx context.Context, conn net.Conn, st remote.ServerStream, err error, ri rpcinfo.RPCInfo,
) (shouldCloseConn bool) {
	if cn, ok := conn.(remote.IsActive); ok && !cn.IsActive() {
		// conn is closed, no need reply
		return
	}
	svcInfo := ri.Invocation().ServiceInfo()
	if svcInfo != nil {
		if methodInfo, _ := GetMethodInfo(ri, svcInfo); methodInfo != nil {
			if methodInfo.OneWay() {
				return
			}
		}
	}
	transErr, isTransErr := err.(*remote.TransError)
	if !isTransErr {
		return
	}

	errMsg := remote.NewMessage(transErr, svcInfo, ri, remote.Exception, remote.Server)
	errMsg.SetPayloadCodec(t.opt.PayloadCodec)
	ctx, err = st.Write(ctx, errMsg)
	if err != nil {
		klog.CtxErrorf(ctx, "KITEX: write error reply failed, remote=%s, error=%s", conn.RemoteAddr(), err.Error())
		return true
	}
	return
}

func (t *serverTransHandler) startTracer(ctx context.Context, ri rpcinfo.RPCInfo) context.Context {
	c := t.opt.TracerCtl.DoStart(ctx, ri)
	return c
}

func (t *serverTransHandler) finishTracer(ctx context.Context, ri rpcinfo.RPCInfo, err error, panicErr interface{}) {
	rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats())
	if rpcStats == nil {
		return
	}
	if panicErr != nil {
		rpcStats.SetPanicked(panicErr)
	}
	if err != nil && t.ext.IsRemoteClosedErr(err) {
		// it should not regard the error which caused by remote connection closed as server error
		err = nil
	}
	t.opt.TracerCtl.DoFinish(ctx, ri, err)
	// for server side, rpcinfo is reused on connection, clear the rpc stats info but keep the level config
	sl := ri.Stats().Level()
	rpcStats.Reset()
	rpcStats.SetLevel(sl)
}

func (t *serverTransHandler) startProfiler(ctx context.Context) context.Context {
	if t.opt.Profiler == nil {
		return ctx
	}
	return t.opt.Profiler.Prepare(ctx)
}

func (t *serverTransHandler) finishProfiler(ctx context.Context) {
	if t.opt.Profiler == nil {
		return
	}
	t.opt.Profiler.Untag(ctx)
}

type svrStream struct {
	conn net.Conn
	ctx  context.Context
	t    *serverTransHandler
	ri   rpcinfo.RPCInfo
	pipe *remote.ServerStreamPipeline
}

func newServerStream(ctx context.Context, conn net.Conn, t *serverTransHandler, ri rpcinfo.RPCInfo) *svrStream {
	return &svrStream{
		conn: conn,
		ctx:  ctx,
		t:    t,
		ri:   ri,
	}
}

func (s *svrStream) SetPipeline(pipe *remote.ServerStreamPipeline) {
	s.pipe = pipe
}

func (s *svrStream) Write(ctx context.Context, sendMsg remote.Message) (nctx context.Context, err error) {
	var bufWriter remote.ByteBuffer
	ri := sendMsg.RPCInfo()
	rpcinfo.Record(ctx, ri, stats.WriteStart, nil)
	defer func() {
		s.t.ext.ReleaseBuffer(bufWriter, err)
		rpcinfo.Record(ctx, ri, stats.WriteFinish, err)
	}()

	svcInfo := ri.Invocation().ServiceInfo()
	if svcInfo != nil {
		if methodInfo, _ := GetMethodInfo(ri, svcInfo); methodInfo != nil {
			if methodInfo.OneWay() {
				return ctx, nil
			}
		}
	}

	bufWriter = s.t.ext.NewWriteByteBuffer(ctx, s.conn, sendMsg)
	err = s.t.codec.Encode(ctx, sendMsg, bufWriter)
	if err != nil {
		return ctx, err
	}
	return ctx, bufWriter.Flush()
}

func (s *svrStream) Read(ctx context.Context, recvMsg remote.Message) (nctx context.Context, err error) {
	var bufReader remote.ByteBuffer
	defer func() {
		s.t.ext.ReleaseBuffer(bufReader, err)
		rpcinfo.Record(ctx, recvMsg.RPCInfo(), stats.ReadFinish, err)
	}()
	rpcinfo.Record(ctx, recvMsg.RPCInfo(), stats.ReadStart, nil)

	bufReader = s.t.ext.NewReadByteBuffer(ctx, s.conn, recvMsg)
	if codec, ok := s.t.codec.(remote.MetaDecoder); ok {
		if err = codec.DecodeMeta(ctx, recvMsg, bufReader); err == nil {
			if s.t.opt.Profiler != nil && s.t.opt.ProfilerTransInfoTagging != nil && recvMsg.TransInfo() != nil {
				var tags []string
				ctx, tags = s.t.opt.ProfilerTransInfoTagging(ctx, recvMsg)
				ctx = s.t.opt.Profiler.Tag(ctx, tags...)
			}
			err = codec.DecodePayload(ctx, recvMsg, bufReader)
		}
	} else {
		err = s.t.codec.Decode(ctx, recvMsg, bufReader)
	}
	if err != nil {
		recvMsg.Tags()[remote.ReadFailed] = true
		return ctx, err
	}
	if recvMsg.MessageType() == remote.Heartbeat {
		// for heartbeat protocols, not graceful, need to be adjusted
		return ctx, errHeartbeatMessage
	}
	return ctx, nil
}

// TODO: splitting meta and payload codec.
func (s *svrStream) RecvMsg(m interface{}) (err error) {
	var recvMsg remote.Message
	defer func() {
		remote.RecycleMessage(recvMsg)
	}()
	recvMsg = remote.NewMessage(nil, nil, s.ri, remote.Call, remote.Server)
	recvMsg.SetPayloadCodec(s.t.opt.PayloadCodec)
	_, err = s.pipe.Read(s.ctx, recvMsg)
	rm := m.(*serviceinfo.PingPongCompatibleMethodArgs)
	rm.Data = recvMsg.Data()
	return
}

func (s *svrStream) SendMsg(m interface{}) (err error) {
	var sendMsg remote.Message
	defer func() {
		remote.RecycleMessage(sendMsg)
	}()
	svcInfo := s.ri.Invocation().ServiceInfo()
	sendMsg = remote.NewMessage(m, svcInfo, s.ri, remote.Reply, remote.Server)
	sendMsg.SetPayloadCodec(s.t.opt.PayloadCodec)
	_, err = s.pipe.Write(s.ctx, sendMsg)
	return
}

func (s *svrStream) SetHeader(streaming.Header) error {
	panic("not implemented streaming method for default server handler")
}

func (s *svrStream) SendHeader(streaming.Header) error {
	panic("not implemented streaming method for default server handler")
}

func (s *svrStream) SetTrailer(streaming.Trailer) error {
	panic("not implemented streaming method for default server handler")
}

func getRemoteInfo(ri rpcinfo.RPCInfo, conn net.Conn) (string, net.Addr) {
	rAddr := conn.RemoteAddr()
	if ri == nil {
		return "", rAddr
	}
	if rAddr != nil && rAddr.Network() == "unix" {
		if ri.From().Address() != nil {
			rAddr = ri.From().Address()
		}
	}
	return ri.From().ServiceName(), rAddr
}

type PingPongRPCInfo struct {
	rpcinfo.RPCInfo
	targetSvcInfo *serviceinfo.ServiceInfo
	svcSearcher   remote.ServiceSearcher
}

func (p *PingPongRPCInfo) SpecifyServiceInfo(svcName, methodName string) (*serviceinfo.ServiceInfo, error) {
	// for single service scenario
	if p.targetSvcInfo != nil {
		if mt := p.targetSvcInfo.MethodInfo(methodName); mt == nil {
			return nil, remote.NewTransErrorWithMsg(remote.UnknownMethod, fmt.Sprintf("unknown method %s", methodName))
		}
		return p.targetSvcInfo, nil
	}
	svcInfo := p.svcSearcher.SearchService(svcName, methodName, false)
	if svcInfo == nil {
		return nil, remote.NewTransErrorWithMsg(remote.UnknownService, fmt.Sprintf("unknown service %s, method %s", svcName, methodName))
	}
	if _, ok := svcInfo.Methods[methodName]; !ok {
		return nil, remote.NewTransErrorWithMsg(remote.UnknownMethod, fmt.Sprintf("unknown method %s (service %s)", methodName, svcName))
	}
	p.targetSvcInfo = svcInfo
	return svcInfo, nil
}
