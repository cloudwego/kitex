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

package genericclient

import (
	"context"
	"errors"
	"runtime"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/client/streamxclient/streamxcallopt"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
)

type genericServiceStreamingClient[T any] struct {
	svcInfo *serviceinfo.ServiceInfo
	kClient client.Client
	g       generic.Generic
}

// StreamClient generic streaming client
type StreamClient[T any] interface {
	generic.Closer

	// GenericCall generic call
	GenericCall(ctx context.Context, method string, request any, callOptions ...T) (response any, err error)
}

// WIP: NOTE: want to unify the user inteface for both uanry and ping pong calls
func (sc *genericServiceStreamingClient[T]) GenericCall(ctx context.Context, method string, request any, callOptions ...T) (response any, err error) {
	mtInfo := sc.svcInfo.MethodInfo(method)
	_args := mtInfo.NewArgs().(*generic.Args)
	_args.Method = method
	_args.Request = request

	_result := mtInfo.NewResult().(*generic.Result)

	// ping pong call over TTHeader
	if _, ok := sc.svcInfo.Extra["ttheader_streaming"]; ok {
		if callOptions != nil {
			if opts, ok := any(callOptions).([]callopt.Option); ok {
				ctx = client.NewCtxWithCallOptions(ctx, opts)
			} else {
				return nil, errors.New("unsupported call option type. it should be []callopt.Option")
			}
		}

		mt, err := sc.g.GetMethod(request, method)
		if err != nil {
			return nil, err
		}
		if mt.Oneway {
			return nil, sc.kClient.Call(ctx, mt.Name, _args, nil)
		}

		if err = sc.kClient.Call(ctx, mt.Name, _args, _result); err != nil {
			return nil, err
		}
		return _result.GetSuccess(), nil
	}

	// NOTE: not supported yet
	// unary call over gRPC
	if _, ok := sc.svcInfo.Extra["grpc_streaming"]; ok {
		var opts []streamxcallopt.CallOption
		if callOptions != nil {
			if opts, ok = any(callOptions).([]streamxcallopt.CallOption); !ok {
				return nil, errors.New("unsupported call option type. it should be []streamxcallopt.CallOption")
			}
		}
		_, _, err = streamxclient.InvokeStream[generic.Args, generic.Result](
			ctx, sc.kClient, serviceinfo.StreamingUnary, method, _args, _result, opts...)
		if err != nil {
			return nil, err
		}
		return _result.GetSuccess(), nil
	}

	return nil, errors.New("unsupported transport protocol. please specify the client provider")
}

func (sc *genericServiceStreamingClient[T]) Close() error {
	// no need a finalizer anymore
	runtime.SetFinalizer(sc, nil)

	// Notice: don't need to close kClient because finalizer will close it.
	return sc.g.Close()
}

type V2ClientStreamingClient[Req, Res any] interface {
	Send(ctx context.Context, req Req) error
	CloseAndRecv(ctx context.Context) (Res, error)
	streamx.ClientStream
}

type V2ServerStreamingClient[Req, Res any] interface {
	Recv(ctx context.Context) (Res, error)
	streamx.ClientStream
}

type V2BidirectionalStreamingClient[Req, Res any] interface {
	Send(ctx context.Context, req Req) error
	Recv(ctx context.Context) (Res, error)
	streamx.ClientStream
}

// TODO: I don't really want to set svcInfo as an argument...
func NewV2StreamingClient(destService string, svcInfo *serviceinfo.ServiceInfo, g generic.Generic, opts ...client.Option) (StreamClient[any], error) {
	return newV2StreamingClientWithServiceInfo(destService, g, svcInfo, opts...)
}

func newV2StreamingClientWithServiceInfo(destService string, g generic.Generic, svcInfo *serviceinfo.ServiceInfo, opts ...client.Option) (StreamClient[any], error) {
	var options []client.Option
	options = append(options, client.WithGeneric(g))
	options = append(options, client.WithDestService(destService))
	options = append(options, opts...)

	kc, err := client.NewClient(svcInfo, options...)
	if err != nil {
		return nil, err
	}
	cli := &genericServiceStreamingClient[any]{
		svcInfo: svcInfo,
		kClient: kc,
		g:       g,
	}
	runtime.SetFinalizer(cli, (*genericServiceStreamingClient[any]).Close)

	svcInfo.GenericMethod = func(name string) serviceinfo.MethodInfo {
		n, err := g.GetMethod(nil, name)
		var key string
		if err != nil {
			// TODO: support fallback solution for binary
			key = serviceinfo.GenericMethod
		} else {
			key = getGenericStreamingMethodInfoKey(n.StreamingMode)
		}
		m := svcInfo.Methods[key]
		return &methodInfo{
			MethodInfo: m,
			oneway:     n.Oneway,
		}
	}

	return cli, nil
}

