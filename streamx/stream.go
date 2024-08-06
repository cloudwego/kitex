package streamx

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var _ ServerStreamingClient[int] = (*GenericClientStream[int, int])(nil)
var _ ClientStreamingClient[int, int] = (*GenericClientStream[int, int])(nil)
var _ BidiStreamingClient[int, int] = (*GenericClientStream[int, int])(nil)
var _ ServerStreamingServer[int] = (*GenericServerStream[int, int])(nil)
var _ ClientStreamingServer[int, int] = (*GenericServerStream[int, int])(nil)
var _ BidiStreamingServer[int, int] = (*GenericServerStream[int, int])(nil)

type StreamCtxKey struct{}

func WithStreamArgsContext(ctx context.Context, args StreamArgs) context.Context {
	ctx = context.WithValue(ctx, StreamCtxKey{}, args)
	return ctx
}

func GetStreamArgsFromContext(ctx context.Context) (args StreamArgs) {
	val := ctx.Value(StreamCtxKey{})
	if val == nil {
		return nil
	}
	args, _ = val.(StreamArgs)
	return args
}

type StreamArgs interface {
	Stream() RawStream
}

func AsStream(args interface{}) (RawStream, error) {
	s, ok := args.(StreamArgs)
	if !ok {
		return nil, errors.New("asStream expects StreamArgs")
	}
	return s.Stream(), nil
}

type MutableStreamArgs interface {
	SetStream(st RawStream)
}

func AsMutableStreamArgs(args StreamArgs) MutableStreamArgs {
	margs, ok := args.(MutableStreamArgs)
	if !ok {
		return nil
	}
	return margs
}

type streamArgs struct {
	stream RawStream
}

func (s *streamArgs) Stream() RawStream {
	return s.stream
}

func (s *streamArgs) SetStream(st RawStream) {
	s.stream = st
}

func NewStreamArgs(stream RawStream) StreamArgs {
	return &streamArgs{stream: stream}
}

type StreamingMode = serviceinfo.StreamingMode

type RawStream interface {
	Mode() StreamingMode
	Method() string
	SendMsg(ctx context.Context, m any) error
	RecvMsg(ctx context.Context, m any) error
}

type ClientStream interface {
	RawStream
	CloseSend(ctx context.Context) error
}

type ServerStream interface {
	RawStream
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
