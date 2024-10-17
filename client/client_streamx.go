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

package client

import (
	"context"

	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
)

type StreamX interface {
	NewStream(ctx context.Context, method string, req any, callOptions ...streamxcallopt.CallOption) (streamx.ClientStream, error)
	Middlewares() (streamMW streamx.StreamMiddleware, recvMW streamx.StreamRecvMiddleware, sendMW streamx.StreamSendMiddleware)
}

func (kc *kClient) Middlewares() (streamMW streamx.StreamMiddleware, recvMW streamx.StreamRecvMiddleware, sendMW streamx.StreamSendMiddleware) {
	return kc.sxStreamMW, kc.sxStreamRecvMW, kc.sxStreamSendMW
}

// NewStream create stream for streamx mode
func (kc *kClient) NewStream(ctx context.Context, method string, req any, callOptions ...streamxcallopt.CallOption) (streamx.ClientStream, error) {
	if !kc.inited {
		panic("client not initialized")
	}
	if kc.closed {
		panic("client is already closed")
	}
	if ctx == nil {
		panic("ctx is nil")
	}
	var ri rpcinfo.RPCInfo
	ctx, ri, _ = kc.initRPCInfo(ctx, method, 0, nil)

	err := rpcinfo.AsMutableRPCConfig(ri.Config()).SetInteractionMode(rpcinfo.Streaming)
	if err != nil {
		return nil, err
	}
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	ctx = kc.opt.TracerCtl.DoStart(ctx, ri)

	streamArgs := streamx.NewStreamArgs(nil)
	// put streamArgs into response arg
	// it's an ugly trick but if we don't want to refactor too much,
	// this is the only way to compatible with current endpoint design
	err = kc.sEps(ctx, req, streamArgs)
	kc.opt.TracerCtl.DoFinish(ctx, ri, err)
	if err != nil {
		return nil, err
	}
	return streamArgs.Stream().(streamx.ClientStream), nil
}
