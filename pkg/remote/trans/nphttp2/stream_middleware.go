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

package nphttp2

import (
	"context"

	ep "github.com/cloudwego/kitex/pkg/endpoint"
	sep "github.com/cloudwego/kitex/pkg/endpoint/server"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// streamWithMiddleware enables interception for stream's SendMsg and RecvMsg.
type streamWithMiddleware struct {
	*serverStream

	recv sep.StreamRecvEndpoint
	send sep.StreamSendEndpoint

	gs *grpcStreamWithMiddleware
}

func newStreamWithMiddleware(st *serverStream, recv sep.StreamRecvEndpoint, send sep.StreamSendEndpoint,
	grpcRecv ep.RecvEndpoint, grpcSend ep.SendEndpoint,
) *streamWithMiddleware {
	return &streamWithMiddleware{
		serverStream: st,
		recv:         recv,
		send:         send,
		gs: &grpcStreamWithMiddleware{
			grpcServerStream: st.grpcStream,
			recv:             grpcRecv,
			send:             grpcSend,
		},
	}
}

func (s *streamWithMiddleware) RecvMsg(ctx context.Context, m interface{}) error {
	return s.recv(ctx, s.serverStream, m)
}

func (s *streamWithMiddleware) SendMsg(ctx context.Context, m interface{}) error {
	return s.send(ctx, s.serverStream, m)
}

func (s *streamWithMiddleware) GetGRPCStream() streaming.Stream {
	return s.gs
}

type grpcStreamWithMiddleware struct {
	*grpcServerStream
	recv ep.RecvEndpoint
	send ep.SendEndpoint
}

func (s *grpcStreamWithMiddleware) RecvMsg(m interface{}) error {
	return s.recv(s.grpcServerStream, m)
}

func (s *grpcStreamWithMiddleware) SendMsg(m interface{}) error {
	return s.send(s.grpcServerStream, m)
}
