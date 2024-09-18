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

package streamx

import (
	"context"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var _ ServerStreamingClient[int] = (*GenericClientStream[int, int])(nil)
var _ ClientStreamingClient[int, int] = (*GenericClientStream[int, int])(nil)
var _ BidiStreamingClient[int, int] = (*GenericClientStream[int, int])(nil)
var _ ServerStreamingServer[int] = (*GenericServerStream[int, int])(nil)
var _ ClientStreamingServer[int, int] = (*GenericServerStream[int, int])(nil)
var _ BidiStreamingServer[int, int] = (*GenericServerStream[int, int])(nil)

type StreamingMode = serviceinfo.StreamingMode

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

const (
	StreamingNone          = serviceinfo.StreamingNone
	StreamingUnary         = serviceinfo.StreamingUnary
	StreamingClient        = serviceinfo.StreamingClient
	StreamingServer        = serviceinfo.StreamingServer
	StreamingBidirectional = serviceinfo.StreamingBidirectional
)

type Stream interface {
	Mode() StreamingMode
	Service() string
	Method() string
	SendMsg(ctx context.Context, m any) error
	RecvMsg(ctx context.Context, m any) error
}

type ClientStream interface {
	Stream
	CloseSend(ctx context.Context) error
}

type ServerStream interface {
	Stream
}

// client 必须通过 metainfo.WithValue(ctx, ..) 给下游传递信息
// client 必须通过 metainfo.GetValue(ctx, ..) 拿到当前 server 的透传信息
// client 必须通过 Header() 拿到下游 server 的透传信息
type ClientStreamMetadata interface {
	Header() (Header, error)
	Trailer() (Trailer, error)
}

// server 可以通过 Set/SendXXX 给上游回传信息
type ServerStreamMetadata interface {
	SetHeader(hd Header) error
	SendHeader(hd Header) error
	SetTrailer(hd Trailer) error
}

type ServerStreamingClient[Res any] interface {
	Recv(ctx context.Context) (*Res, error)
	ClientStream
	ClientStreamMetadata
}

type ServerStreamingServer[Res any] interface {
	Send(ctx context.Context, res *Res) error
	ServerStream
	ServerStreamMetadata
}

type ClientStreamingClient[Req, Res any] interface {
	Send(ctx context.Context, req *Req) error
	CloseAndRecv(ctx context.Context) (*Res, error)
	ClientStream
	ClientStreamMetadata
}

type ClientStreamingServer[Req, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	//SendAndClose(ctx context.Context, res *Res) error
	ServerStream
	ServerStreamMetadata
}

type BidiStreamingClient[Req, Res any] interface {
	Send(ctx context.Context, req *Req) error
	Recv(ctx context.Context) (*Res, error)
	ClientStream
	ClientStreamMetadata
}

type BidiStreamingServer[Req, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	Send(ctx context.Context, res *Res) error
	ServerStream
	ServerStreamMetadata
}

type GenericStreamIOMiddlewareSetter interface {
	SetStreamSendEndpoint(e StreamSendEndpoint)
	SetStreamRecvEndpoint(e StreamSendEndpoint)
}

func NewGenericClientStream[Req, Res any](cs ClientStream) *GenericClientStream[Req, Res] {
	return &GenericClientStream[Req, Res]{
		ClientStream:         cs,
		ClientStreamMetadata: cs.(ClientStreamMetadata),
	}
}

type GenericClientStream[Req, Res any] struct {
	ClientStream
	ClientStreamMetadata
	StreamSendMiddleware
	StreamRecvMiddleware
}

func (x *GenericClientStream[Req, Res]) SetStreamSendMiddleware(e StreamSendMiddleware) {
	x.StreamSendMiddleware = e
}

func (x *GenericClientStream[Req, Res]) SetStreamRecvMiddleware(e StreamRecvMiddleware) {
	x.StreamRecvMiddleware = e
}

func (x *GenericClientStream[Req, Res]) SendMsg(ctx context.Context, m any) error {
	if x.StreamSendMiddleware != nil {
		return x.StreamSendMiddleware(streamSendNext)(ctx, x.ClientStream, m)
	}
	return x.ClientStream.SendMsg(ctx, m)
}

func (x *GenericClientStream[Req, Res]) RecvMsg(ctx context.Context, m any) (err error) {
	if x.StreamRecvMiddleware != nil {
		err = x.StreamRecvMiddleware(streamRecvNext)(ctx, x.ClientStream, m)
	} else {
		err = x.ClientStream.RecvMsg(ctx, m)
	}
	return err
}

func (x *GenericClientStream[Req, Res]) Send(ctx context.Context, m *Req) error {
	return x.SendMsg(ctx, m)
}

func (x *GenericClientStream[Req, Res]) Recv(ctx context.Context) (m *Res, err error) {
	m = new(Res)
	if err = x.RecvMsg(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (x *GenericClientStream[Req, Res]) CloseAndRecv(ctx context.Context) (*Res, error) {
	if err := x.ClientStream.CloseSend(ctx); err != nil {
		return nil, err
	}
	return x.Recv(ctx)
}

func NewGenericServerStream[Req, Res any](ss ServerStream) *GenericServerStream[Req, Res] {
	return &GenericServerStream[Req, Res]{
		ServerStream:         ss,
		ServerStreamMetadata: ss.(ServerStreamMetadata),
	}
}

type GenericServerStream[Req, Res any] struct {
	ServerStream
	ServerStreamMetadata
	StreamSendMiddleware
	StreamRecvMiddleware
}

func (x *GenericServerStream[Req, Res]) SetStreamSendMiddleware(e StreamSendMiddleware) {
	x.StreamSendMiddleware = e
}

func (x *GenericServerStream[Req, Res]) SetStreamRecvMiddleware(e StreamRecvMiddleware) {
	x.StreamRecvMiddleware = e
}

func (x *GenericServerStream[Req, Res]) SendMsg(ctx context.Context, m any) error {
	if x.StreamSendMiddleware != nil {
		return x.StreamSendMiddleware(streamSendNext)(ctx, x.ServerStream, m)
	}
	return x.ServerStream.SendMsg(ctx, m)
}

func (x *GenericServerStream[Req, Res]) RecvMsg(ctx context.Context, m any) (err error) {
	if x.StreamRecvMiddleware != nil {
		err = x.StreamRecvMiddleware(streamRecvNext)(ctx, x.ServerStream, m)
	} else {
		err = x.ServerStream.RecvMsg(ctx, m)
	}
	return err
}

func (x *GenericServerStream[Req, Res]) Send(ctx context.Context, m *Res) error {
	if x.StreamSendMiddleware != nil {
		return x.StreamSendMiddleware(streamSendNext)(ctx, x.ServerStream, m)
	}
	return x.ServerStream.SendMsg(ctx, m)
}

func (x *GenericServerStream[Req, Res]) SendAndClose(ctx context.Context, m *Res) error {
	return x.Send(ctx, m)
}

func (x *GenericServerStream[Req, Res]) Recv(ctx context.Context) (m *Req, err error) {
	m = new(Req)
	if x.StreamRecvMiddleware != nil {
		err = x.StreamRecvMiddleware(streamRecvNext)(ctx, x.ServerStream, m)
	} else {
		err = x.ServerStream.RecvMsg(ctx, m)
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

func streamSendNext(ctx context.Context, stream Stream, msg any) (err error) {
	return stream.SendMsg(ctx, msg)
}

func streamRecvNext(ctx context.Context, stream Stream, msg any) (err error) {
	return stream.RecvMsg(ctx, msg)
}
