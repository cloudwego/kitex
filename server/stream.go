/*
 * Copyright 2023 CloudWeGo Authors
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

package server

import (
	"context"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/streamx"
)

func (s *server) initStreamMiddlewares(ctx context.Context) {
	// === for old version streaming ===
	s.opt.Streaming.EventHandler = s.opt.TracerCtl.GetStreamEventHandler()
	s.opt.Streaming.InitMiddlewares(ctx)

	// === for streamx version streaming ===
	// add tracing middlewares
	ehandler := s.opt.TracerCtl.GetStreamEventHandler()
	if ehandler != nil {
		s.opt.StreamX.StreamRecvMiddlewares = append(
			s.opt.StreamX.StreamRecvMiddlewares, streamx.NewStreamRecvStatMiddleware(ehandler),
		)
		s.opt.StreamX.StreamSendMiddlewares = append(
			s.opt.StreamX.StreamSendMiddlewares, streamx.NewStreamSendStatMiddleware(ehandler),
		)
	}
}

func (s *server) buildStreamInvokeChain() {
	// === for old version streaming ===
	s.opt.RemoteOpt.RecvEndpoint = s.opt.Streaming.BuildRecvInvokeChain(s.invokeRecvEndpoint())
	s.opt.RemoteOpt.SendEndpoint = s.opt.Streaming.BuildSendInvokeChain(s.invokeSendEndpoint())

	// === for streamx version streaming ===
	s.opt.RemoteOpt.StreamMiddleware = streamx.StreamMiddlewareChain(s.opt.StreamX.StreamMiddlewares...)
	s.opt.RemoteOpt.StreamRecvMiddleware = streamx.StreamRecvMiddlewareChain(s.opt.StreamX.StreamRecvMiddlewares...)
	s.opt.RemoteOpt.StreamSendMiddleware = streamx.StreamSendMiddlewareChain(s.opt.StreamX.StreamSendMiddlewares...)
}

func (s *server) invokeRecvEndpoint() endpoint.RecvEndpoint {
	return func(stream streaming.Stream, resp interface{}) (err error) {
		return stream.RecvMsg(resp)
	}
}

func (s *server) invokeSendEndpoint() endpoint.SendEndpoint {
	return func(stream streaming.Stream, req interface{}) (err error) {
		return stream.SendMsg(req)
	}
}

// contextStream is responsible for solving ctx diverge in server side streaming.
// it receives the ctx from previous middlewares and the Stream that exposed to users，then rewrite
// Context() method so that users could call Stream.Context() in handler to get the processed ctx.
type contextStream struct {
	streaming.Stream
	ctx context.Context
}

func (cs contextStream) Context() context.Context {
	return cs.ctx
}
