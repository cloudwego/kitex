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

package netpollmux

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"sync"

	"github.com/cloudwego/netpoll"

	stats2 "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans"
	np "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

type svrTransHandlerFactory struct{}

// NewSvrTransHandlerFactory creates a default netpollmux remote.ServerTransHandlerFactory.
func NewSvrTransHandlerFactory() remote.ServerTransHandlerFactory {
	return &svrTransHandlerFactory{}
}

// NewTransHandler implements the remote.ServerTransHandlerFactory interface.
// TODO: use object pool?
func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	return newSvrTransHandler(opt)
}

func newSvrTransHandler(opt *remote.ServerOption) (*svrTransHandler, error) {
	return &svrTransHandler{
		opt:     opt,
		codec:   opt.Codec,
		svcInfo: opt.SvcInfo,
		ext:     np.NewNetpollConnExtension(),
	}, nil
}

var _ remote.ServerTransHandler = &svrTransHandler{}

type svrTransHandler struct {
	opt        *remote.ServerOption
	svcInfo    *serviceinfo.ServiceInfo
	inkHdlFunc endpoint.Endpoint
	codec      remote.Codec
	transPipe  *remote.TransPipeline
	ext        trans.Extension
}

// Write implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) Write(ctx context.Context, conn net.Conn, sendMsg remote.Message) (err error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	stats2.Record(ctx, ri, stats.WriteStart, nil)
	defer func() {
		stats2.Record(ctx, ri, stats.WriteFinish, nil)
	}()

	if methodInfo, _ := trans.GetMethodInfo(ri, t.svcInfo); methodInfo != nil {
		if methodInfo.OneWay() {
			return nil
		}
	}

	bufWriter := np.NewWriterByteBuffer(netpoll.NewLinkBuffer())
	err = t.codec.Encode(ctx, sendMsg, bufWriter)
	if err != nil {
		return err
	}
	conn.(*muxSvrConn).Put(func() (buf remote.ByteBuffer, isNil bool) {
		return bufWriter, false
	})
	return nil
}

// Read implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) (err error) {
	return nil
}

func (t *svrTransHandler) readWithByteBuffer(ctx context.Context, bufReader remote.ByteBuffer, msg remote.Message) (err error) {
	defer func() {
		if bufReader != nil {
			if err != nil {
				bufReader.Skip(bufReader.ReadableLen())
			}
			bufReader.Release(err)
		}
		stats2.Record(ctx, msg.RPCInfo(), stats.ReadFinish, err)
	}()
	stats2.Record(ctx, msg.RPCInfo(), stats.ReadStart, nil)

	err = t.codec.Decode(ctx, msg, bufReader)
	if err != nil {
		msg.Tags()[remote.ReadFailed] = true
		return err
	}
	return nil
}

// OnRead implements the remote.ServerTransHandler interface.
// Returns write err only.
func (t *svrTransHandler) OnRead(muxSvrConnCtx context.Context, conn net.Conn) error {
	defer t.tryRecover(muxSvrConnCtx, conn)

	connection := conn.(netpoll.Connection)
	// protocol header check
	length, _, err := parseHeader(connection.Reader())
	if err != nil {
		err = fmt.Errorf("%w: addr(%s)", err, connection.RemoteAddr())
		klog.Errorf("KITEX: %s", err.Error())
		connection.Close()
		return err
	}
	reader, err := connection.Reader().Slice(length)
	if err != nil {
		err = fmt.Errorf("%w: addr(%s)", err, connection.RemoteAddr())
		klog.Errorf("KITEX: %s", err.Error())
		connection.Close()
		return nil
	}

	gofunc.GoFunc(muxSvrConnCtx, func() {
		// rpcInfoCtx is a pooled ctx with inited RPCInfo which can be reused.
		// it's recycled in defer.
		muxSvrConn, _ := muxSvrConnCtx.Value(ctxKeyMuxSvrConn{}).(*muxSvrConn)
		rpcInfoCtx := muxSvrConn.pool.Get().(context.Context)

		rpcInfo := rpcinfo.GetRPCInfo(rpcInfoCtx)
		// This is the request-level, one-shot ctx.
		// It adds the tracer principally, thus do not recycle.
		ctx := t.startTracer(rpcInfoCtx, rpcInfo)
		var err error
		var recvMsg remote.Message
		var sendMsg remote.Message
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				if conn != nil {
					ri := rpcinfo.GetRPCInfo(ctx)
					rService, rAddr := getRemoteInfo(ri, conn)
					klog.Errorf("KITEX: panic happened, close conn[%s], remoteService=%s, %v\n%s", rAddr, rService, panicErr, string(debug.Stack()))
					conn.Close()
				} else {
					klog.Errorf("KITEX: panic happened, %v\n%s", panicErr, string(debug.Stack()))
				}
			}
			t.finishTracer(ctx, rpcInfo, err, panicErr)
			remote.RecycleMessage(recvMsg)
			remote.RecycleMessage(sendMsg)
			muxSvrConn.pool.Put(rpcInfoCtx)
		}()

		// read
		recvMsg = remote.NewMessageWithNewer(t.svcInfo, rpcInfo, remote.Call, remote.Server)
		bufReader := np.NewReaderByteBuffer(reader)
		err = t.readWithByteBuffer(ctx, bufReader, recvMsg)
		if err != nil {
			t.writeErrorReplyIfNeeded(ctx, recvMsg, muxSvrConn, rpcInfo, err, false, true)
			// for proxy case, need read actual remoteAddr, error print must exec after writeErrorReplyIfNeeded
			t.OnError(ctx, err, muxSvrConn)
			return
		}

		var methodInfo serviceinfo.MethodInfo
		if methodInfo, err = trans.GetMethodInfo(rpcInfo, t.svcInfo); err != nil {
			t.writeErrorReplyIfNeeded(ctx, recvMsg, muxSvrConn, rpcInfo, err, false, true)
			t.OnError(ctx, err, muxSvrConn)
			return
		}
		if methodInfo.OneWay() {
			sendMsg = remote.NewMessage(nil, t.svcInfo, rpcInfo, remote.Reply, remote.Server)
		} else {
			sendMsg = remote.NewMessage(methodInfo.NewResult(), t.svcInfo, rpcInfo, remote.Reply, remote.Server)
		}

		ctx, err = t.transPipe.OnMessage(ctx, recvMsg, sendMsg)
		if err != nil {
			// error cannot be wrapped to print here, so it must exec before NewTransError
			t.OnError(ctx, err, muxSvrConn)
			err = remote.NewTransError(remote.InternalError, err)
			t.writeErrorReplyIfNeeded(ctx, recvMsg, muxSvrConn, rpcInfo, err, false, false)
			return
		}

		remote.FillSendMsgFromRecvMsg(recvMsg, sendMsg)
		if err = t.transPipe.Write(ctx, muxSvrConn, sendMsg); err != nil {
			t.OnError(ctx, err, muxSvrConn)
			return
		}
	})
	return nil
}

