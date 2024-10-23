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
	"fmt"
	"net"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// NewDefaultCliTransHandler to provide default impl of cliTransHandler, it can be reused in netpoll, shm-ipc, framework-sdk extensions
func NewDefaultCliTransHandler(opt *remote.ClientOption, ext Extension) (remote.ClientTransHandler, error) {
	return &cliTransHandler{
		opt:   opt,
		codec: opt.Codec,
		ext:   ext,
	}, nil
}

type cliTransHandler struct {
	opt       *remote.ClientOption
	codec     remote.Codec
	transPipe *remote.ClientTransPipeline
	ext       Extension
}

type cliStream struct {
	conn net.Conn
	t    *cliTransHandler
	ri   rpcinfo.RPCInfo
	pipe *remote.ClientStreamPipeline
}

func newCliStream(conn net.Conn, t *cliTransHandler, ri rpcinfo.RPCInfo) *cliStream {
	return &cliStream{
		conn: conn,
		t:    t,
		ri:   ri,
	}
}

func (s *cliStream) SetPipeline(pipe *remote.ClientStreamPipeline) {
	s.pipe = pipe
}

func (s *cliStream) Write(ctx context.Context, sendMsg remote.Message) (nctx context.Context, err error) {
	var bufWriter remote.ByteBuffer
	rpcinfo.Record(ctx, sendMsg.RPCInfo(), stats.WriteStart, nil)
	defer func() {
		s.t.ext.ReleaseBuffer(bufWriter, err)
		rpcinfo.Record(ctx, sendMsg.RPCInfo(), stats.WriteFinish, err)
	}()

	bufWriter = s.t.ext.NewWriteByteBuffer(ctx, s.conn, sendMsg)
	sendMsg.SetPayloadCodec(s.t.opt.PayloadCodec)
	err = s.t.codec.Encode(ctx, sendMsg, bufWriter)
	if err != nil {
		return ctx, err
	}
	return ctx, bufWriter.Flush()
}

func (s *cliStream) Read(ctx context.Context, recvMsg remote.Message) (nctx context.Context, err error) {
	var bufReader remote.ByteBuffer
	rpcinfo.Record(ctx, recvMsg.RPCInfo(), stats.ReadStart, nil)
	defer func() {
		s.t.ext.ReleaseBuffer(bufReader, err)
		rpcinfo.Record(ctx, recvMsg.RPCInfo(), stats.ReadFinish, err)
	}()

	s.t.ext.SetReadTimeout(ctx, s.conn, recvMsg.RPCInfo().Config(), recvMsg.RPCRole())
	bufReader = s.t.ext.NewReadByteBuffer(ctx, s.conn, recvMsg)
	recvMsg.SetPayloadCodec(s.t.opt.PayloadCodec)
	err = s.t.codec.Decode(ctx, recvMsg, bufReader)
	if err != nil {
		if s.t.ext.IsTimeoutErr(err) {
			err = kerrors.ErrRPCTimeout.WithCause(err)
		}
		return ctx, err
	}

	return ctx, nil
}

func (s *cliStream) Header() (streaming.Header, error) {
	panic("not implemented streaming method for default client handler")
}

func (s *cliStream) Trailer() (streaming.Trailer, error) {
	panic("not implemented streaming method for default client handler")
}

func (s *cliStream) CloseSend() error {
	return nil
}

func (s *cliStream) RecvMsg(ctx context.Context, resp interface{}) (err error) {
	ri := s.ri
	recvMsg := remote.NewMessage(resp, ri.Invocation().ServiceInfo(), ri, remote.Reply, remote.Client)
	defer func() {
		remote.RecycleMessage(recvMsg)
	}()
	ctx, err = s.pipe.Read(ctx, recvMsg)
	return err
}

func (s *cliStream) SendMsg(ctx context.Context, req interface{}) (err error) {
	ri := s.ri
	methodName := ri.Invocation().MethodName()
	svcInfo := ri.Invocation().ServiceInfo()
	m := ri.Invocation().MethodInfo()
	var sendMsg remote.Message
	if m == nil {
		return fmt.Errorf("method info is nil, methodName=%s, serviceInfo=%+v", methodName, svcInfo)
	} else if m.OneWay() {
		sendMsg = remote.NewMessage(req, svcInfo, ri, remote.Oneway, remote.Client)
	} else {
		sendMsg = remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
	}
	defer func() {
		remote.RecycleMessage(sendMsg)
	}()
	ctx, err = s.pipe.Write(ctx, sendMsg)
	return err
}

func (t *cliTransHandler) NewStream(ctx context.Context, conn net.Conn) (streaming.ClientStream, error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	cs := newCliStream(conn, t, ri)
	cs.SetPipeline(remote.NewClientStreamPipeline(cs, t.transPipe))
	return cs, nil
}

func (t *cliTransHandler) SetPipeline(pipe *remote.ClientTransPipeline) {
	t.transPipe = pipe
}

// OnInactive implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// ineffective now and do nothing
}

// OnError implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	if pe, ok := err.(*kerrors.DetailedError); ok {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, error=%s\nstack=%s", conn.RemoteAddr(), err.Error(), pe.Stack())
	} else {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, error=%s", conn.RemoteAddr(), err.Error())
	}
}
