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
	"net"
	"time"

	stats2 "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

type cliTransHandlerFactory struct{}

// NewCliTransHandlerFactory returns a new remote.ClientTransHandlerFactory for netpoll.
func NewCliTransHandlerFactory() remote.ClientTransHandlerFactory {
	return &cliTransHandlerFactory{}
}

// NewTransHandler implements the remote.ClientTransHandlerFactory interface.
func (f *cliTransHandlerFactory) NewTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	return newCliTransHandler(opt)
}

func newCliTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	return &cliTransHandler{
		opt: opt,
	}, nil
}

var _ remote.ClientTransHandler = &cliTransHandler{}

type cliTransHandler struct {
	opt       *remote.ClientOption
	codec     remote.Codec
	transPipe *remote.TransPipeline
}

func (h cliTransHandler) Write(ctx context.Context, conn net.Conn, sendMsg remote.Message) (err error) {
	var bufWriter remote.ByteBuffer
	stats2.Record(ctx, sendMsg.RPCInfo(), stats.WriteStart, nil)
	defer func() {
		_ = bufWriter.Release(err)
		stats2.Record(ctx, sendMsg.RPCInfo(), stats.WriteFinish, err)
	}()

	bufWriter = NewBufferWriter(conn)
	sendMsg.SetPayloadCodec(h.opt.PayloadCodec)
	err = h.codec.Encode(ctx, sendMsg, bufWriter)
	if err != nil {
		return err
	}
	return bufWriter.Flush()
}

func (h cliTransHandler) Read(ctx context.Context, conn net.Conn, recvMsg remote.Message) (err error) {
	var bufReader remote.ByteBuffer
	stats2.Record(ctx, recvMsg.RPCInfo(), stats.ReadStart, nil)
	defer func() {
		_ = bufReader.Release(err)
		stats2.Record(ctx, recvMsg.RPCInfo(), stats.ReadFinish, err)
	}()

	setReadTimeout(conn, recvMsg.RPCInfo().Config(), recvMsg.RPCRole())
	bufReader = NewBufferReader(conn)
	recvMsg.SetPayloadCodec(h.opt.PayloadCodec)
	err = h.codec.Decode(ctx, recvMsg, bufReader)
	if err != nil {
		if h.IsTimeoutErr(err) {
			err = kerrors.ErrRPCTimeout.WithCause(err)
		}
		return err
	}

	return nil
}

func setReadTimeout(conn net.Conn, cfg rpcinfo.RPCConfig, role remote.RPCRole) {
	var readTimeout time.Duration
	if role == remote.Client {
		readTimeout = trans.GetReadTimeout(cfg)
	} else {
		readTimeout = cfg.ReadWriteTimeout()
	}
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
}

func (h cliTransHandler) IsTimeoutErr(err error) bool {
	if err != nil {
		if nerr, ok := err.(net.Error); ok {
			return nerr.Timeout()
		}
	}
	return false
}

func (c cliTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// ineffective now and do nothing
	return
}

func (c cliTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	if pe, ok := err.(*kerrors.DetailedError); ok {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, error=%s\nstack=%s", conn.RemoteAddr(), err.Error(), pe.Stack())
	} else {
		klog.CtxErrorf(ctx, "KITEX: send request error, remote=%s, error=%s", conn.RemoteAddr(), err.Error())
	}
}

func (c cliTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	// ineffective now and do nothing
	return ctx, nil
}

func (h cliTransHandler) SetPipeline(pipeline *remote.TransPipeline) {
	h.transPipe = pipeline
}
