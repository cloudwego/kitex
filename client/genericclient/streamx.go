/*
 * Copyright 2025 CloudWeGo Authors
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

package genericclient

import (
	"context"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/callopt/streamcall"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type StreamXClient interface {
	ClientStreaming(ctx context.Context, method string, mecallOptions ...streamcall.Option) (StreamXClientStreamingClient, error)
	ServerStreaming(ctx context.Context, method string, req interface{}, callOptions ...streamcall.Option) (StreamXServerStreamingClient, error)
	BidirectionalStreaming(ctx context.Context, method string, callOptions ...streamcall.Option) (StreamXBidiStreamingClient, error)
	GenericCall(ctx context.Context, method string, Req interface{}, callOptions ...callopt.Option) (interface{}, error)
}

type streamXClient struct {
	g       generic.Generic
	svcInfo *serviceinfo.ServiceInfo
	gc      Client
	sc      client.Streaming
}

func NewStreamXClient(destService string, g generic.Generic, opts ...client.Option) (StreamXClient, error) {
	return NewStreamXClientWithServiceInfo(destService, g, StreamingServiceInfo(g), opts...)
}

func NewStreamXClientWithServiceInfo(destService string, g generic.Generic, serviceInfo *serviceinfo.ServiceInfo, opts ...client.Option) (StreamXClient, error) {
	var options []client.Option
	options = append(options, client.WithGeneric(g))
	options = append(options, client.WithDestService(destService))
	options = append(options, opts...)

	kc, err := client.NewClient(serviceInfo, options...)
	if err != nil {
		return nil, err
	}
	gkc, err := NewClient(destService, g, options...)
	if err != nil {
		return nil, err
	}
	return &streamXClient{
		g:       g,
		svcInfo: serviceInfo,
		gc:      gkc,
		sc:      kc.(client.Streaming),
	}, nil
}

func (c *streamXClient) ClientStreaming(ctx context.Context, method string, callOptions ...streamcall.Option) (StreamXClientStreamingClient, error) {
	ctx = client.NewCtxWithCallOptions(ctx, streamcall.GetCallOptions(callOptions))
	st, err := c.sc.StreamX(ctx, method)
	if err != nil {
		return nil, err
	}
	return newStreamXClientStreamingClient(c.svcInfo.Methods[serviceinfo.GenericClientStreamingMethod], method, st), nil
}

func (c *streamXClient) ServerStreaming(ctx context.Context, method string, Req interface{}, callOptions ...streamcall.Option) (StreamXServerStreamingClient, error) {
	ctx = client.NewCtxWithCallOptions(ctx, streamcall.GetCallOptions(callOptions))
	st, err := c.sc.StreamX(ctx, method)
	if err != nil {
		return nil, err
	}
	stream := newStreamXServerStreamingClient(c.svcInfo.Methods[serviceinfo.GenericServerStreamingMethod], method, st).(*streamXServerStreamingClient)

	args := stream.methodInfo.NewArgs().(*generic.Args)
	args.Method = stream.method
	args.Request = Req
	if err := st.SendMsg(ctx, args); err != nil {
		return nil, err
	}
	if err := stream.CloseSend(ctx); err != nil {
		return nil, err
	}
	return stream, nil
}

func (c *streamXClient) BidirectionalStreaming(ctx context.Context, methods string, callOptions ...streamcall.Option) (StreamXBidiStreamingClient, error) {
	ctx = client.NewCtxWithCallOptions(ctx, streamcall.GetCallOptions(callOptions))
	st, err := c.sc.StreamX(ctx, methods)
	if err != nil {
		return nil, err
	}
	return newStreamXBidiStreamingClient(c.svcInfo.Methods[serviceinfo.GenericBidirectionalStreamingMethod], methods, st), nil
}

func (c *streamXClient) GenericCall(ctx context.Context, method string, req interface{}, callOptions ...callopt.Option) (interface{}, error) {
	return c.gc.GenericCall(ctx, method, req, callOptions...)
}

type StreamXClientStreamingClient interface {
	Send(ctx context.Context, req interface{}) error
	CloseAndRecv(ctx context.Context) (interface{}, error)
	streaming.ClientStream
}

type streamXClientStreamingClient struct {
	methodInfo serviceinfo.MethodInfo
	method     string
	streaming.ClientStream
}

func newStreamXClientStreamingClient(methodInfo serviceinfo.MethodInfo, method string, st streaming.ClientStream) StreamXClientStreamingClient {
	return &streamXClientStreamingClient{
		methodInfo:   methodInfo,
		method:       method,
		ClientStream: st,
	}
}

func (c *streamXClientStreamingClient) Send(ctx context.Context, req interface{}) error {
	args := c.methodInfo.NewArgs().(*generic.Args)
	args.Method = c.method
	args.Request = req
	return c.ClientStream.SendMsg(ctx, args)
}

func (c *streamXClientStreamingClient) CloseAndRecv(ctx context.Context) (interface{}, error) {
	if err := c.ClientStream.CloseSend(ctx); err != nil {
		return nil, err
	}
	res := c.methodInfo.NewResult().(*generic.Result)
	if err := c.ClientStream.RecvMsg(ctx, res); err != nil {
		return nil, err
	}
	return res.GetSuccess(), nil
}

type StreamXServerStreamingClient interface {
	Recv(ctx context.Context) (interface{}, error)
	streaming.ClientStream
}

type streamXServerStreamingClient struct {
	methodInfo serviceinfo.MethodInfo
	method     string
	streaming.ClientStream
}

func newStreamXServerStreamingClient(methodInfo serviceinfo.MethodInfo, method string, st streaming.ClientStream) StreamXServerStreamingClient {
	return &streamXServerStreamingClient{
		methodInfo:   methodInfo,
		method:       method,
		ClientStream: st,
	}
}

func (c *streamXServerStreamingClient) Recv(ctx context.Context) (interface{}, error) {
	res := c.methodInfo.NewResult().(*generic.Result)
	if err := c.ClientStream.RecvMsg(ctx, res); err != nil {
		return nil, err
	}
	return res.GetSuccess(), nil
}

type StreamXBidiStreamingClient interface {
	Send(ctx context.Context, req interface{}) error
	Recv(ctx context.Context) (interface{}, error)
	streaming.ClientStream
}

type streamXBidiStreamingClient struct {
	methodInfo serviceinfo.MethodInfo
	method     string
	streaming.ClientStream
}

func newStreamXBidiStreamingClient(methodInfo serviceinfo.MethodInfo, method string, st streaming.ClientStream) StreamXBidiStreamingClient {
	return &streamXBidiStreamingClient{
		methodInfo:   methodInfo,
		method:       method,
		ClientStream: st,
	}
}

func (c *streamXBidiStreamingClient) Send(ctx context.Context, req interface{}) error {
	args := c.methodInfo.NewArgs().(*generic.Args)
	args.Method = c.method
	args.Request = req
	return c.ClientStream.SendMsg(ctx, args)
}

func (c *streamXBidiStreamingClient) Recv(ctx context.Context) (interface{}, error) {
	res := c.methodInfo.NewResult().(*generic.Result)
	if err := c.ClientStream.RecvMsg(ctx, res); err != nil {
		return nil, err
	}
	return res.GetSuccess(), nil
}
