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
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	internal_stream "github.com/cloudwego/kitex/internal/stream"
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
	var transmissionTimeoutEffect bool
	// retrieve deadline from context as the whole stream timeout
	if ddl, ok := ctx.Deadline(); ok {
		tm := time.Until(ddl)
		intHeader[ttheader.RPCTimeout] = strconv.Itoa(int(tm.Milliseconds()))
		// When user using a ctx with transmission timeout and handles the ctx with context.WithTimeout or context.WithDeadline,
		// we should determine whether the deadline injected by the user is valid to optimized error returning.
		// The logic of context.WithTimeout is that the new deadline will only take effect if new deadline < old deadline,
		// we just judge it manually here.
		transDdl, ok := internal_stream.GetTransmissionDeadline(ctx)
		if ok && !ddl.After(transDdl) {
			transmissionTimeoutEffect = true
		}
	}
	strHeader[ttheader.HeaderIDLServiceName] = invocation.ServiceName()
	metainfo.SaveMetaInfoToMap(ctx, strHeader)

	trans, err := c.transPool.Get(addr.Network(), addr.String())
	if err != nil {
		return nil, err
	}

	// create new stream
	s := newStreamForClientSide(ctx, trans, streamFrame{sid: genStreamID(), method: method})
	// stream should be configured before WriteStream or there would be a race condition for metaFrameHandler
	s.setRecvTimeout(rconfig.StreamRecvTimeout())
	s.setMetaFrameHandler(c.metaHandler)
	s.setTransmissionTimeoutEffect(transmissionTimeoutEffect)

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

func (s *stream) convertContextErr(ctx context.Context) error {
	cErr := ctx.Err()
	switch cErr {
	// biz code invokes cancel()
	case context.Canceled:
		return errBizCancel
	case context.DeadlineExceeded:
		// triggered by upstream transmission timeout
		// - gRPC handler
		// - TTHeader Streaming handler
		// - thrift handler when server.WithEnableContextTimeout is configured
		// todo: considering print information of case above
		tm, ok := internal_stream.GetTransmissionTimeout(ctx)
		if ok && s.transmissionTimeoutEffect {
			return errUpstreamTransmissionTimeout.WithCause(fmt.Errorf("timeout: %s", tm))
		}
		// biz code injects timeout
		return errBizTimeout
	}

	// todo: consider cancelCause API
	//// biz code make use of cancelCause
	//if cause(ctx) {
	//	return errBizCancelWithCause.WithCause(cErr)
	//}
	if ex, ok := cErr.(tException); ok {
		return ex
	}
	return errInternalCancel.WithCause(cErr)
}
