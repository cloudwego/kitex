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

// Package remotecli for remote client
package remotecli

import (
	"context"
	"fmt"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/transport"
)

// NewStream create a client side stream
func NewStream(ctx context.Context, ri rpcinfo.RPCInfo, handler remote.ClientTransHandler, opt *remote.ClientOption) (streaming.ClientStream, *StreamConnManager, error) {
	var err error
	for _, shdlr := range opt.StreamingMetaHandlers {
		ctx, err = shdlr.OnConnectStream(ctx)
		if err != nil {
			return nil, nil, err
		}
	}

	tp := ri.Config().TransportProtocol()

	if tp&transport.GRPC == transport.GRPC {
		// grpc streaming
		connPool := opt.GRPCStreamingConnPool
		if connPool == nil {
			connPool = opt.ConnPool
		}
		cm := NewConnWrapper(connPool)
		rawConn, err := cm.GetConn(ctx, opt.Dialer, ri)
		if err != nil {
			return nil, nil, err
		}
		return nphttp2.NewClientStream(ctx, opt.SvcInfo, rawConn, handler), &StreamConnManager{cm}, nil
	}

	// ttheader streaming
	if tp&transport.TTHeaderStreaming == transport.TTHeaderStreaming {
		// wrap internal client provider
		cs, err := opt.TTHeaderStreamingProvider.NewStream(ctx, ri)
		if err != nil {
			return nil, nil, err
		}
		return cs, nil, nil
	}
	return nil, nil, fmt.Errorf("no matching transport protocol for stream call, tp=%s", tp.String())
}

// NewStreamConnManager returns a new StreamConnManager
func NewStreamConnManager(cr ConnReleaser) *StreamConnManager {
	return &StreamConnManager{ConnReleaser: cr}
}

// StreamConnManager manages the underlying connection of the stream
type StreamConnManager struct {
	ConnReleaser
}

// ReleaseConn releases the raw connection of the stream
func (scm *StreamConnManager) ReleaseConn(err error, ri rpcinfo.RPCInfo) {
	scm.ConnReleaser.ReleaseConn(err, ri)
}
