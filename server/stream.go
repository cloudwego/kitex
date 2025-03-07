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

	internal_stream "github.com/cloudwego/kitex/internal/stream"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/endpoint/sep"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
)

func (s *server) wrapStreamMiddleware() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		// recvEP and sendEP are the endpoints for the new stream interface,
		// and grpcRecvEP and grpcSendEP are the endpoints for the old grpc stream interface,
		// the latter two are going to be removed in the future.
		sendEP := s.opt.StreamOptions.BuildSendChain()
		recvEP := s.opt.StreamOptions.BuildRecvChain()
		grpcSendEP := s.opt.Streaming.BuildSendInvokeChain()
		grpcRecvEP := s.opt.Streaming.BuildRecvInvokeChain()
		return func(ctx context.Context, req, resp interface{}) (err error) {
			if st, ok := req.(*streaming.Args); ok {
				nst := newStream(ctx, st.ServerStream, sendEP, recvEP, s.opt.StreamOptions.EventHandler, grpcSendEP, grpcRecvEP)
				st.ServerStream = nst
				st.Stream = nst.GetGRPCStream()
			}
			return next(ctx, req, resp)
		}
	}
}

func newStream(ctx context.Context, s streaming.ServerStream, sendEP sep.StreamSendEndpoint, recvEP sep.StreamRecvEndpoint,
	eventHandler internal_stream.StreamEventHandler, grpcSendEP endpoint.SendEndpoint, grpcRecvEP endpoint.RecvEndpoint,
) *stream {
	st := &stream{
		ServerStream: s,
		ctx:          ctx,
		recv:         recvEP,
		send:         sendEP,
		eventHandler: eventHandler,
	}
	if grpcStreamGetter, ok := s.(streaming.GRPCStreamGetter); ok {
		if grpcStream := grpcStreamGetter.GetGRPCStream(); grpcStream != nil {
			st.grpcStream = newGRPCStream(grpcStream, grpcSendEP, grpcRecvEP)
			st.grpcStream.st = st
		}
	}
	return st
}

type stream struct {
	streaming.ServerStream
	grpcStream   *grpcStream
	ctx          context.Context
	eventHandler internal_stream.StreamEventHandler

	recv sep.StreamRecvEndpoint
	send sep.StreamSendEndpoint
}

var _ streaming.GRPCStreamGetter = (*stream)(nil)

func (s *stream) GetGRPCStream() streaming.Stream {
	if s.grpcStream == nil {
		return nil
	}
	return s.grpcStream
}

// RecvMsg receives a message from the client.
func (s *stream) RecvMsg(ctx context.Context, m interface{}) (err error) {
	err = s.recv(ctx, s.ServerStream, m)
	if s.eventHandler != nil {
		s.eventHandler(s.ctx, stats.StreamRecv, err)
	}
	return
}

// SendMsg sends a message to the client.
func (s *stream) SendMsg(ctx context.Context, m interface{}) (err error) {
	err = s.send(ctx, s.ServerStream, m)
	if s.eventHandler != nil {
		s.eventHandler(s.ctx, stats.StreamSend, err)
	}
	return
}

func newGRPCStream(st streaming.Stream, sendEP endpoint.SendEndpoint, recvEP endpoint.RecvEndpoint) *grpcStream {
	return &grpcStream{
		Stream:       st,
		sendEndpoint: sendEP,
		recvEndpoint: recvEP,
	}
}

type grpcStream struct {
	streaming.Stream

	st *stream

	sendEndpoint endpoint.SendEndpoint
	recvEndpoint endpoint.RecvEndpoint
}

func (s *grpcStream) RecvMsg(m interface{}) (err error) {
	err = s.recvEndpoint(s.Stream, m)
	if s.st.eventHandler != nil {
		s.st.eventHandler(s.st.ctx, stats.StreamRecv, err)
	}
	return
}

func (s *grpcStream) SendMsg(m interface{}) (err error) {
	err = s.sendEndpoint(s.Stream, m)
	if s.st.eventHandler != nil {
		s.st.eventHandler(s.st.ctx, stats.StreamSend, err)
	}
	return
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
