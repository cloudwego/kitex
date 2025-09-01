/*
 * Copyright 2025 CloudWeGo Authors
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

package generic

import (
	"context"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// ClientStreamingServer define server side client streaming APIs
type ClientStreamingServer interface {
	// Recv receives a message from the client.
	Recv(ctx context.Context) (req interface{}, err error)
	// SendAndClose sends a message to the client and closes the stream.
	SendAndClose(ctx context.Context, res interface{}) error
	// SetHeader inherits from the underlying streaming.ServerStream.
	SetHeader(hd streaming.Header) error
	// SendHeader inherits from the underlying streaming.ServerStream.
	SendHeader(hd streaming.Header) error
	// SetTrailer inherits from the underlying streaming.ServerStream.
	SetTrailer(hd streaming.Trailer) error
	// Streaming returns the underlying streaming.ServerStream.
	Streaming() streaming.ServerStream
}

type clientStreamingServer struct {
	methodInfo serviceinfo.MethodInfo
	streaming  streaming.ServerStream
}

func (s *clientStreamingServer) Recv(ctx context.Context) (req interface{}, err error) {
	args := s.methodInfo.NewArgs().(*Args)
	if err = s.streaming.RecvMsg(ctx, args); err != nil {
		return
	}
	req = args.Request
	return
}

func (s *clientStreamingServer) SendAndClose(ctx context.Context, res interface{}) error {
	results := s.methodInfo.NewResult().(*Result)
	results.Success = res
	return s.streaming.SendMsg(ctx, results)
}

func (s *clientStreamingServer) SetHeader(hd streaming.Header) error {
	return s.streaming.SetHeader(hd)
}

func (s *clientStreamingServer) SendHeader(hd streaming.Header) error {
	return s.streaming.SendHeader(hd)
}

func (s *clientStreamingServer) SetTrailer(hd streaming.Trailer) error {
	return s.streaming.SetTrailer(hd)
}

func (s *clientStreamingServer) Streaming() streaming.ServerStream {
	return s.streaming
}

// ServerStreamingServer define server side server streaming APIs
type ServerStreamingServer interface {
	// Send sends a message to the client.
	Send(ctx context.Context, res interface{}) error
	// SetHeader inherits from the underlying streaming.ServerStream.
	SetHeader(hd streaming.Header) error
	// SendHeader inherits from the underlying streaming.ServerStream.
	SendHeader(hd streaming.Header) error
	// SetTrailer inherits from the underlying streaming.ServerStream.
	SetTrailer(hd streaming.Trailer) error
	// Streaming returns the underlying streaming.ServerStream.
	Streaming() streaming.ServerStream
}

type serverStreamingServer struct {
	methodInfo serviceinfo.MethodInfo
	streaming  streaming.ServerStream
}

func (s *serverStreamingServer) Send(ctx context.Context, res interface{}) error {
	results := s.methodInfo.NewResult().(*Result)
	results.Success = res
	return s.streaming.SendMsg(ctx, results)
}

func (s *serverStreamingServer) SetHeader(hd streaming.Header) error {
	return s.streaming.SetHeader(hd)
}

func (s *serverStreamingServer) SendHeader(hd streaming.Header) error {
	return s.streaming.SendHeader(hd)
}

func (s *serverStreamingServer) SetTrailer(hd streaming.Trailer) error {
	return s.streaming.SetTrailer(hd)
}

func (s *serverStreamingServer) Streaming() streaming.ServerStream {
	return s.streaming
}

// BidiStreamingServer define server side bidi streaming APIs
type BidiStreamingServer interface {
	// Recv receives a message from the client.
	Recv(ctx context.Context) (req interface{}, err error)
	// Send sends a message to the client.
	Send(ctx context.Context, res interface{}) error
	// SetHeader inherits from the underlying streaming.ServerStream.
	SetHeader(hd streaming.Header) error
	// SendHeader inherits from the underlying streaming.ServerStream.
	SendHeader(hd streaming.Header) error
	// SetTrailer inherits from the underlying streaming.ServerStream.
	SetTrailer(hd streaming.Trailer) error
	// Streaming returns the underlying streaming.ServerStream.
	Streaming() streaming.ServerStream
}

type bidiStreamingServer struct {
	methodInfo serviceinfo.MethodInfo
	streaming  streaming.ServerStream
}

func (s *bidiStreamingServer) Recv(ctx context.Context) (req interface{}, err error) {
	args := s.methodInfo.NewArgs().(*Args)
	if err = s.streaming.RecvMsg(ctx, args); err != nil {
		return
	}
	req = args.Request
	return
}

func (s *bidiStreamingServer) Send(ctx context.Context, res interface{}) error {
	results := s.methodInfo.NewResult().(*Result)
	results.Success = res
	return s.streaming.SendMsg(ctx, results)
}

func (s *bidiStreamingServer) SetHeader(hd streaming.Header) error {
	return s.streaming.SetHeader(hd)
}

func (s *bidiStreamingServer) SendHeader(hd streaming.Header) error {
	return s.streaming.SendHeader(hd)
}

func (s *bidiStreamingServer) SetTrailer(hd streaming.Trailer) error {
	return s.streaming.SetTrailer(hd)
}

func (s *bidiStreamingServer) Streaming() streaming.ServerStream {
	return s.streaming
}
