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

package streaming

import (
	"context"

	"github.com/cloudwego/kitex/pkg/stats"
)

/* Streaming Mode
---------------  [Unary Streaming]  ---------------
--------------- (Req) returns (Res) ---------------
client.Send(req)   === req ==>   server.Recv(req)
client.Recv(res)   <== res ===   server.Send(res)


------------------- [Client Streaming] -------------------
--------------- (stream Req) returns (Res) ---------------
client.Send(req)         === req ==>       server.Recv(req)
                             ...
client.Send(req)         === req ==>       server.Recv(req)

client.CloseSend()       === EOF ==>       server.Recv(EOF)
client.Recv(res)         <== res ===       server.SendAndClose(res)
** OR
client.CloseAndRecv(res) === EOF ==>       server.Recv(EOF)
                         <== res ===       server.SendAndClose(res)


------------------- [Server Streaming] -------------------
---------- (Request) returns (stream Response) ----------
client.Send(req)   === req ==>   server.Recv(req)
client.CloseSend() === EOF ==>   server.Recv(EOF)
client.Recv(res)   <== res ===   server.Send(req)
                       ...
client.Recv(res)   <== res ===   server.Send(req)
client.Recv(EOF)   <== EOF ===   server handler return


----------- [Bidirectional Streaming] -----------
--- (stream Request) returns (stream Response) ---
* goroutine 1 *
client.Send(req)   === req ==>   server.Recv(req)
                       ...
client.Send(req)   === req ==>   server.Recv(req)
client.CloseSend() === EOF ==>   server.Recv(EOF)

* goroutine 2 *
client.Recv(res)   <== res ===   server.Send(req)
                       ...
client.Recv(res)   <== res ===   server.Send(req)
client.Recv(EOF)   <== EOF ===   server handler return
*/

type (
	Header  map[string]string
	Trailer map[string]string
)

// ClientStream define client stream APIs
type ClientStream interface {
	// SendMsg send message to peer
	// will block until an error or enough buffer to send
	// not concurrent-safety
	SendMsg(ctx context.Context, m any) error
	// RecvMsg recvive message from peer
	// will block until an error or a message received
	// not concurrent-safety
	RecvMsg(ctx context.Context, m any) error
	// Header returns the header info received from the server if there
	// is any. It blocks if the header info is not ready to read.  If the header info
	// is nil and the error is also nil, then the stream was terminated without
	// headers, and the error can be discovered by calling RecvMsg.
	Header() (Header, error)
	// Trailer returns the trailer info from the server, if there is any.
	// It must only be called after stream.CloseAndRecv has returned, or
	// stream.Recv has returned a non-nil error (including io.EOF).
	Trailer() (Trailer, error)
	// CloseSend closes the send direction of the stream. It closes the stream
	// when non-nil error is met. It is also not safe to call CloseSend
	// concurrently with SendMsg.
	CloseSend(ctx context.Context) error
	// Context the stream context.Context
	Context() context.Context
}

// ServerStream define server stream APIs
type ServerStream interface {
	// SendMsg send message to peer
	// will block until an error or enough buffer to send
	// not concurrent-safety
	SendMsg(ctx context.Context, m any) error
	// RecvMsg recvive message from peer
	// will block until an error or a message received
	// not concurrent-safety
	RecvMsg(ctx context.Context, m any) error
	// SetHeader sets the header info. It may be called multiple times.
	// When call multiple times, all the provided header info will be merged.
	// All the header info will be sent out when one of the following happens:
	//  - ServerStream.SendHeader() is called;
	//  - The first response is sent out;
	//  - The server handler returns (error or nil).
	SetHeader(hd Header) error
	// SendHeader sends the header info.
	// The provided hd and headers set by SetHeader() will be sent.
	// It fails if called multiple times or called after the first response is sent out.
	SendHeader(hd Header) error
	// SetTrailer sets the trailer info which will be sent with the RPC status.
	// When called more than once, all the provided trailer info will be merged.
	SetTrailer(hd Trailer) error
}

// ServerStreamingClient define client side server streaming APIs
type ServerStreamingClient[Res any] interface {
	Recv(ctx context.Context) (*Res, error)
	ClientStream
}

// NewServerStreamingClient creates an implementation of ServerStreamingClient[Res].
func NewServerStreamingClient[Res any](st ClientStream) ServerStreamingClient[Res] {
	return &serverStreamingClientImpl[Res]{ClientStream: st}
}

type serverStreamingClientImpl[Res any] struct {
	ClientStream
}

func (s *serverStreamingClientImpl[Res]) Recv(ctx context.Context) (*Res, error) {
	m := new(Res)
	return m, s.ClientStream.RecvMsg(ctx, m)
}

