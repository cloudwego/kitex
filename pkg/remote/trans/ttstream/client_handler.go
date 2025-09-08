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

package ttstream

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream/ktx"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

var _ remote.ClientStreamFactory = (*clientTransHandler)(nil)

// NewCliTransHandlerFactory return a client trans handler factory.
func NewCliTransHandlerFactory(opts ...ClientHandlerOption) remote.ClientStreamFactory {
	cp := new(clientTransHandler)
	cp.transPool = newMuxConnTransPool(DefaultMuxConnConfig)
	for _, opt := range opts {
		opt(cp)
	}
	return cp
}

type clientTransHandler struct {
	transPool     transPool
	metaHandler   MetaFrameHandler
	headerHandler HeaderFrameWriteHandler
	traceCtl      *rpcinfo.TraceController
}

// NewStream creates a client stream
func (c clientTransHandler) NewStream(ctx context.Context, ri rpcinfo.RPCInfo) (streaming.ClientStream, error) {
	rconfig := ri.Config()
	invocation := ri.Invocation()
	method := invocation.MethodName()
	addr := ri.To().Address()
	if addr == nil {
		return nil, kerrors.ErrNoDestAddress
	}

	var strHeader streaming.Header
	var intHeader IntHeader
	var err error
	if c.headerHandler != nil {
		intHeader, strHeader, err = c.headerHandler.OnWriteStream(ctx)
		if err != nil {
			return nil, err
		}
	}
	if intHeader == nil {
		intHeader = IntHeader{}
	}
	if strHeader == nil {
		strHeader = map[string]string{}
	}
	strHeader[ttheader.HeaderIDLServiceName] = invocation.ServiceName()
	metainfo.SaveMetaInfoToMap(ctx, strHeader)

	trans, err := c.transPool.Get(addr.Network(), addr.String())
	if err != nil {
		return nil, err
	}

	// create new stream
	s := newStream(ctx, trans, streamFrame{sid: genStreamID(), method: method})
	// stream should be configured before WriteStream or there would be a race condition for metaFrameHandler
	s.setRecvTimeout(rconfig.StreamRecvTimeout())
	s.setMetaFrameHandler(c.metaHandler)
	s.setTraceController(c.traceCtl)
	s.setTraceContext(ctx)

	if err = trans.WriteStream(ctx, s, intHeader, strHeader); err != nil {
		return nil, err
	}

	// if ctx from server side, we should cancel the stream when server handler already returned
	registerStreamCancelCallback(ctx, s)

	cs := newClientStream(s)
	return cs, err
}

func registerStreamCancelCallback(ctx context.Context, s *stream) {
	ktx.RegisterCancelCallback(ctx, func() {
		_ = s.cancel()
		// it's safe to call closeSend twice
		// we do closeSend here to ensure stream can be closed normally
		s.closeSend(nil)
	})
}
