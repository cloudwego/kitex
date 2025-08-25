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

// NOTE: The basic generic streaming functions have been completed, but the interface needs adjustments. The feature is expected to be released later.

package genericclient

import (
	"context"
	"errors"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// Deprecated, use generic.ServiceInfoWithGeneric instead
func StreamingServiceInfo(g generic.Generic) *serviceinfo.ServiceInfo {
	return generic.ServiceInfoWithGeneric(g)
}

type ClientStreaming interface {
	streaming.Stream
	Send(req interface{}) error
	CloseAndRecv() (resp interface{}, err error)
}

type ServerStreaming interface {
	streaming.Stream
	Recv() (resp interface{}, err error)
}

type BidirectionalStreaming interface {
	streaming.Stream
	Send(req interface{}) error
	Recv() (resp interface{}, err error)
}

// Deprecated: use NewClient instead.
func NewStreamingClient(destService string, g generic.Generic, opts ...client.Option) (Client, error) {
	return NewClient(destService, g, opts...)
}

// Deprecated: use NewClientWithServiceInfo instead.
func NewStreamingClientWithServiceInfo(destService string, g generic.Generic, svcInfo *serviceinfo.ServiceInfo, opts ...client.Option) (Client, error) {
	return NewClientWithServiceInfo(destService, g, svcInfo, opts...)
}

type deprecatedClientStreamingClient struct {
	streaming.Stream
	method     string
	methodInfo serviceinfo.MethodInfo
}

func NewClientStreaming(ctx context.Context, genericCli Client, method string, callOpts ...callopt.Option) (ClientStreaming, error) {
	gCli, ok := genericCli.(*genericServiceClient)
	if !ok {
		return nil, errors.New("invalid generic client")
	}
	if gCli.isBinaryGeneric {
		// To be compatible with binary generic calls, streaming mode must be passed in.
		ctx = igeneric.WithGenericStreamingMode(ctx, serviceinfo.StreamingClient)
	}
	stream, err := getStream(ctx, gCli, method, callOpts...)
	if err != nil {
		return nil, err
	}
	ri := rpcinfo.GetRPCInfo(stream.Context())
	return &deprecatedClientStreamingClient{stream, method, ri.Invocation().MethodInfo()}, nil
}

func (cs *deprecatedClientStreamingClient) Send(req interface{}) error {
	_args := cs.methodInfo.NewArgs().(*generic.Args)
	_args.Method = cs.method
	_args.Request = req
	return cs.Stream.SendMsg(_args)
}

func (cs *deprecatedClientStreamingClient) CloseAndRecv() (resp interface{}, err error) {
	if err := cs.Stream.Close(); err != nil {
		return nil, err
	}
	_result := cs.methodInfo.NewResult().(*generic.Result)
	if err = cs.Stream.RecvMsg(_result); err != nil {
		return nil, err
	}
	return _result.GetSuccess(), nil
}

type deprecatedServerStreamingClient struct {
	streaming.Stream
	methodInfo serviceinfo.MethodInfo
}

func NewServerStreaming(ctx context.Context, genericCli Client, method string, req interface{}, callOpts ...callopt.Option) (ServerStreaming, error) {
	gCli, ok := genericCli.(*genericServiceClient)
	if !ok {
		return nil, errors.New("invalid generic client")
	}
	if gCli.isBinaryGeneric {
		// To be compatible with binary generic calls, streaming mode must be passed in.
		ctx = igeneric.WithGenericStreamingMode(ctx, serviceinfo.StreamingServer)
	}
	stream, err := getStream(ctx, gCli, method, callOpts...)
	if err != nil {
		return nil, err
	}
	ri := rpcinfo.GetRPCInfo(stream.Context())
	mtInfo := ri.Invocation().MethodInfo()
	ss := &deprecatedServerStreamingClient{stream, mtInfo}
	_args := mtInfo.NewArgs().(*generic.Args)
	_args.Method = method
	_args.Request = req
	if err = ss.Stream.SendMsg(_args); err != nil {
		return nil, err
	}
	if err = ss.Stream.Close(); err != nil {
		return nil, err
	}
	return ss, nil
}

func (ss *deprecatedServerStreamingClient) Recv() (resp interface{}, err error) {
	_result := ss.methodInfo.NewResult().(*generic.Result)
	if err = ss.Stream.RecvMsg(_result); err != nil {
		return nil, err
	}
	return _result.GetSuccess(), nil
}

type deprecatedBidirectionalStreamingClient struct {
	streaming.Stream
	method     string
	methodInfo serviceinfo.MethodInfo
}

func NewBidirectionalStreaming(ctx context.Context, genericCli Client, method string, callOpts ...callopt.Option) (BidirectionalStreaming, error) {
	gCli, ok := genericCli.(*genericServiceClient)
	if !ok {
		return nil, errors.New("invalid generic client")
	}
	if gCli.isBinaryGeneric {
		// To be compatible with binary generic calls, streaming mode must be passed in.
		ctx = igeneric.WithGenericStreamingMode(ctx, serviceinfo.StreamingBidirectional)
	}
	stream, err := getStream(ctx, gCli, method, callOpts...)
	if err != nil {
		return nil, err
	}
	ri := rpcinfo.GetRPCInfo(stream.Context())
	return &deprecatedBidirectionalStreamingClient{stream, method, ri.Invocation().MethodInfo()}, nil
}

func (bs *deprecatedBidirectionalStreamingClient) Send(req interface{}) error {
	_args := bs.methodInfo.NewArgs().(*generic.Args)
	_args.Method = bs.method
	_args.Request = req
	return bs.Stream.SendMsg(_args)
}

func (bs *deprecatedBidirectionalStreamingClient) Recv() (resp interface{}, err error) {
	_result := bs.methodInfo.NewResult().(*generic.Result)
	if err = bs.Stream.RecvMsg(_result); err != nil {
		return nil, err
	}
	return _result.GetSuccess(), nil
}

func getStream(ctx context.Context, genericCli *genericServiceClient, method string, callOpts ...callopt.Option) (streaming.Stream, error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOpts)
	res := new(streaming.Result)
	err := genericCli.sClient.Stream(ctx, method, nil, res)
	if err != nil {
		return nil, err
	}
	return res.Stream, nil
}
