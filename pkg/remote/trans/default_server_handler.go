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
	"net"
	"runtime/debug"

	stats2 "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

// NewDefaultSvrTransHandler to provide default impl of svrTransHandler, it can be reused in netpoll, shm-ipc, framework-sdk extensions
func NewDefaultSvrTransHandler(opt *remote.ServerOption, ext Extension) (remote.ServerTransHandler, error) {
	svrHdlr := &svrTransHandler{
		opt:     opt,
		codec:   opt.Codec,
		svcInfo: opt.SvcInfo,
		ext:     ext,
	}
	if svrHdlr.opt.TracerCtl == nil {
		// init TraceCtl when it is nil, or it will lead some unit tests panic
		svrHdlr.opt.TracerCtl = &stats2.Controller{}
	}
	return svrHdlr, nil
}

type svrTransHandler struct {
	opt        *remote.ServerOption
	svcInfo    *serviceinfo.ServiceInfo
	inkHdlFunc endpoint.Endpoint
	codec      remote.Codec
	transPipe  *remote.TransPipeline
	ext        Extension
}

// Write implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) Write(ctx context.Context, conn net.Conn, sendMsg remote.Message) (err error) {
	var bufWriter remote.ByteBuffer
	ri := sendMsg.RPCInfo()
	stats2.Record(ctx, ri, stats.WriteStart, nil)
	defer func() {
		t.ext.ReleaseBuffer(bufWriter, err)
		stats2.Record(ctx, ri, stats.WriteFinish, err)
	}()

	if methodInfo, _ := GetMethodInfo(ri, t.svcInfo); methodInfo != nil {
		if methodInfo.OneWay() {
			return nil
		}
	}

	bufWriter = t.ext.NewWriteByteBuffer(ctx, conn, sendMsg)
	err = t.codec.Encode(ctx, sendMsg, bufWriter)
	if err != nil {
		return err
	}
	return bufWriter.Flush()
}

// Read implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) Read(ctx context.Context, conn net.Conn, recvMsg remote.Message) (err error) {
	var bufReader remote.ByteBuffer
	defer func() {
		t.ext.ReleaseBuffer(bufReader, err)
		stats2.Record(ctx, recvMsg.RPCInfo(), stats.ReadFinish, err)
	}()
	stats2.Record(ctx, recvMsg.RPCInfo(), stats.ReadStart, nil)

	bufReader = t.ext.NewReadByteBuffer(ctx, conn, recvMsg)
	err = t.codec.Decode(ctx, recvMsg, bufReader)
	if err != nil {
		recvMsg.Tags()[remote.ReadFailed] = true
		return err
	}
	return nil
}

// OnRead implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) OnRead(ctx context.Context, conn net.Conn) error {
	ri := rpcinfo.GetRPCInfo(ctx)
	t.ext.SetReadTimeout(ctx, conn, ri.Config(), remote.Server)
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
		t.finishTracer(ctx, ri, err, panicErr)
		remote.RecycleMessage(recvMsg)
		remote.RecycleMessage(sendMsg)
	}()
	ctx = t.startTracer(ctx, ri)
	recvMsg = remote.NewMessageWithNewer(t.svcInfo, ri, remote.Call, remote.Server)
	recvMsg.SetPayloadCodec(t.opt.PayloadCodec)
	err = t.Read(ctx, conn, recvMsg)
	if err != nil {
		closeConn = true
		t.writeErrorReplyIfNeeded(ctx, recvMsg, conn, err, ri, true)
		t.OnError(ctx, err, conn)
		return nil
	}

	var methodInfo serviceinfo.MethodInfo
	if methodInfo, err = GetMethodInfo(ri, t.svcInfo); err != nil {
		// it won't be err, because the method has been checked in decode, err check here just do defensive inspection
		closeConn = true
		t.writeErrorReplyIfNeeded(ctx, recvMsg, conn, err, ri, true)
		// for proxy case, need read actual remoteAddr, error print must exec after writeErrorReplyIfNeeded
		t.OnError(ctx, err, conn)
		return nil
	}
	if methodInfo.OneWay() {
		sendMsg = remote.NewMessage(nil, t.svcInfo, ri, remote.Reply, remote.Server)
	} else {
		sendMsg = remote.NewMessage(methodInfo.NewResult(), t.svcInfo, ri, remote.Reply, remote.Server)
	}

	ctx, err = t.transPipe.OnMessage(ctx, recvMsg, sendMsg)
	if err != nil {
		// error cannot be wrapped to print here, so it must exec before NewTransError
		t.OnError(ctx, err, conn)
		err = remote.NewTransError(remote.InternalError, err)
		t.writeErrorReplyIfNeeded(ctx, recvMsg, conn, err, ri, false)
		return nil
	}

	remote.FillSendMsgFromRecvMsg(recvMsg, sendMsg)
	if err = t.transPipe.Write(ctx, conn, sendMsg); err != nil {
		t.OnError(ctx, err, conn)
		return nil
	}
	return nil
}

