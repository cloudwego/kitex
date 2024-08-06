package jsonrpc_test

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/server/streamxserver"
	"github.com/cloudwego/kitex/streamx"
	"github.com/cloudwego/kitex/streamx/provider/jsonrpc"
)

// === gen code ===

var serviceInfo = &serviceinfo.ServiceInfo{
	ServiceName: "a.b.c",
	Methods: map[string]serviceinfo.MethodInfo{
		"ClientStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				ss, _ := streamx.GetStreamArgsFromContext(ctx).(streamx.ServerStream)
				if ss == nil {
					return errors.New("server stream is nil")
				}
				gs := streamx.NewGenericServerStream[Request, Response](ss)
				return handler.(ServerInterface).ClientStream(ctx, gs)
			},
			func() interface{} { return new(ClientStreamArgs) },
			func() interface{} { return new(ClientStreamResult) },
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingClient),
		),
		"ServerStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				ss, _ := streamx.GetStreamArgsFromContext(ctx).(streamx.ServerStream)
				if ss == nil {
					return errors.New("server stream is nil")
				}
				gs := streamx.NewGenericServerStream[Request, Response](ss)
				req, err := gs.Recv(ctx)
				if err != nil {
					return err
				}
				return handler.(ServerInterface).ServerStream(ctx, req, gs)
			},
			func() interface{} { return new(ServerStreamArgs) },
			func() interface{} { return new(ServerStreamResult) },
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingServer),
		),
		"BidiStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				ss, _ := streamx.GetStreamArgsFromContext(ctx).(streamx.ServerStream)
				if ss == nil {
					return errors.New("server stream is nil")
				}
				gs := streamx.NewGenericServerStream[Request, Response](ss)
				return handler.(ServerInterface).BidiStream(ctx, gs)
			},
			func() interface{} { return new(BidiStreamArgs) },
			func() interface{} { return new(BidiStreamResult) },
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingServer),
		),
	},
	Extra: map[string]interface{}{"streaming": true},
}

func NewClient(destService string, opts ...streamxclient.ClientOption) (ClientInterface, error) {
	var options []streamxclient.ClientOption
	options = append(options, streamxclient.WithDestService(destService))
	options = append(options, opts...)
	cp, err := jsonrpc.NewClientProvider(serviceInfo)
	if err != nil {
		return nil, err
	}
	options = append(options, streamxclient.WithClientProvider(cp))
	client, err := streamxclient.NewClient(serviceInfo, options...)
	if err != nil {
		return nil, err
	}
	kc := &kClient{StreamNewer: client}
	return kc, nil
}

func NewServer(handler ServerInterface, opts ...streamxserver.ServerOption) (streamxserver.Server, error) {
	var options []streamxserver.ServerOption
	options = append(options, opts...)

	sp, err := jsonrpc.NewServerProvider(serviceInfo)
	if err != nil {
		return nil, err
	}
	options = append(options, streamxserver.WithServerProvider(sp))
	svr := streamxserver.NewServer(options...)
	if err := svr.RegisterService(serviceInfo, handler); err != nil {
		return nil, err
	}
	return svr, nil
}

type Request struct {
	Type    int32  `json:"Type"`
	Message string `json:"Message"`
}
type Response = Request

type ClientStreamArgs struct {
	Req *Request
}

type ClientStreamResult struct {
	Success *Response
}

type ServerStreamArgs = ClientStreamArgs
type ServerStreamResult = ClientStreamResult
type BidiStreamArgs = ClientStreamArgs
type BidiStreamResult = ClientStreamResult

type ServerInterface interface {
	Unary(ctx context.Context, req *Request) (*Response, error)
	ClientStream(ctx context.Context, stream streamx.ClientStreamingServer[Request, Response]) error
	ServerStream(ctx context.Context, req *Request, stream streamx.ServerStreamingServer[Response]) error
	BidiStream(ctx context.Context, stream streamx.BidiStreamingServer[Request, Response]) error
}

type ClientInterface interface {
	Unary(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (r *Response, err error)
	ClientStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (
		stream streamx.ClientStreamingClient[Request, Response], err error)
	ServerStream(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (
		stream streamx.ServerStreamingClient[Response], err error)
	BidiStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (
		stream streamx.BidiStreamingClient[Request, Response], err error)
}

var _ ClientInterface = (*kClient)(nil)

type kClient struct {
	client.StreamNewer
}

func (c *kClient) Unary(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (r *Response, err error) {
	return nil, nil
}

func (c *kClient) ClientStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (stream streamx.ClientStreamingClient[Request, Response], err error) {
	cs, err := c.StreamNewer.NewStream(ctx, "ClientStream", nil, nil, callOptions...)
	if err != nil {
		return nil, err
	}
	gcs := streamx.NewGenericClientStream[Request, Response](cs)
	return gcs, nil
}

func (c *kClient) ServerStream(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (stream streamx.ServerStreamingClient[Response], err error) {
	cs, err := c.StreamNewer.NewStream(ctx, "ServerStream", req, nil, callOptions...)
	if err != nil {
		return nil, err
	}
	x := streamx.NewGenericClientStream[Request, Response](cs)
	if err = x.ClientStream.SendMsg(ctx, req); err != nil {
		return nil, err
	}
	if err = x.ClientStream.CloseSend(ctx); err != nil {
		return nil, err
	}
	return x, nil
}

func (c *kClient) BidiStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (stream streamx.BidiStreamingClient[Request, Response], err error) {
	cs, err := c.StreamNewer.NewStream(ctx, "BidiStream", nil, nil, callOptions...)
	if err != nil {
		return nil, err
	}
	x := streamx.NewGenericClientStream[Request, Response](cs)
	return x, nil
}
