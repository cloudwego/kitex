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
)

func (s *server) initStreamMiddlewares(ctx context.Context) {
	s.opt.Streaming.EventHandler = s.opt.TracerCtl.GetStreamEventHandler()
	s.opt.Streaming.InitMiddlewares(ctx)
}

func newStream(ctx context.Context, ss streaming.ServerStream, sendEP endpoint.SendEndpoint, recvEP endpoint.RecvEndpoint) *stream {
	st := &stream{
		ServerStream: ss,
	}
	if grpcStreamGetter, ok := ss.(streaming.GRPCStreamGetter); ok {
		grpcStream := grpcStreamGetter.GetGRPCStream()
		st.grpcStream = newGRPCStream(ctx, grpcStream, sendEP, recvEP)
	}
	return st
}

type stream struct {
	streaming.ServerStream
	grpcStream grpcStream
}

func (s *stream) GetGRPCStream() streaming.Stream {
	return &s.grpcStream
}

func newGRPCStream(ctx context.Context, st streaming.Stream, sendEP endpoint.SendEndpoint, recvEP endpoint.RecvEndpoint) grpcStream {
	return grpcStream{
		Stream:       st,
		ctx:          ctx,
		sendEndpoint: sendEP,
		recvEndpoint: recvEP,
	}
}

type grpcStream struct {
	streaming.Stream
	// it receives the ctx from previous middlewares and the Stream that exposed to users，then rewrite
	// Context() method so that users could call Stream.Context() in handler to get the processed ctx.
	ctx context.Context

	sendEndpoint endpoint.SendEndpoint
	recvEndpoint endpoint.RecvEndpoint
}

func (s *grpcStream) Context() context.Context {
	return s.ctx
}

func (s *grpcStream) RecvMsg(m interface{}) (err error) {
	return s.recvEndpoint(s.Stream, m)
}

func (s *grpcStream) SendMsg(m interface{}) (err error) {
	return s.sendEndpoint(s.Stream, m)
}

func invokeRecvEndpoint() endpoint.RecvEndpoint {
	return func(stream streaming.Stream, resp interface{}) (err error) {
		return stream.RecvMsg(resp)
	}
}

func invokeSendEndpoint() endpoint.SendEndpoint {
	return func(stream streaming.Stream, req interface{}) (err error) {
		return stream.SendMsg(req)
	}
}
