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

package bound

import (
	"context"
	"net"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// NewServerMetaHandler to build transMetaHandler that handle transport info
func NewServerMetaHandler(mhs []remote.MetaHandler) remote.ServerPipelineHandler {
	return &svrMetaHandler{mhs: mhs}
}

var _ remote.ServerPipelineHandler = (*svrMetaHandler)(nil)

type svrMetaHandler struct {
	mhs []remote.MetaHandler
}

// OnStreamWrite exec before encode
func (h *svrMetaHandler) OnStreamWrite(ctx context.Context, st remote.ServerStream, sendMsg remote.Message) (context.Context, error) {
	var err error
	for _, hdlr := range h.mhs {
		ctx, err = hdlr.WriteMeta(ctx, sendMsg)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func (h *svrMetaHandler) OnStreamRead(ctx context.Context, st remote.ServerStream, msg remote.Message) (nctx context.Context, err error) {
	for _, hdlr := range h.mhs {
		ctx, err = hdlr.ReadMeta(ctx, msg)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

// OnActive implements the remote.InboundHandler interface.
func (h *svrMetaHandler) OnActive(ctx context.Context, conn net.Conn) (context.Context, error) {
	return ctx, nil
}

// OnRead implements the remote.InboundHandler interface.
func (h *svrMetaHandler) OnRead(ctx context.Context, conn net.Conn) (context.Context, error) {
	return ctx, nil
}

func (h *svrMetaHandler) OnStream(ctx context.Context, st streaming.ServerStream) (context.Context, error) {
	var err error
	for _, hdlr := range h.mhs {
		ctx, err = hdlr.OnReadStream(ctx)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

// OnInactive implements the remote.InboundHandler interface.
func (h *svrMetaHandler) OnInactive(ctx context.Context, conn net.Conn) context.Context {
	return ctx
}

// NewClientMetaHandler to build transMetaHandler that handle transport info
func NewClientMetaHandler(mhs []remote.MetaHandler) remote.ClientPipelineHandler {
	return &cliMetaHandler{mhs: mhs}
}

var _ remote.ClientPipelineHandler = (*cliMetaHandler)(nil)

type cliMetaHandler struct {
	mhs []remote.MetaHandler
}

func (h *cliMetaHandler) OnConnectStream(ctx context.Context) (context.Context, error) {
	var err error
	for _, hdlr := range h.mhs {
		ctx, err = hdlr.OnConnectStream(ctx)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

// OnInactive implements the remote.InboundHandler interface.
func (h *cliMetaHandler) OnInactive(ctx context.Context, conn net.Conn) context.Context {
	return ctx
}

func (h *cliMetaHandler) OnStreamWrite(ctx context.Context, st remote.ClientStream, sendMsg remote.Message) (context.Context, error) {
	var err error
	for _, hdlr := range h.mhs {
		ctx, err = hdlr.WriteMeta(ctx, sendMsg)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

func (h *cliMetaHandler) OnStreamRead(ctx context.Context, st remote.ClientStream, msg remote.Message) (nctx context.Context, err error) {
	for _, hdlr := range h.mhs {
		ctx, err = hdlr.ReadMeta(ctx, msg)
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}