// OnMessage implements the remote.ServerTransHandler interface.
// msg is the decoded instance, such as Arg and Result.
func (t *svrTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	err := t.inkHdlFunc(ctx, args.Data(), result.Data())
	return ctx, err
}

// OnActive implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	// init rpcinfo
	_, ctx = t.opt.InitRPCInfoFunc(ctx, conn.RemoteAddr())
	return ctx, nil
}

// OnInactive implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// recycle rpcinfo
	rpcinfo.PutRPCInfo(rpcinfo.GetRPCInfo(ctx))
}

// OnError implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	ri := rpcinfo.GetRPCInfo(ctx)
	rService, rAddr := getRemoteInfo(ri, conn)
	if t.ext.IsRemoteClosedErr(err) {
		// it should not regard error which cause by remote connection closed as server error
		if ri == nil {
			return
		}
		remote := rpcinfo.AsMutableEndpointInfo(ri.From())
		remote.SetTag(rpcinfo.RemoteClosedTag, "1")
	} else if pe, ok := err.(*kerrors.DetailedError); ok {
		klog.CtxErrorf(ctx, "KITEX: processing request error, remoteService=%s, remoteAddr=%v, error=%s\nstack=%s", rService, rAddr, err.Error(), pe.Stack())
	} else {
		klog.CtxErrorf(ctx, "KITEX: processing request error, remoteService=%s, remoteAddr=%v, error=%s", rService, rAddr, err.Error())
	}
}

// SetInvokeHandleFunc implements the remote.InvokeHandleFuncSetter interface.
func (t *svrTransHandler) SetInvokeHandleFunc(inkHdlFunc endpoint.Endpoint) {
	t.inkHdlFunc = inkHdlFunc
}

// SetPipeline implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) SetPipeline(p *remote.TransPipeline) {
	t.transPipe = p
}

func (t *svrTransHandler) writeErrorReplyIfNeeded(ctx context.Context, recvMsg remote.Message, conn net.Conn, err error, ri rpcinfo.RPCInfo, doOnMessage bool) {
	if cn, ok := conn.(remote.IsActive); ok && !cn.IsActive() {
		// conn is closed, no need reply
		return
	}
	if methodInfo, _ := GetMethodInfo(ri, t.svcInfo); methodInfo != nil {
		if methodInfo.OneWay() {
			return
		}
	}
	transErr, isTransErr := err.(*remote.TransError)
	if !isTransErr {
		return
	}
	errMsg := remote.NewMessage(transErr, t.svcInfo, ri, remote.Exception, remote.Server)
	remote.FillSendMsgFromRecvMsg(recvMsg, errMsg)
	if doOnMessage {
		// if error happen before normal OnMessage, exec it to transfer header trans info into rpcinfo
		t.transPipe.OnMessage(ctx, recvMsg, errMsg)
	}
	err = t.transPipe.Write(ctx, conn, errMsg)
	if err != nil {
		klog.CtxErrorf(ctx, "KITEX: write error reply failed, remote=%s, error=%s", conn.RemoteAddr(), err.Error())
	}
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