// ServerStreamingServer define server side server streaming APIs
type ServerStreamingServer[Res any] interface {
	Send(ctx context.Context, res *Res) error
	ServerStream
}

// NewServerStreamingServer creates an implementation of ServerStreamingServer[Res].
func NewServerStreamingServer[Res any](st ServerStream) ServerStreamingServer[Res] {
	return &serverStreamingServerImpl[Res]{ServerStream: st}
}

type serverStreamingServerImpl[Res any] struct {
	ServerStream
}

func (s *serverStreamingServerImpl[Res]) Send(ctx context.Context, res *Res) error {
	return s.ServerStream.SendMsg(ctx, res)
}

// ClientStreamingClient define client side client streaming APIs
type ClientStreamingClient[Req, Res any] interface {
	Send(ctx context.Context, req *Req) error
	CloseAndRecv(ctx context.Context) (*Res, error)
	ClientStream
}

// NewClientStreamingClient creates an implementation of ClientStreamingClient[Req, Res].
func NewClientStreamingClient[Req, Res any](st ClientStream) ClientStreamingClient[Req, Res] {
	return &clientStreamingClientImpl[Req, Res]{ClientStream: st}
}

type clientStreamingClientImpl[Req, Res any] struct {
	ClientStream
}

func (s *clientStreamingClientImpl[Req, Res]) Send(ctx context.Context, req *Req) error {
	return s.ClientStream.SendMsg(ctx, req)
}

func (s *clientStreamingClientImpl[Req, Res]) CloseAndRecv(ctx context.Context) (*Res, error) {
	if err := s.ClientStream.CloseSend(ctx); err != nil {
		return nil, err
	}
	m := new(Res)
	return m, s.ClientStream.RecvMsg(ctx, m)
}

// ClientStreamingServer define server side client streaming APIs
type ClientStreamingServer[Req, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	SendAndClose(ctx context.Context, res *Res) error
	ServerStream
}

// NewClientStreamingServer creates an implementation of ClientStreamingServer[Req, Res].
func NewClientStreamingServer[Req, Res any](st ServerStream) ClientStreamingServer[Req, Res] {
	return &clientStreamingServerImpl[Req, Res]{ServerStream: st}
}

type clientStreamingServerImpl[Req, Res any] struct {
	ServerStream
}

func (s *clientStreamingServerImpl[Req, Res]) Recv(ctx context.Context) (*Req, error) {
	m := new(Req)
	return m, s.ServerStream.RecvMsg(ctx, m)
}

func (s *clientStreamingServerImpl[Req, Res]) SendAndClose(ctx context.Context, res *Res) error {
	return s.ServerStream.SendMsg(ctx, res)
}

// BidiStreamingClient define client side bidi streaming APIs
type BidiStreamingClient[Req, Res any] interface {
	Send(ctx context.Context, req *Req) error
	Recv(ctx context.Context) (*Res, error)
	ClientStream
}

// NewBidiStreamingClient creates an implementation of BidiStreamingClient[Req, Res].
func NewBidiStreamingClient[Req, Res any](st ClientStream) BidiStreamingClient[Req, Res] {
	return &bidiStreamingClientImpl[Req, Res]{ClientStream: st}
}

type bidiStreamingClientImpl[Req, Res any] struct {
	ClientStream
}

func (s *bidiStreamingClientImpl[Req, Res]) Send(ctx context.Context, req *Req) error {
	return s.ClientStream.SendMsg(ctx, req)
}

func (s *bidiStreamingClientImpl[Req, Res]) Recv(ctx context.Context) (*Res, error) {
	m := new(Res)
	return m, s.ClientStream.RecvMsg(ctx, m)
}

// BidiStreamingServer define server side bidi streaming APIs
type BidiStreamingServer[Req, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	Send(ctx context.Context, res *Res) error
	ServerStream
}

// NewBidiStreamingServer creates an implementation of BidiStreamingServer[Req, Res].
func NewBidiStreamingServer[Req, Res any](st ServerStream) BidiStreamingServer[Req, Res] {
	return &bidiStreamingServerImpl[Req, Res]{ServerStream: st}
}

type bidiStreamingServerImpl[Req, Res any] struct {
	ServerStream
}

func (s *bidiStreamingServerImpl[Req, Res]) Recv(ctx context.Context) (*Req, error) {
	m := new(Req)
	return m, s.ServerStream.RecvMsg(ctx, m)
}

func (s *bidiStreamingServerImpl[Req, Res]) Send(ctx context.Context, res *Res) error {
	return s.ServerStream.SendMsg(ctx, res)
}

// EventHandler define stats event handler
type EventHandler func(ctx context.Context, evt stats.Event, err error)
