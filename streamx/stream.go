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

type ServerStreamingClient[Res any] interface {
	Recv(ctx context.Context) (*Res, error)
	ClientStream
}

type ServerStreamingServer[Res any] interface {
	Send(ctx context.Context, res *Res) error
	ServerStream
}

type ClientStreamingClient[Req any, Res any] interface {
	Send(ctx context.Context, req *Req) error
	CloseAndRecv(ctx context.Context) (*Res, error)
	ClientStream
}

type ClientStreamingServer[Req any, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	SendAndClose(ctx context.Context, res *Res) error
	ServerStream
}

type BidiStreamingClient[Req any, Res any] interface {
	Send(ctx context.Context, req *Req) error
	Recv(ctx context.Context) (*Res, error)
	ClientStream
}

type BidiStreamingServer[Req any, Res any] interface {
	Recv(ctx context.Context) (*Req, error)
	Send(ctx context.Context, res *Res) error
	ServerStream
}

func NewGenericClientStream[Req any, Res any](cs ClientStream) *GenericClientStream[Req, Res] {
	return &GenericClientStream[Req, Res]{ClientStream: cs}
}

type GenericClientStream[Req any, Res any] struct {
	ClientStream
}

func (x *GenericClientStream[Req, Res]) Send(ctx context.Context, m *Req) error {
	return x.ClientStream.SendMsg(ctx, m)
}

func (x *GenericClientStream[Req, Res]) Recv(ctx context.Context) (*Res, error) {
	m := new(Res)
	if err := x.ClientStream.RecvMsg(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func (x *GenericClientStream[Req, Res]) CloseAndRecv(ctx context.Context) (*Res, error) {
	if err := x.ClientStream.CloseSend(ctx); err != nil {
		return nil, err
	}
	m := new(Res)
	if err := x.ClientStream.RecvMsg(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

func NewGenericServerStream[Req any, Res any](ss ServerStream) *GenericServerStream[Req, Res] {
	return &GenericServerStream[Req, Res]{ServerStream: ss}
}

type GenericServerStream[Req any, Res any] struct {
	ServerStream
}

func (x *GenericServerStream[Req, Res]) Send(ctx context.Context, m *Res) error {
	return x.ServerStream.SendMsg(ctx, m)
}

func (x *GenericServerStream[Req, Res]) SendAndClose(ctx context.Context, m *Res) error {
	return x.ServerStream.SendMsg(ctx, m)
}

func (x *GenericServerStream[Req, Res]) Recv(ctx context.Context) (*Req, error) {
	m := new(Req)
	if err := x.ServerStream.RecvMsg(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

type RawStreamGetter interface {
	RawStream() Stream
}

var _ ServerStream = (*serverStream)(nil)

type serverStream struct {
	RawStreamGetter
	ServerStream
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

func NewClientStream(ss ClientStream) ClientStream {
	return &clientStream{ClientStream: ss}
}

type clientStream struct {
	RawStreamGetter
	ClientStream
}

func (s clientStream) RawStream() Stream {
	return s.ClientStream
}

func (s clientStream) SendMsg(ctx context.Context, m any) error {
	return s.ClientStream.SendMsg(ctx, m)
}

func (s clientStream) RecvMsg(ctx context.Context, m any) error {
	return s.ClientStream.RecvMsg(ctx, m)
}
