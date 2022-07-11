/*
 * Copyright 2022 CloudWeGo Authors
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

package gonet

import (
	"context"
	"errors"
	"net"
	"runtime/debug"
	"syscall"

	stats2 "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

type svrTransHandlerFactory struct{}

// NewSvrTransHandlerFactory creates a default netpoll server transport handler factory.
func NewSvrTransHandlerFactory() remote.ServerTransHandlerFactory {
	return &svrTransHandlerFactory{}
}

// NewTransHandler implements the remote.ServerTransHandlerFactory interface.
func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	return newSvrTransHandler(opt)
}

func newSvrTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	svrHdlr := &svrTransHandler{
		opt:   opt,
		codec: opt.Codec,
	}
	if svrHdlr.opt.TracerCtl == nil {
		// init TraceCtl when it is nil, or it will lead some unit tests panic
		svrHdlr.opt.TracerCtl = &stats2.Controller{}
	}
	return svrHdlr, nil
}

var _ remote.ServerTransHandler = &svrTransHandler{}

type svrTransHandler struct {
	opt        *remote.ServerOption
	codec      remote.Codec
	transPipe  *remote.TransPipeline
	inkHdlFunc endpoint.Endpoint
}

func (h *svrTransHandler) Write(ctx context.Context, conn net.Conn, sendMsg remote.Message) (err error) {
	var bufWriter remote.ByteBuffer
	ri := sendMsg.RPCInfo()
	stats2.Record(ctx, ri, stats.WriteStart, nil)
	defer func() {
		_ = bufWriter.Release(err)
		stats2.Record(ctx, ri, stats.WriteFinish, err)
	}()

	if methodInfo, _ := trans.GetMethodInfo(ri, h.opt.SvcInfo); methodInfo != nil {
		if methodInfo.OneWay() {
			return nil
		}
	}

	bufWriter = NewBufferWriter(conn)
	err = h.codec.Encode(ctx, sendMsg, bufWriter)
	if err != nil {
		return err
	}
	return bufWriter.Flush()
}

func (h *svrTransHandler) Read(ctx context.Context, conn net.Conn, recvMsg remote.Message) (err error) {
	var bufReader remote.ByteBuffer
	defer func() {
		_ = bufReader.Release(err)
		stats2.Record(ctx, recvMsg.RPCInfo(), stats.ReadFinish, err)
	}()
	stats2.Record(ctx, recvMsg.RPCInfo(), stats.ReadStart, nil)

	bufReader = NewBufferReader(conn)
	err = h.codec.Decode(ctx, recvMsg, bufReader)
	if err != nil {
		recvMsg.Tags()[remote.ReadFailed] = true
		return err
	}
	return nil
}

func (h *svrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// recycle rpcinfo
	rpcinfo.PutRPCInfo(rpcinfo.GetRPCInfo(ctx))
}

func (h *svrTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	ri := rpcinfo.GetRPCInfo(ctx)
	rService, rAddr := getRemoteInfo(ri, conn)
	if isRemoteClosedErr(err) {
		// it should not regard error which cause by remote connection closed as server error
		if ri == nil {
			return
		}
		ep := rpcinfo.AsMutableEndpointInfo(ri.From())
		_ = ep.SetTag(rpcinfo.RemoteClosedTag, "1")
	} else if pe, ok := err.(*kerrors.DetailedError); ok {
		klog.CtxErrorf(ctx, "KITEX: processing request error, remoteService=%s, remoteAddr=%v, error=%s\nstack=%s", rService, rAddr, err.Error(), pe.Stack())
	} else {
		klog.CtxErrorf(ctx, "KITEX: processing request error, remoteService=%s, remoteAddr=%v, error=%s", rService, rAddr, err.Error())
	}
}

func (h *svrTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	err := h.inkHdlFunc(ctx, args.Data(), result.Data())
	return ctx, err
}

// SetInvokeHandleFunc implements the remote.InvokeHandleFuncSetter interface.
func (t *svrTransHandler) SetInvokeHandleFunc(inkHdlFunc endpoint.Endpoint) {
	t.inkHdlFunc = inkHdlFunc
}

func (h *svrTransHandler) SetPipeline(pipeline *remote.TransPipeline) {
	h.transPipe = pipeline
}

func (h *svrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	// init rpcinfo
	_, ctx = h.opt.InitRPCInfoFunc(ctx, conn.RemoteAddr())
	return ctx, nil
}

func (h *svrTransHandler) OnRead(ctx context.Context, conn net.Conn) error {
	ri := rpcinfo.GetRPCInfo(ctx)
	setReadTimeout(conn, ri.Config(), remote.Server)
	var err error
	var closeConn bool
	var recvMsg remote.Message
	var sendMsg remote.Message
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			closeConn = true
			if conn != nil {
				ri := rpcinfo.GetRPCInfo(ctx)
				rService, rAddr := getRemoteInfo(ri, conn)
				klog.CtxErrorf(ctx, "KITEX: panic happened, close conn, remoteAddress=%s, remoteService=%s, error=%v\nstack=%s", rAddr, rService, panicErr, string(debug.Stack()))
			} else {
				klog.CtxErrorf(ctx, "KITEX: panic happened, error=%v\nstack=%s", panicErr, string(debug.Stack()))
			}
		}
		if closeConn && conn != nil {
			conn.Close()
		}
		h.finishTracer(ctx, ri, err, panicErr)
		remote.RecycleMessage(recvMsg)
		remote.RecycleMessage(sendMsg)
	}()
	ctx = h.startTracer(ctx, ri)
	recvMsg = remote.NewMessageWithNewer(h.opt.SvcInfo, ri, remote.Call, remote.Server)
	recvMsg.SetPayloadCodec(h.opt.PayloadCodec)
	err = h.Read(ctx, conn, recvMsg)
	if err != nil {
		closeConn = true
		h.writeErrorReplyIfNeeded(ctx, recvMsg, conn, err, ri, true)
		h.OnError(ctx, err, conn)
		return nil
	}

	var methodInfo serviceinfo.MethodInfo
	if methodInfo, err = trans.GetMethodInfo(ri, h.opt.SvcInfo); err != nil {
		// it won't be err, because the method has been checked in decode, err check here just do defensive inspection
		closeConn = true
		h.writeErrorReplyIfNeeded(ctx, recvMsg, conn, err, ri, true)
		// for proxy case, need read actual remoteAddr, error print must exec after writeErrorReplyIfNeeded
		h.OnError(ctx, err, conn)
		return nil
	}
	if methodInfo.OneWay() {
		sendMsg = remote.NewMessage(nil, h.opt.SvcInfo, ri, remote.Reply, remote.Server)
	} else {
		sendMsg = remote.NewMessage(methodInfo.NewResult(), h.opt.SvcInfo, ri, remote.Reply, remote.Server)
	}

	ctx, err = h.transPipe.OnMessage(ctx, recvMsg, sendMsg)
	if err != nil {
		// error cannot be wrapped to print here, so it must exec before NewTransError
		h.OnError(ctx, err, conn)
		err = remote.NewTransError(remote.InternalError, err)
		closeConn = h.writeErrorReplyIfNeeded(ctx, recvMsg, conn, err, ri, false)
		return nil
	}

	remote.FillSendMsgFromRecvMsg(recvMsg, sendMsg)
	if err = h.transPipe.Write(ctx, conn, sendMsg); err != nil {
		closeConn = true
		h.OnError(ctx, err, conn)
		return nil
	}
	return nil
}

func (h *svrTransHandler) startTracer(ctx context.Context, ri rpcinfo.RPCInfo) context.Context {
	c := h.opt.TracerCtl.DoStart(ctx, ri)
	return c
}

func (h *svrTransHandler) finishTracer(ctx context.Context, ri rpcinfo.RPCInfo, err error, panicErr interface{}) {
	rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats())
	if rpcStats == nil {
		return
	}
	if panicErr != nil {
		rpcStats.SetPanicked(panicErr)
	}
	if err != nil && isRemoteClosedErr(err) {
		// it should not regard the error which caused by remote connection closed as server error
		err = nil
	}
	h.opt.TracerCtl.DoFinish(ctx, ri, err)
	// for server side, rpcinfo is reused on connection, clear the rpc stats info but keep the level config
	sl := ri.Stats().Level()
	rpcStats.Reset()
	rpcStats.SetLevel(sl)
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

func isRemoteClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) || errors.Is(err, syscall.EPIPE)
}

func (h *svrTransHandler) writeErrorReplyIfNeeded(
	ctx context.Context, recvMsg remote.Message, conn net.Conn, err error, ri rpcinfo.RPCInfo, doOnMessage bool,
) (shouldCloseConn bool) {
	if cn, ok := conn.(remote.IsActive); ok && !cn.IsActive() {
		// conn is closed, no need reply
		return
	}
	if methodInfo, _ := trans.GetMethodInfo(ri, h.opt.SvcInfo); methodInfo != nil {
		if methodInfo.OneWay() {
			return
		}
	}
	transErr, isTransErr := err.(*remote.TransError)
	if !isTransErr {
		return
	}
	errMsg := remote.NewMessage(transErr, h.opt.SvcInfo, ri, remote.Exception, remote.Server)
	remote.FillSendMsgFromRecvMsg(recvMsg, errMsg)
	if doOnMessage {
		// if error happen before normal OnMessage, exec it to transfer header trans info into rpcinfo
		h.transPipe.OnMessage(ctx, recvMsg, errMsg)
	}
	err = h.transPipe.Write(ctx, conn, errMsg)
	if err != nil {
		klog.CtxErrorf(ctx, "KITEX: write error reply failed, remote=%s, error=%s", conn.RemoteAddr(), err.Error())
		return true
	}
	return
}
