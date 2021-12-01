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

	stats2 "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/stats"
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
	transPipe *remote.TransPipeline
	ext       Extension
}

// Write implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) Write(ctx context.Context, conn net.Conn, sendMsg remote.Message) (err error) {
	var bufWriter remote.ByteBuffer
	stats2.Record(ctx, sendMsg.RPCInfo(), stats.WriteStart, nil)
	defer func() {
		t.ext.ReleaseBuffer(bufWriter, err)
		stats2.Record(ctx, sendMsg.RPCInfo(), stats.WriteFinish, err)
	}()

	bufWriter = t.ext.NewWriteByteBuffer(ctx, conn, sendMsg)
	sendMsg.SetPayloadCodec(t.opt.PayloadCodec)
	err = t.codec.Encode(ctx, sendMsg, bufWriter)
	if err != nil {
		return err
	}
	return bufWriter.Flush()
}

// Read implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) Read(ctx context.Context, conn net.Conn, recvMsg remote.Message) (err error) {
	var bufReader remote.ByteBuffer
	stats2.Record(ctx, recvMsg.RPCInfo(), stats.ReadStart, nil)
	defer func() {
		t.ext.ReleaseBuffer(bufReader, err)
		stats2.Record(ctx, recvMsg.RPCInfo(), stats.ReadFinish, err)
	}()

	t.ext.SetReadTimeout(ctx, conn, recvMsg.RPCInfo().Config(), recvMsg.RPCRole())
	bufReader = t.ext.NewReadByteBuffer(ctx, conn, recvMsg)
	recvMsg.SetPayloadCodec(t.opt.PayloadCodec)
	err = t.codec.Decode(ctx, recvMsg, bufReader)
	if err != nil {
		if t.ext.IsTimeoutErr(err) {
			return kerrors.ErrRPCTimeout.WithCause(err)
		}
		return err
	}

	return nil
}

// OnMessage implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	// do nothing
	return ctx, nil
}

// OnInactive implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	// ineffective now and do nothing
}

// OnError implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	if pe, ok := err.(*kerrors.DetailedError); ok {
		klog.Errorf("KITEX: send request error, remote=%s, err=%s\n%s", conn.RemoteAddr(), err.Error(), pe.Stack())
	} else {
		klog.Errorf("KITEX: send request error, remote=%s, err=%s", conn.RemoteAddr(), err.Error())
	}
}

// SetPipeline implements the remote.ClientTransHandler interface.
func (t *cliTransHandler) SetPipeline(p *remote.TransPipeline) {
	t.transPipe = p
}
