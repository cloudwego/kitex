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

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
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
	protocolID, err := getProtocolID(ri)
	if err != nil {
		return nil, err
	}

	var strHeader streaming.Header
	var intHeader IntHeader
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
	cs := newClientStream(ctx, trans, streamFrame{sid: genStreamID(), method: method, protocolID: protocolID})
	// stream should be configured before WriteStream or there would be a race condition for metaFrameHandler
	cs.setRecvTimeout(rconfig.StreamRecvTimeout())
	cs.setMetaFrameHandler(c.metaHandler)
	cs.setTraceController(c.traceCtl)

	if err = trans.WriteStream(ctx, cs, intHeader, strHeader); err != nil {
		return nil, err
	}

	cs.handleStreamStartEvent(rpcinfo.StreamStartEvent{})

	return cs, err
}

func getProtocolID(ri rpcinfo.RPCInfo) (ttheader.ProtocolID, error) {
	switch ri.Config().PayloadCodec() {
	case serviceinfo.Thrift:
		return ttheader.ProtocolIDThriftStruct, nil
	case serviceinfo.Protobuf:
		return ttheader.ProtocolIDProtobufStruct, nil
	default:
		return 0, fmt.Errorf("not supported payload type: %v", ri.Config().PayloadCodec())
	}
}
