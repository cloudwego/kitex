/*
 * Copyright 2021 CloudWeGo Authors
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

// Package genericclient ...
package genericclient

import (
	"context"
	"runtime"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/callopt/streamcall"
	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/transport"
)

var _ Client = &genericServiceClient{}

// NewClient create a generic client
func NewClient(destService string, g generic.Generic, opts ...client.Option) (Client, error) {
	svcInfo := generic.ServiceInfoWithGeneric(g)
	return NewClientWithServiceInfo(destService, g, svcInfo, opts...)
}

// NewClientWithServiceInfo create a generic client with serviceInfo
func NewClientWithServiceInfo(destService string, g generic.Generic, svcInfo *serviceinfo.ServiceInfo, opts ...client.Option) (Client, error) {
	var options []client.Option
	options = append(options, client.WithGeneric(g))
	options = append(options, client.WithDestService(destService))
	options = append(options, client.WithTransportProtocol(transport.TTHeaderStreaming))
	options = append(options, opts...)

	kc, err := client.NewClient(svcInfo, options...)
	if err != nil {
		return nil, err
	}
	cli := &genericServiceClient{
		svcInfo: svcInfo,
		kClient: kc,
		sClient: kc.(client.Streaming),
		g:       g,
	}
	cli.isBinaryGeneric, _ = g.GetExtra(igeneric.IsBinaryGeneric).(bool)
	cli.getMethodFunc, _ = g.GetExtra(igeneric.GetMethodNameByRequestFuncKey).(generic.GetMethodNameByRequestFunc)

	runtime.SetFinalizer(cli, (*genericServiceClient).Close)
	return cli, nil
}

// Client generic client
type Client interface {
	generic.Closer

	// GenericCall generic call
	GenericCall(ctx context.Context, method string, request interface{}, callOptions ...callopt.Option) (response interface{}, err error)
	// ClientStreaming creates an implementation of ClientStreamingClient
	ClientStreaming(ctx context.Context, method string, callOptions ...streamcall.Option) (ClientStreamingClient, error)
	// ServerStreaming creates an implementation of ServerStreamingClient
	ServerStreaming(ctx context.Context, method string, req interface{}, callOptions ...streamcall.Option) (ServerStreamingClient, error)
	// BidirectionalStreaming creates an implementation of BidiStreamingClient
	BidirectionalStreaming(ctx context.Context, method string, callOptions ...streamcall.Option) (BidiStreamingClient, error)
}

type genericServiceClient struct {
	svcInfo *serviceinfo.ServiceInfo
	kClient client.Client
	sClient client.Streaming
	g       generic.Generic
	// binary generic need method info context
	isBinaryGeneric bool
	// http generic get method name by request
	getMethodFunc generic.GetMethodNameByRequestFunc
}

func (gc *genericServiceClient) GenericCall(ctx context.Context, method string, request interface{}, callOptions ...callopt.Option) (response interface{}, err error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOptions)
	if gc.isBinaryGeneric {
		// To be compatible with binary generic calls, streaming mode must be passed in.
		ctx = igeneric.WithGenericStreamingMode(ctx, serviceinfo.StreamingNone)
	}
	if gc.getMethodFunc != nil {
		// get method name from http generic by request
		method, _ = gc.getMethodFunc(request)
	}
	var _args *generic.Args
	var _result *generic.Result
	mtInfo := gc.svcInfo.MethodInfo(ctx, method)
	if mtInfo != nil {
		_args = mtInfo.NewArgs().(*generic.Args)
		_args.Method = method
		_args.Request = request
		_result = mtInfo.NewResult().(*generic.Result)
	} else {
		// To correctly report the unknown method error, an empty object is returned here.
		_args = &generic.Args{}
		_result = &generic.Result{}
	}

	if err = gc.kClient.Call(ctx, method, _args, _result); err != nil {
		return
	}
	return _result.GetSuccess(), nil
}

func (gc *genericServiceClient) Close() error {
	// no need a finalizer anymore
	runtime.SetFinalizer(gc, nil)

	// Notice: don't need to close kClient because finalizer will close it.
	return gc.g.Close()
}

func (gc *genericServiceClient) ClientStreaming(ctx context.Context, method string, callOptions ...streamcall.Option) (ClientStreamingClient, error) {
	ctx = client.NewCtxWithCallOptions(ctx, streamcall.GetCallOptions(callOptions))
	if gc.isBinaryGeneric {
		// To be compatible with binary generic calls, streaming mode must be passed in.
		ctx = igeneric.WithGenericStreamingMode(ctx, serviceinfo.StreamingClient)
	}
	st, err := gc.sClient.StreamX(ctx, method)
	if err != nil {
		return nil, err
	}
	ri := rpcinfo.GetRPCInfo(st.Context())
	return newClientStreamingClient(ri.Invocation().MethodInfo(), method, st), nil
}

func (gc *genericServiceClient) ServerStreaming(ctx context.Context, method string, req interface{}, callOptions ...streamcall.Option) (ServerStreamingClient, error) {
	ctx = client.NewCtxWithCallOptions(ctx, streamcall.GetCallOptions(callOptions))
	if gc.isBinaryGeneric {
		// To be compatible with binary generic calls, streaming mode must be passed in.
		ctx = igeneric.WithGenericStreamingMode(ctx, serviceinfo.StreamingServer)
	}
	st, err := gc.sClient.StreamX(ctx, method)
	if err != nil {
		return nil, err
	}
	ri := rpcinfo.GetRPCInfo(st.Context())
	stream := newServerStreamingClient(ri.Invocation().MethodInfo(), method, st).(*serverStreamingClient)

	args := stream.methodInfo.NewArgs().(*generic.Args)
	args.Method = stream.method
	args.Request = req
	if err := st.SendMsg(st.Context(), args); err != nil {
		return nil, err
	}
	if err := stream.Streaming().CloseSend(st.Context()); err != nil {
		return nil, err
	}
	return stream, nil
}

func (gc *genericServiceClient) BidirectionalStreaming(ctx context.Context, method string, callOptions ...streamcall.Option) (BidiStreamingClient, error) {
	ctx = client.NewCtxWithCallOptions(ctx, streamcall.GetCallOptions(callOptions))
	if gc.isBinaryGeneric {
		// To be compatible with binary generic calls, streaming mode must be passed in.
		ctx = igeneric.WithGenericStreamingMode(ctx, serviceinfo.StreamingBidirectional)
	}
	st, err := gc.sClient.StreamX(ctx, method)
	if err != nil {
		return nil, err
	}
	ri := rpcinfo.GetRPCInfo(st.Context())
	return newBidiStreamingClient(ri.Invocation().MethodInfo(), method, st), nil
}

// ClientStreamingClient define client side generic client streaming APIs
type ClientStreamingClient interface {
	// Send sends a message to the server.
	Send(ctx context.Context, req interface{}) error
	// CloseAndRecv closes the send direction of the stream and returns the response from the server.
	CloseAndRecv(ctx context.Context) (interface{}, error)
	// Header inherits from the underlying streaming.ClientStream.
	Header() (streaming.Header, error)
	// Trailer inherits from the underlying streaming.ClientStream.
	Trailer() (streaming.Trailer, error)
	// CloseSend inherits from the underlying streaming.ClientStream.
	CloseSend(ctx context.Context) error
	// Context inherits from the underlying streaming.ClientStream.
	Context() context.Context
	// Streaming returns the underlying streaming.ClientStream.
	Streaming() streaming.ClientStream
}

type clientStreamingClient struct {
	methodInfo serviceinfo.MethodInfo
	method     string
	streaming  streaming.ClientStream
}

func newClientStreamingClient(methodInfo serviceinfo.MethodInfo, method string, st streaming.ClientStream) ClientStreamingClient {
	return &clientStreamingClient{
		methodInfo: methodInfo,
		method:     method,
		streaming:  st,
	}
}

func (c *clientStreamingClient) Send(ctx context.Context, req interface{}) error {
	args := c.methodInfo.NewArgs().(*generic.Args)
	args.Method = c.method
	args.Request = req
	return c.streaming.SendMsg(ctx, args)
}

func (c *clientStreamingClient) CloseAndRecv(ctx context.Context) (interface{}, error) {
	if err := c.streaming.CloseSend(ctx); err != nil {
		return nil, err
	}
	res := c.methodInfo.NewResult().(*generic.Result)
	if err := c.streaming.RecvMsg(ctx, res); err != nil {
		return nil, err
	}
	return res.GetSuccess(), nil
}

func (c *clientStreamingClient) Header() (streaming.Header, error) {
	return c.streaming.Header()
}

func (c *clientStreamingClient) Trailer() (streaming.Trailer, error) {
	return c.streaming.Trailer()
}

func (c *clientStreamingClient) CloseSend(ctx context.Context) error {
	return c.streaming.CloseSend(ctx)
}

func (c *clientStreamingClient) Context() context.Context {
	return c.streaming.Context()
}

func (c *clientStreamingClient) Streaming() streaming.ClientStream {
	return c.streaming
}

// ServerStreamingClient define client side generic server streaming APIs
type ServerStreamingClient interface {
	// Recv receives a message from the server.
	Recv(ctx context.Context) (interface{}, error)
	// Header inherits from the underlying streaming.ClientStream.
	Header() (streaming.Header, error)
	// Trailer inherits from the underlying streaming.ClientStream.
	Trailer() (streaming.Trailer, error)
	// CloseSend inherits from the underlying streaming.ClientStream.
	CloseSend(ctx context.Context) error
	// Context inherits from the underlying streaming.ClientStream.
	Context() context.Context
	// Streaming returns the underlying streaming.ClientStream.
	Streaming() streaming.ClientStream
}

type serverStreamingClient struct {
	methodInfo serviceinfo.MethodInfo
	method     string
	streaming  streaming.ClientStream
}

func newServerStreamingClient(methodInfo serviceinfo.MethodInfo, method string, st streaming.ClientStream) ServerStreamingClient {
	return &serverStreamingClient{
		methodInfo: methodInfo,
		method:     method,
		streaming:  st,
	}
}

func (c *serverStreamingClient) Recv(ctx context.Context) (interface{}, error) {
	res := c.methodInfo.NewResult().(*generic.Result)
	if err := c.streaming.RecvMsg(ctx, res); err != nil {
		return nil, err
	}
	return res.GetSuccess(), nil
}

func (c *serverStreamingClient) Header() (streaming.Header, error) {
	return c.streaming.Header()
}

func (c *serverStreamingClient) Trailer() (streaming.Trailer, error) {
	return c.streaming.Trailer()
}

func (c *serverStreamingClient) CloseSend(ctx context.Context) error {
	return c.streaming.CloseSend(ctx)
}

func (c *serverStreamingClient) Context() context.Context {
	return c.streaming.Context()
}

func (c *serverStreamingClient) Streaming() streaming.ClientStream {
	return c.streaming
}

// BidiStreamingClient define client side generic bidirectional streaming APIs
type BidiStreamingClient interface {
	// Send sends a message to the server.
	Send(ctx context.Context, req interface{}) error
	// Recv receives a message from the server.
	Recv(ctx context.Context) (interface{}, error)
	// Header inherits from the underlying streaming.ClientStream.
	Header() (streaming.Header, error)
	// Trailer inherits from the underlying streaming.ClientStream.
	Trailer() (streaming.Trailer, error)
	// CloseSend inherits from the underlying streaming.ClientStream.
	CloseSend(ctx context.Context) error
	// Context inherits from the underlying streaming.ClientStream.
	Context() context.Context
	// Streaming returns the underlying streaming.ClientStream.
	Streaming() streaming.ClientStream
}

type bidiStreamingClient struct {
	methodInfo serviceinfo.MethodInfo
	method     string
	streaming  streaming.ClientStream
}

func newBidiStreamingClient(methodInfo serviceinfo.MethodInfo, method string, st streaming.ClientStream) BidiStreamingClient {
	return &bidiStreamingClient{
		methodInfo: methodInfo,
		method:     method,
		streaming:  st,
	}
}

func (c *bidiStreamingClient) Send(ctx context.Context, req interface{}) error {
	args := c.methodInfo.NewArgs().(*generic.Args)
	args.Method = c.method
	args.Request = req
	return c.streaming.SendMsg(ctx, args)
}

func (c *bidiStreamingClient) Recv(ctx context.Context) (interface{}, error) {
	res := c.methodInfo.NewResult().(*generic.Result)
	if err := c.streaming.RecvMsg(ctx, res); err != nil {
		return nil, err
	}
	return res.GetSuccess(), nil
}

func (c *bidiStreamingClient) Header() (streaming.Header, error) {
	return c.streaming.Header()
}

func (c *bidiStreamingClient) Trailer() (streaming.Trailer, error) {
	return c.streaming.Trailer()
}

func (c *bidiStreamingClient) CloseSend(ctx context.Context) error {
	return c.streaming.CloseSend(ctx)
}

func (c *bidiStreamingClient) Context() context.Context {
	return c.streaming.Context()
}

func (c *bidiStreamingClient) Streaming() streaming.ClientStream {
	return c.streaming
}
