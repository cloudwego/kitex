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

package jsonrpc_test

import (
	"context"

	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/jsonrpc"
	"github.com/cloudwego/kitex/server/streamxserver"
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
				return streamxserver.InvokeStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](
					ctx, serviceinfo.StreamingUnary, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingUnary),
		),
		"ClientStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return streamxserver.InvokeStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](
					ctx, serviceinfo.StreamingClient, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingClient),
		),
		"ServerStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return streamxserver.InvokeStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](
					ctx, serviceinfo.StreamingServer, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingServer),
		),
		"BidiStream": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return streamxserver.InvokeStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](
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

func NewClient(destService string, opts ...streamxclient.Option) (ClientInterface, error) {
	var options []streamxclient.Option
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
	kc := &kClient{Client: cli}
	return kc, nil
}

func NewServer(handler ServerInterface, opts ...streamxserver.Option) (streamxserver.Server, error) {
	var options []streamxserver.Option
	options = append(options, opts...)
	sp, err := jsonrpc.NewServerProvider(serviceInfo)
	if err != nil {
		return nil, err
	}
	svr := streamxserver.NewServer(options...)
	if err := svr.RegisterService(serviceInfo, handler, streamxserver.WithProvider(sp)); err != nil {
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

// --- Define Client Implementation ---

var _ ClientInterface = (*kClient)(nil)

type kClient struct {
	streamxclient.Client
}

func (c *kClient) Unary(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (*Response, error) {
	res := new(Response)
	_, err := streamxclient.InvokeStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](
		ctx, c.Client, serviceinfo.StreamingUnary, "Unary", req, res, callOptions...)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *kClient) ClientStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (stream ClientStreamingClient[Request, Response], err error) {
	return streamxclient.InvokeStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](
		ctx, c.Client, serviceinfo.StreamingClient, "ClientStream", nil, nil, callOptions...)
}

func (c *kClient) ServerStream(ctx context.Context, req *Request, callOptions ...streamxcallopt.CallOption) (
	stream ServerStreamingClient[Response], err error) {
	return streamxclient.InvokeStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](
		ctx, c.Client, serviceinfo.StreamingServer, "ServerStream", req, nil, callOptions...)
}

func (c *kClient) BidiStream(ctx context.Context, callOptions ...streamxcallopt.CallOption) (
	stream BidiStreamingClient[Request, Response], err error) {
	return streamxclient.InvokeStream[jsonrpc.Header, jsonrpc.Trailer, Request, Response](
		ctx, c.Client, serviceinfo.StreamingBidirectional, "BidiStream", nil, nil, callOptions...)
}
