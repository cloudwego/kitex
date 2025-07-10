package generic

import (
	"context"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type ClientStreamingServer interface {
	Recv(ctx context.Context) (req interface{}, err error)
	SendAndClose(ctx context.Context, res interface{}) error
	streaming.ServerStream
}

type clientStreamingServer struct {
	methodInfo serviceinfo.MethodInfo
	streaming.ServerStream
}

func (s *clientStreamingServer) Recv(ctx context.Context) (req interface{}, err error) {
	args := s.methodInfo.NewArgs().(*Args)
	if err = s.ServerStream.RecvMsg(ctx, args); err != nil {
		return
	}
	req = args.Request
	return
}

func (s *clientStreamingServer) SendAndClose(ctx context.Context, res interface{}) error {
	results := s.methodInfo.NewResult().(*Result)
	results.Success = res
	return s.ServerStream.SendMsg(ctx, results)
}

type ServerStreamingServer interface {
	Send(ctx context.Context, res interface{}) error
	streaming.ServerStream
}

type serverStreamingServer struct {
	methodInfo serviceinfo.MethodInfo
	streaming.ServerStream
}

func (s *serverStreamingServer) Send(ctx context.Context, res interface{}) error {
	results := s.methodInfo.NewResult().(*Result)
	results.Success = res
	return s.ServerStream.SendMsg(ctx, results)
}

type BidiStreamingServer interface {
	Recv(ctx context.Context) (req interface{}, err error)
	Send(ctx context.Context, res interface{}) error
	streaming.ServerStream
}

type bidiStreamingServer struct {
	methodInfo serviceinfo.MethodInfo
	streaming.ServerStream
}

func (s *bidiStreamingServer) Recv(ctx context.Context) (req interface{}, err error) {
	args := s.methodInfo.NewArgs().(*Args)
	if err = s.ServerStream.RecvMsg(ctx, args); err != nil {
		return
	}
	req = args.Request
	return
}

func (s *bidiStreamingServer) Send(ctx context.Context, res interface{}) error {
	results := s.methodInfo.NewResult().(*Result)
	results.Success = res
	return s.ServerStream.SendMsg(ctx, results)
}