type clientV2StreamingClient struct {
	method     string
	methodInfo serviceinfo.MethodInfo
	*streamx.GenericClientStream[generic.Args, generic.Result]
}

func NewClientV2Streaming(ctx context.Context, genericCli StreamClient[any], method string, callOpts ...streamxcallopt.CallOption) (context.Context, V2ClientStreamingClient[any, any], error) {
	gCli, ok := genericCli.(*genericServiceStreamingClient[any])
	if !ok {
		return ctx, nil, errors.New("invalid generic client")
	}
	ctx, stream, err := streamxclient.InvokeStream[generic.Args, generic.Result](ctx, gCli.kClient, serviceinfo.StreamingClient, method, nil, nil, callOpts...)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, &clientV2StreamingClient{method, gCli.svcInfo.MethodInfo(method), stream}, nil
}

func (cs *clientV2StreamingClient) Send(ctx context.Context, req any) error {
	_args := cs.methodInfo.NewArgs().(*generic.Args)
	_args.Method = cs.method
	_args.Request = req
	return cs.SendMsg(ctx, _args)
}

func (cs *clientV2StreamingClient) CloseAndRecv(ctx context.Context) (resp any, err error) {
	if err = cs.CloseSend(ctx); err != nil {
		return nil, err
	}
	_result := cs.methodInfo.NewResult().(*generic.Result)
	if err = cs.RecvMsg(ctx, _result); err != nil {
		return nil, err
	}
	return _result.GetSuccess(), nil
}

type serverV2StreamingClient struct {
	methodInfo serviceinfo.MethodInfo
	*streamx.GenericClientStream[generic.Args, generic.Result]
}

func NewServerV2Streaming(ctx context.Context, genericCli StreamClient[any], method string, req any, callOpts ...streamxcallopt.CallOption) (context.Context, V2ServerStreamingClient[any, any], error) {
	gCli, ok := genericCli.(*genericServiceStreamingClient[any])
	if !ok {
		return nil, nil, errors.New("invalid generic client")
	}
	mi := gCli.svcInfo.MethodInfo(method)
	_args := mi.NewArgs().(*generic.Args)
	_args.Method = method
	_args.Request = req
	ctx, stream, err := streamxclient.InvokeStream[generic.Args, generic.Result](ctx, gCli.kClient, serviceinfo.StreamingServer, method, _args, nil, callOpts...)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, &serverV2StreamingClient{mi, stream}, nil
}

func (ss *serverV2StreamingClient) Recv(ctx context.Context) (resp any, err error) {
	_result := ss.methodInfo.NewResult().(*generic.Result)
	if err = ss.RecvMsg(ctx, _result); err != nil {
		return nil, err
	}
	return _result.GetSuccess(), nil
}

type bidirectionalV2StreamingClient struct {
	method     string
	methodInfo serviceinfo.MethodInfo
	*streamx.GenericClientStream[generic.Args, generic.Result]
}

func NewBidirectionalV2Streaming(ctx context.Context, genericCli StreamClient[any], method string, callOpts ...streamxcallopt.CallOption) (context.Context, V2BidirectionalStreamingClient[any, any], error) {
	gCli, ok := genericCli.(*genericServiceStreamingClient[any])
	if !ok {
		return ctx, nil, errors.New("invalid generic client")
	}
	ctx, stream, err := streamxclient.InvokeStream[generic.Args, generic.Result](ctx, gCli.kClient, serviceinfo.StreamingBidirectional, method, nil, nil, callOpts...)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, &bidirectionalV2StreamingClient{method, gCli.svcInfo.MethodInfo(method), stream}, nil
}

func (bs *bidirectionalV2StreamingClient) Send(ctx context.Context, req any) error {
	_args := bs.methodInfo.NewArgs().(*generic.Args)
	_args.Method = bs.method
	_args.Request = req
	return bs.SendMsg(ctx, _args)
}

func (bs *bidirectionalV2StreamingClient) Recv(ctx context.Context) (resp any, err error) {
	_result := bs.methodInfo.NewResult().(*generic.Result)
	if err = bs.RecvMsg(ctx, _result); err != nil {
		return nil, err
	}
	return _result.GetSuccess(), nil
}
