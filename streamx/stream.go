package streamx

import (
	"context"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var _ ServerStreamingClient[int, int, int] = (*GenericClientStream[int, int, int, int])(nil)
var _ ClientStreamingClient[int, int, int, int] = (*GenericClientStream[int, int, int, int])(nil)
var _ BidiStreamingClient[int, int, int, int] = (*GenericClientStream[int, int, int, int])(nil)
var _ ServerStreamingServer[int, int, int] = (*GenericServerStream[int, int, int, int])(nil)
var _ ClientStreamingServer[int, int, int, int] = (*GenericServerStream[int, int, int, int])(nil)
var _ BidiStreamingServer[int, int, int, int] = (*GenericServerStream[int, int, int, int])(nil)

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
type ClientStreamMetadata[Header, Trailer any] interface {
	Header() (Header, error)
	Trailer() (Trailer, error)
}

// server 可以通过 Set/SendXXX 给上游回传信息
type ServerStreamMetadata[Header, Trailer any] interface {
	SetHeader(hd Header) error
	SendHeader(hd Header) error
	SetTrailer(hd Trailer) error
}

type ServerStreamingClient[Header, Trailer, Res any] interface {
	Recv(ctx context.Context) (*Res, error)
	ClientStream
	ClientStreamMetadata[Header, Trailer]
}

type ServerStreamingServer[Header, Trailer, Res any] interface {
	Send(ctx context.Context, res *Res) error
	ServerStream
	ServerStreamMetadata[Header, Trailer]
}

type ClientStreamingClient[Header, Trailer, Req, Res any] interface {
	Send(ctx context.Context, req *Req) error
	CloseAndRecv(ctx context.Context) (*Res, error)
	ClientStream
	ClientStreamMetadata[Header, Trailer]
}

type ClientStreamingServer[Header, Trailer, Req, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	//SendAndClose(ctx context.Context, res *Res) error
	ServerStream
	ServerStreamMetadata[Header, Trailer]
}

type BidiStreamingClient[Header, Trailer, Req, Res any] interface {
	Send(ctx context.Context, req *Req) error
	Recv(ctx context.Context) (*Res, error)
	ClientStream
	ClientStreamMetadata[Header, Trailer]
}

type BidiStreamingServer[Header, Trailer, Req, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	Send(ctx context.Context, res *Res) error
	ServerStream
	ServerStreamMetadata[Header, Trailer]
}

type GenericStreamIOMiddlewareSetter interface {
	SetStreamSendEndpoint(e StreamSendEndpoint)
	SetStreamRecvEndpoint(e StreamSendEndpoint)
}

func NewGenericClientStream[Header, Trailer, Req, Res any](cs ClientStream) *GenericClientStream[Header, Trailer, Req, Res] {
	return &GenericClientStream[Header, Trailer, Req, Res]{
		ClientStream:         cs,
		ClientStreamMetadata: cs.(ClientStreamMetadata[Header, Trailer]),
	}
}

type GenericClientStream[Header, Trailer, Req, Res any] struct {
	ClientStream
	ClientStreamMetadata[Header, Trailer]
	StreamSendMiddleware
	StreamRecvMiddleware
}

func (x *GenericClientStream[Header, Trailer, Req, Res]) SetStreamSendMiddleware(e StreamSendMiddleware) {
	x.StreamSendMiddleware = e
}

func (x *GenericClientStream[Header, Trailer, Req, Res]) SetStreamRecvMiddleware(e StreamRecvMiddleware) {
	x.StreamRecvMiddleware = e
}

func (x *GenericClientStream[Header, Trailer, Req, Res]) SendMsg(ctx context.Context, m any) error {
	if x.StreamSendMiddleware != nil {
		return x.StreamSendMiddleware(streamSendNext)(ctx, x.ClientStream, m)
	}
	return x.ClientStream.SendMsg(ctx, m)
}

func (x *GenericClientStream[Header, Trailer, Req, Res]) RecvMsg(ctx context.Context, m any) (err error) {
	if x.StreamRecvMiddleware != nil {
		err = x.StreamRecvMiddleware(streamRecvNext)(ctx, x.ClientStream, m)
	} else {
		err = x.ClientStream.RecvMsg(ctx, m)
	}
	return err
}

func (x *GenericClientStream[Header, Trailer, Req, Res]) Send(ctx context.Context, m *Req) error {
	return x.SendMsg(ctx, m)
}

func (x *GenericClientStream[Header, Trailer, Req, Res]) Recv(ctx context.Context) (m *Res, err error) {
	m = new(Res)
	if err = x.RecvMsg(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (x *GenericClientStream[Header, Trailer, Req, Res]) CloseAndRecv(ctx context.Context) (*Res, error) {
	if err := x.ClientStream.CloseSend(ctx); err != nil {
		return nil, err
	}
	return x.Recv(ctx)
}

func NewGenericServerStream[Header, Trailer, Req, Res any](ss ServerStream) *GenericServerStream[Header, Trailer, Req, Res] {
	return &GenericServerStream[Header, Trailer, Req, Res]{
		ServerStream:         ss,
		ServerStreamMetadata: ss.(ServerStreamMetadata[Header, Trailer]),
	}
}

type GenericServerStream[Header, Trailer, Req, Res any] struct {
	ServerStream
	ServerStreamMetadata[Header, Trailer]
	StreamSendMiddleware
	StreamRecvMiddleware
}

func (x *GenericServerStream[Header, Trailer, Req, Res]) SetStreamSendMiddleware(e StreamSendMiddleware) {
	x.StreamSendMiddleware = e
}

func (x *GenericServerStream[Header, Trailer, Req, Res]) SetStreamRecvMiddleware(e StreamRecvMiddleware) {
	x.StreamRecvMiddleware = e
}

func (x *GenericServerStream[Header, Trailer, Req, Res]) SendMsg(ctx context.Context, m any) error {
	if x.StreamSendMiddleware != nil {
		return x.StreamSendMiddleware(streamSendNext)(ctx, x.ServerStream, m)
	}
	return x.ServerStream.SendMsg(ctx, m)
}

func (x *GenericServerStream[Header, Trailer, Req, Res]) RecvMsg(ctx context.Context, m any) (err error) {
	if x.StreamRecvMiddleware != nil {
		err = x.StreamRecvMiddleware(streamRecvNext)(ctx, x.ServerStream, m)
	} else {
		err = x.ServerStream.RecvMsg(ctx, m)
	}
	return err
}

func (x *GenericServerStream[Header, Trailer, Req, Res]) Send(ctx context.Context, m *Res) error {
	if x.StreamSendMiddleware != nil {
		return x.StreamSendMiddleware(streamSendNext)(ctx, x.ServerStream, m)
	}
	return x.ServerStream.SendMsg(ctx, m)
}

func (x *GenericServerStream[Header, Trailer, Req, Res]) SendAndClose(ctx context.Context, m *Res) error {
	return x.Send(ctx, m)
}

func (x *GenericServerStream[Header, Trailer, Req, Res]) Recv(ctx context.Context) (m *Req, err error) {
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

type RawStreamGetter interface {
	RawStream() Stream
}

var _ ServerStream = (*serverStream)(nil)

type serverStream struct {
	ServerStream
	RawStreamGetter
}

func newServerStream(ss ServerStream) *serverStream {
	return &serverStream{ServerStream: ss}
}

func (s serverStream) RawStream() Stream {
	return s.ServerStream
}

func (s serverStream) SendMsg(ctx context.Context, m any) error {
	return s.ServerStream.SendMsg(ctx, m)
}

func (s serverStream) RecvMsg(ctx context.Context, m any) error {
	return s.ServerStream.RecvMsg(ctx, m)
}

var _ ClientStream = (*clientStream)(nil)

func NewClientStream(cs ClientStream) ClientStream {
	return &clientStream{ClientStream: cs}
}

type clientStream struct {
	ClientStream
	RawStreamGetter
}

func (s clientStream) RawStream() Stream {
	if rs, ok := s.ClientStream.(RawStreamGetter); ok {
		return rs.RawStream()
	}
	return s.ClientStream
}

func (s clientStream) SendMsg(ctx context.Context, m any) error {
	return s.ClientStream.SendMsg(ctx, m)
}

func (s clientStream) RecvMsg(ctx context.Context, m any) error {
	return s.ClientStream.RecvMsg(ctx, m)
}

func streamSendNext(ctx context.Context, stream Stream, msg any) (err error) {
	return stream.SendMsg(ctx, msg)
}

func streamRecvNext(ctx context.Context, stream Stream, msg any) (err error) {
	return stream.RecvMsg(ctx, msg)
}
