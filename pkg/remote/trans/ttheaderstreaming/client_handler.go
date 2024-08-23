/*
 * Copyright 2024 CloudWeGo Authors
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

package ttheaderstreaming

import (
	"context"
	"fmt"
	"net"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/ttheader"
	"github.com/cloudwego/kitex/pkg/remote/trans"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/transport"
)

var _ remote.ClientStreamAllocator = (*ttheaderStreamingClientTransHandler)(nil)

func NewCliTransHandler(opt *remote.ClientOption, ext trans.Extension) (remote.ClientTransHandler, error) {
	return &ttheaderStreamingClientTransHandler{
		opt:              opt,
		codec:            ttheader.NewStreamCodec(),
		ext:              ext,
		frameMetaHandler: getClientFrameMetaHandler(opt.TTHeaderFrameMetaHandler),
	}, nil
}

func getClientFrameMetaHandler(handler remote.FrameMetaHandler) remote.FrameMetaHandler {
	if handler != nil {
		return handler
	}
	return NewClientTTHeaderFrameHandler()
}

type ttheaderStreamingClientTransHandler struct {
	opt              *remote.ClientOption
	codec            remote.Codec
	ext              trans.Extension
	frameMetaHandler remote.FrameMetaHandler
}

func (t *ttheaderStreamingClientTransHandler) newHeaderMessage(
	ctx context.Context, svcInfo *serviceinfo.ServiceInfo,
) (nCtx context.Context, message remote.Message, err error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	message = remote.NewMessage(nil, svcInfo, ri, remote.Stream, remote.Client)
	message.SetProtocolInfo(remote.NewProtocolInfo(transport.TTHeader, svcInfo.PayloadCodec))
	message.Tags()[codec.HeaderFlagsKey] = codec.HeaderFlagsStreaming
	message.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader

	if err = t.frameMetaHandler.WriteHeader(ctx, message); err != nil {
		return ctx, nil, fmt.Errorf("ClientFrameProcessor.WriteHeader failed: %w", err)
	}

	for _, h := range t.opt.StreamingMetaHandlers {
		if ctx, err = h.OnConnectStream(ctx); err != nil {
			return ctx, nil, fmt.Errorf("%T.OnConnectStream failed: %w", h, err)
		}
	}
	return ctx, message, nil
}

func (t *ttheaderStreamingClientTransHandler) NewStream(
	ctx context.Context, svcInfo *serviceinfo.ServiceInfo, conn net.Conn, handler remote.TransReadWriter,
) (streaming.Stream, error) {
	ctx, headerMessage, err := t.newHeaderMessage(ctx, svcInfo)
	if err != nil {
		return nil, err
	}

	rawStream := newTTHeaderStream(
		ctx, conn, headerMessage.RPCInfo(), t.codec, remote.Client, t.ext, t.frameMetaHandler)
	if t.opt.TTHeaderStreamingGRPCCompatible {
		rawStream.loadOutgoingMetadataForGRPC()
	}
	defer func() {
		if err != nil {
			_ = rawStream.closeSend(err) // make sure the sendLoop is released
		}
	}()

	if err = rawStream.sendHeaderFrame(headerMessage); err != nil {
		return nil, err
	}

	if t.opt.TTHeaderStreamingWaitMetaFrame {
		// Not enabled by default for performance reason, and should only be enabled when a MetaFrame will
		// definitely be returned, otherwise it will cause an error or even a hang
		// For example, when service discovery is taken over by a proxy, the proxy should return a MetaFrame,
		// so that Kitex client can get the real address and set it into RPCInfo; otherwise the tracer may not
		// be able to get the right address (e.g. for Client Streaming send events)
		if err = rawStream.clientReadMetaFrame(); err != nil {
			return nil, err
		}
	}
	return rawStream, nil
}

// Write is not used and should never be called
func (t *ttheaderStreamingClientTransHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) (nctx context.Context, err error) {
	panic("not used")
}

// Read is not used and should never be called
func (t *ttheaderStreamingClientTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) (nctx context.Context, err error) {
	panic("not used")
}

// OnInactive is not used and should never be called
func (t *ttheaderStreamingClientTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	panic("not used")
}

// OnError is not used and should never be called
func (t *ttheaderStreamingClientTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	panic("not used")
}

// OnMessage is not used and should never be called
func (t *ttheaderStreamingClientTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	panic("not used")
}

// SetPipeline is not used and should never be called
func (t *ttheaderStreamingClientTransHandler) SetPipeline(pipeline *remote.TransPipeline) {
	panic("not used")
}
