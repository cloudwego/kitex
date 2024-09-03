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

type ClientStreamingServer[Req, Res any] streamx.ClientStreamingServer[jsonrpc.Header, jsonrpc.Trailer, Req, Res]
type ServerStreamingServer[Res any] streamx.ServerStreamingServer[jsonrpc.Header, jsonrpc.Trailer, Res]
type BidiStreamingServer[Req, Res any] streamx.BidiStreamingServer[jsonrpc.Header, jsonrpc.Trailer, Req, Res]
type ClientStreamingClient[Req, Res any] streamx.ClientStreamingClient[jsonrpc.Header, jsonrpc.Trailer, Req, Res]
type ServerStreamingClient[Res any] streamx.ServerStreamingClient[jsonrpc.Header, jsonrpc.Trailer, Res]
type BidiStreamingClient[Req, Res any] streamx.BidiStreamingClient[jsonrpc.Header, jsonrpc.Trailer, Req, Res]

var serviceInfo = &serviceinfo.ServiceInfo{
	ServiceName: "a.b.c",
	Methods: map[string]serviceinfo.MethodInfo{
		"Unary": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				// prepare args
				streamArgs := streamx.GetStreamArgsFromContext(ctx)
				if streamArgs == nil {
					return errors.New("server stream is nil")
				}
				gs := streamx.NewGenericServerStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](streamArgs.Stream())
				req, err := gs.Recv(ctx)
				if err != nil {
					return err
				}
				reqArgs.(streamx.StreamReqArgs).SetReq(req)

				// handler call
				shandler := handler.(streamx.StreamHandler)
				return shandler.Middleware(
					func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
						res, err := shandler.Handler.(ServerInterface).Unary(ctx, req)
						if err != nil {
							return err
						}
						err = gs.Send(ctx, res)
						if err != nil {
							return err
						}
						resArgs.(streamx.StreamResArgs).SetRes(res)
						return nil
					},
				)(ctx, reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs), streamArgs)
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingUnary),
		),
		"ClientStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				streamArgs := streamx.GetStreamArgsFromContext(ctx)
				if streamArgs == nil || streamArgs.Stream() == nil {
					return errors.New("server stream is nil")
				}
				gs := streamx.NewGenericServerStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](streamArgs.Stream())

				shandler := handler.(streamx.StreamHandler)
				return shandler.Middleware(
					func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
						res, err := shandler.Handler.(ServerInterface).ClientStream(ctx, gs)
						if err != nil {
							return err
						}
						err = gs.SendMsg(ctx, res)
						if err != nil {
							return err
						}
						resArgs.SetRes(res)
						return nil
					},
				)(ctx, reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs), streamArgs)
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingClient),
		),
		"ServerStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				streamArgs := streamx.GetStreamArgsFromContext(ctx)
				if streamArgs == nil || streamArgs.Stream() == nil {
					return errors.New("server stream is nil")
				}
				gs := streamx.NewGenericServerStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](streamArgs.Stream())
				req, err := gs.Recv(ctx)
				if err != nil {
					return err
				}
				reqArgs.(streamx.StreamReqArgs).SetReq(req)

				shandler := handler.(streamx.StreamHandler)
				return shandler.Middleware(
					func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
						return shandler.Handler.(ServerInterface).ServerStream(ctx, req, gs)
					},
				)(ctx, reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs), streamArgs)
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingServer),
		),
		"BidiStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				streamArgs := streamx.GetStreamArgsFromContext(ctx)
				if streamArgs == nil || streamArgs.Stream() == nil {
					return errors.New("server stream is nil")
				}
				gs := streamx.NewGenericServerStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](streamArgs.Stream())

				shandler := handler.(streamx.StreamHandler)
				return shandler.Middleware(
					func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
						return shandler.Handler.(ServerInterface).BidiStream(ctx, gs)
					},
				)(ctx, reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs), streamArgs)
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
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
	options = append(options, streamxclient.WithProvider(cp))
	cli, err := streamxclient.NewClient(serviceInfo, options...)
	if err != nil {
		return nil, err
	}
	kc := &kClient{StreamX: cli}
	return kc, nil
}

