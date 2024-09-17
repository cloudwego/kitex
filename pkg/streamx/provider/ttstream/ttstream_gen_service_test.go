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

package ttstream_test

import (
	"context"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/streamxserver"
)

// === gen code ===

// --- Define Header and Trailer type ---
type ClientStreamingServer[Req, Res any] streamx.ClientStreamingServer[ttstream.Header, ttstream.Trailer, Req, Res]
type ServerStreamingServer[Res any] streamx.ServerStreamingServer[ttstream.Header, ttstream.Trailer, Res]
type BidiStreamingServer[Req, Res any] streamx.BidiStreamingServer[ttstream.Header, ttstream.Trailer, Req, Res]

type ClientStreamingClient[Req, Res any] streamx.ClientStreamingClient[ttstream.Header, ttstream.Trailer, Req, Res]
type ServerStreamingClient[Res any] streamx.ServerStreamingClient[ttstream.Header, ttstream.Trailer, Res]
type BidiStreamingClient[Req, Res any] streamx.BidiStreamingClient[ttstream.Header, ttstream.Trailer, Req, Res]

// --- Define Service Method handler ---
var pingpongServiceInfo = &serviceinfo.ServiceInfo{
	ServiceName:  "kitex.service.pingpong",
	PayloadCodec: serviceinfo.Thrift,
	HandlerType:  (*PingPongServerInterface)(nil),
	Methods: map[string]serviceinfo.MethodInfo{
		"PingPong": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				realArg := reqArgs.(*ServerPingPongArgs)
				realResult := resArgs.(*ServerPingPongResult)
				success, err := handler.(PingPongServerInterface).PingPong(ctx, realArg.Req)
				if err != nil {
					return err
				}
				realResult.Success = success
				return nil
			},
			func() interface{} { return NewServerPingPongArgs() },
			func() interface{} { return NewServerPingPongResult() },
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingNone),
		),
	},
	Extra: map[string]interface{}{"streaming": false},
}

var streamingServiceInfo = &serviceinfo.ServiceInfo{
	ServiceName: "kitex.service.streaming",
	Methods: map[string]serviceinfo.MethodInfo{
		"Unary": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return streamxserver.InvokeStream[ttstream.Header, ttstream.Trailer, Request, Response](
					ctx, serviceinfo.StreamingUnary, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingUnary),
		),
		"ClientStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return streamxserver.InvokeStream[ttstream.Header, ttstream.Trailer, Request, Response](
					ctx, serviceinfo.StreamingClient, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingClient),
		),
		"ServerStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return streamxserver.InvokeStream[ttstream.Header, ttstream.Trailer, Request, Response](
					ctx, serviceinfo.StreamingServer, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingServer),
		),
		"BidiStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return streamxserver.InvokeStream[ttstream.Header, ttstream.Trailer, Request, Response](
					ctx, serviceinfo.StreamingBidirectional, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
		),
	},
	Extra: map[string]interface{}{"streaming": true, "streamx": true},
}

// --- Define RegisterService  interface ---
func RegisterService(svr server.Server, handler StreamingServerInterface, opts ...server.RegisterOption) error {
	return svr.RegisterService(streamingServiceInfo, handler, opts...)
}

// --- Define New Client interface ---
func NewPingPongClient(destService string, opts ...client.Option) (PingPongClientInterface, error) {
	var options []client.Option
	options = append(options, client.WithDestService(destService))
	options = append(options, opts...)
	cli, err := client.NewClient(pingpongServiceInfo, options...)
	if err != nil {
		return nil, err
	}
	kc := &kClient{caller: cli}
	return kc, nil
}

func NewStreamingClient(destService string, opts ...streamxclient.Option) (StreamingClientInterface, error) {
	var options []streamxclient.Option
	options = append(options, streamxclient.WithDestService(destService))
	options = append(options, opts...)
	cp, err := ttstream.NewClientProvider(streamingServiceInfo)
	if err != nil {
		return nil, err
	}
	options = append(options, streamxclient.WithProvider(cp))
	cli, err := streamxclient.NewClient(streamingServiceInfo, options...)
	if err != nil {
		return nil, err
	}
	kc := &kClient{streamer: cli, caller: cli.(client.Client)}
	return kc, nil
}

// --- Define Server Implementation Interface ---
type PingPongServerInterface interface {
	PingPong(ctx context.Context, req *Request) (*Response, error)
}
type StreamingServerInterface interface {
	Unary(ctx context.Context, req *Request) (*Response, error)
	ClientStream(ctx context.Context, stream ClientStreamingServer[Request, Response]) (*Response, error)
	ServerStream(ctx context.Context, req *Request, stream ServerStreamingServer[Response]) error
	BidiStream(ctx context.Context, stream BidiStreamingServer[Request, Response]) error
}

// --- Define Client Implementation Interface ---
type PingPongClientInterface interface {
	PingPong(ctx context.Context, req *Request) (r *Response, err error)
}

type StreamingClientInterface interface {
	Unary(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (r *Response, err error)
	ClientStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (
		stream ClientStreamingClient[Request, Response], err error)
	ServerStream(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (
		stream ServerStreamingClient[Response], err error)
	BidiStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (
		stream BidiStreamingClient[Request, Response], err error)
}

// --- Define Client Implementation ---
var _ StreamingClientInterface = (*kClient)(nil)
var _ PingPongClientInterface = (*kClient)(nil)

type kClient struct {
	caller   client.Client
	streamer streamxclient.Client
}

func (c *kClient) PingPong(ctx context.Context, req *Request) (r *Response, err error) {
	var _args ServerPingPongArgs
	_args.Req = req
	var _result ServerPingPongResult
	if err = c.caller.Call(ctx, "PingPong", &_args, &_result); err != nil {
		return
	}
	return _result.GetSuccess(), nil
}

func (c *kClient) Unary(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (*Response, error) {
	res := new(Response)
	_, err := streamxclient.InvokeStream[ttstream.Header, ttstream.Trailer, Request, Response](
		ctx, c.streamer, serviceinfo.StreamingUnary, "Unary", req, res, callOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *kClient) ClientStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (stream ClientStreamingClient[Request, Response], err error) {
	return streamxclient.InvokeStream[ttstream.Header, ttstream.Trailer, Request, Response](
		ctx, c.streamer, serviceinfo.StreamingClient, "ClientStream", nil, nil, callOptions...)
}

func (c *kClient) ServerStream(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (
	stream ServerStreamingClient[Response], err error) {
	return streamxclient.InvokeStream[ttstream.Header, ttstream.Trailer, Request, Response](
		ctx, c.streamer, serviceinfo.StreamingServer, "ServerStream", req, nil, callOptions...)
}

func (c *kClient) BidiStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (
	stream BidiStreamingClient[Request, Response], err error) {
	return streamxclient.InvokeStream[ttstream.Header, ttstream.Trailer, Request, Response](
		ctx, c.streamer, serviceinfo.StreamingBidirectional, "BidiStream", nil, nil, callOptions...)
}