// OnMessage implements the remote.ServerTransHandler interface.
// msg is the decoded instance, such as Arg or Result.
// OnMessage notifies the higher level to process. It's used in async and server-side logic.
func (t *svrTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	err := t.inkHdlFunc(ctx, args.Data(), result.Data())
	return ctx, err
}

type ctxKeyMuxSvrConn struct{}

// OnActive implements the remote.ServerTransHandler interface.
// sync.Pool for RPCInfo is setup here.
func (t *svrTransHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	connection := conn.(netpoll.Connection)

	// 1. set readwrite timeout
	connection.SetReadTimeout(t.opt.ReadWriteTimeout)

	// 2. set mux server conn
	pool := &sync.Pool{
		New: func() interface{} {
			// init rpcinfo
			_, ctx := t.opt.InitRPCInfoFunc(ctx, connection.RemoteAddr())
			return ctx
		},
	}
	return context.WithValue(context.Background(), ctxKeyMuxSvrConn{}, newMuxSvrConn(connection, pool)), nil
}

// OnInactive implements the remote.ServerTransHandler interface.
func (t *svrTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// do nothing now
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
		klog.Errorf("KITEX: processing request error, remoteService=%s, remoteAddr=%v, err=%s\n%s", rService, rAddr, err.Error(), pe.Stack())
	} else {
		klog.Errorf("KITEX: processing request error, remoteService=%s, remoteAddr=%v, err=%s", rService, rAddr, err.Error())
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

func (t *svrTransHandler) writeErrorReplyIfNeeded(ctx context.Context, recvMsg remote.Message, conn net.Conn, ri rpcinfo.RPCInfo, err error, closeConn, doOnMessage bool) {
	defer func() {
		if closeConn {
			conn.Close()
		}
	}()
	if methodInfo, _ := trans.GetMethodInfo(ri, t.svcInfo); methodInfo != nil {
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
		klog.Errorf("KITEX: write error reply failed, remote=%s, err=%s", conn.RemoteAddr(), err.Error())
	}
}

func (t *svrTransHandler) tryRecover(ctx context.Context, conn net.Conn) {
	if err := recover(); err != nil {
		// rpcStat := internal.AsMutableRPCStats(t.rpcinfo.Stats())
		// rpcStat.SetPanicked(err)
		// t.opt.TracerCtl.DoFinish(ctx, klog)
		// 这里不需要 Reset rpcStats 因为连接会关闭，会直接把 RPCInfo 进行 Recycle

		if conn != nil {
			conn.Close()
			klog.Errorf("KITEX: panic happened, close conn[%s], %s\n%s", conn.RemoteAddr(), err, string(debug.Stack()))
		} else {
			klog.Errorf("KITEX: panic happened, %s\n%s", err, string(debug.Stack()))
		}
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
	if errors.Is(err, netpoll.ErrConnClosed) {
		// it should not regard error which cause by remote connection closed as server error
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
	if rAddr.Network() == "unix" {
		if ri.From().Address() != nil {
			rAddr = ri.From().Address()
		}
	}
	return ri.From().ServiceName(), rAddr
}