func NewServer(handler ServerInterface, opts ...streamxserver.ServerOption) (streamxserver.Server, error) {
	var options []streamxserver.ServerOption
	options = append(options, opts...)

	sp, err := jsonrpc.NewServerProvider(serviceInfo)
	if err != nil {
		return nil, err
	}
	options = append(options, streamxserver.WithProvider(sp))
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

type Response struct {
	Type    int32  `json:"Type"`
	Message string `json:"Message"`
}

type ServerInterface interface {
	Unary(ctx context.Context, req *Request) (*Response, error)
	ClientStream(ctx context.Context, stream ClientStreamingServer[Request, Response]) (*Response, error)
	ServerStream(ctx context.Context, req *Request, stream ServerStreamingServer[Response]) error
	BidiStream(ctx context.Context, stream BidiStreamingServer[Request, Response]) error
}

type ClientInterface interface {
	Unary(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (r *Response, err error)
	ClientStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (
		stream ClientStreamingClient[Request, Response], err error)
	ServerStream(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (
		stream ServerStreamingClient[Response], err error)
	BidiStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (
		stream BidiStreamingClient[Request, Response], err error)
}

var _ ClientInterface = (*kClient)(nil)

type kClient struct {
	client.StreamX
}

func (c *kClient) Unary(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (r *Response, err error) {
	var res = new(Response)
	err = c.StreamMiddleware(
		func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
			cs, err := c.StreamX.NewStream(ctx, "Unary", req, callOptions...)
			if err != nil {
				return err
			}
			gcs := streamx.NewGenericClientStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](cs)
			streamx.AsMutableStreamArgs(streamArgs).SetStream(gcs)
			if err = gcs.ClientStream.SendMsg(ctx, req); err != nil {
				return err
			}
			if err = gcs.ClientStream.RecvMsg(ctx, res); err != nil {
				return err
			}
			resArgs.SetRes(res)
			return nil
		},
	)(ctx, streamx.NewStreamReqArgs(req), streamx.NewStreamResArgs(nil), streamx.NewStreamArgs(nil))
	return res, nil
}

func (c *kClient) ClientStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (stream ClientStreamingClient[Request, Response], err error) {
	err = c.StreamMiddleware(func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
		cs, err := c.StreamX.NewStream(ctx, "ClientStream", nil, callOptions...)
		if err != nil {
			return err
		}
		streamx.AsMutableStreamArgs(streamArgs).SetStream(cs)
		stream = streamx.NewGenericClientStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](cs)
		return nil
	})(ctx, streamx.NewStreamReqArgs(nil), streamx.NewStreamResArgs(nil), streamx.NewStreamArgs(nil))
	return
}

func (c *kClient) ServerStream(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (stream ServerStreamingClient[Response], err error) {
	err = c.StreamMiddleware(
		func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
			cs, err := c.StreamX.NewStream(ctx, "ServerStream", req, callOptions...)
			if err != nil {
				return err
			}
			streamx.AsMutableStreamArgs(streamArgs).SetStream(cs)
			gcs := streamx.NewGenericClientStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](cs)
			if err = gcs.ClientStream.SendMsg(ctx, req); err != nil {
				return err
			}
			if err = gcs.ClientStream.CloseSend(ctx); err != nil {
				return err
			}
			stream = gcs
			return nil
		},
	)(ctx, streamx.NewStreamReqArgs(req), streamx.NewStreamResArgs(nil), streamx.NewStreamArgs(nil))
	return
}

func (c *kClient) BidiStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (stream BidiStreamingClient[Request, Response], err error) {
	err = c.StreamMiddleware(
		func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
			cs, err := c.StreamX.NewStream(ctx, "BidiStream", nil, callOptions...)
			if err != nil {
				return err
			}
			streamx.AsMutableStreamArgs(streamArgs).SetStream(cs)
			stream = streamx.NewGenericClientStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](cs)
			return nil
		},
	)(ctx, streamx.NewStreamReqArgs(nil), streamx.NewStreamResArgs(nil), streamx.NewStreamArgs(nil))
	return
}
