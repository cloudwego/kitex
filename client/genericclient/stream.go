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
	"fmt"
	"runtime"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

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

/*
// create a normal generic client (the following uses a binary generic as an example)
client, err := genericclient.NewStreamingClient("destServiceName", generic.BinaryPbGeneric(), client.WithTransportProtocol(transport.GRPC))
// create a generic client streaming client
streamCli, err := genericclient.NewClientStreaming(context.Background(), client, "StreamRequestEcho", callOpts...)
// send streaming requests

	for i := 0; i < 10; i++ {
	    if err = streamCli.Send(req); err != nil {
	       klog.Fatal(err)
	    }
	    time.Sleep(time.Second)
	}

// receive a response
resp, err := streamCli.CloseAndRecv()
*/

//type genericStreamingServiceClient struct {
//	genericServiceClient
//	serviceinfo.StreamingMode
//	methodType serviceinfo.StreamingMode
//}

func NewStreamingClient(destService string, g generic.Generic, opts ...client.Option) (Client, error) {
	svcInfo := generic.StreamingServiceInfo(g.PayloadCodecType(), g.CodecInfo())
	return NewStreamingClientWithServiceInfo(destService, g, svcInfo, opts...)
}

// TODO: combine with NewClienttWithServiceInfo?
func NewStreamingClientWithServiceInfo(destService string, g generic.Generic, svcInfo *serviceinfo.ServiceInfo, opts ...client.Option) (Client, error) {
	var options []client.Option
	options = append(options, client.WithGeneric(g))
	options = append(options, client.WithDestService(destService))
	options = append(options, opts...)

	kc, err := client.NewClient(svcInfo, options...)
	if err != nil {
		return nil, err
	}
	cli := &genericServiceClient{
		kClient: kc,
		g:       g,
	}
	runtime.SetFinalizer(cli, (*genericServiceClient).Close)

	svcInfo.GenericMethod = func(name string) serviceinfo.MethodInfo {
		n, err := g.GetMethod(nil, name)
		m := svcInfo.Methods[getGenericStreamingMethodInfoKey(n.StreamingMode)]
		if err != nil {
			return m
		}
		return &methodInfo{
			MethodInfo: m,
			oneway:     n.Oneway,
		}
	}

	return cli, nil
}

func getGenericStreamingMethodInfoKey(streamingMode serviceinfo.StreamingMode) string {
	switch streamingMode {
	case serviceinfo.StreamingClient:
		return serviceinfo.GenericClientStreamingMethod
	case serviceinfo.StreamingServer:
		return serviceinfo.GenericServerStreamingMethod
	case serviceinfo.StreamingBidirectional:
		return serviceinfo.GenericBidirectionalStreamingMethod
	default:
		return serviceinfo.GenericMethod
	}
}

type clientStreamingClient struct {
	streaming.Stream
	svcInfo *serviceinfo.ServiceInfo
	method  string
}

// TODO: ここでsvcInfoにモードを設定する
func NewClientStreaming(ctx context.Context, genericCli Client, method string, callOpts ...callopt.Option) (ClientStreaming, error) {
	gCli, ok := genericCli.(*genericServiceClient)
	if !ok {
		// TODO: error message
		return nil, errors.New("oijpijoij")
	}
	stream, err := getStream(ctx, genericCli, method, callOpts...)
	if err != nil {
		return nil, err
	}
	return &clientStreamingClient{stream, gCli.svcInfo, method}, nil
}

func (cs *clientStreamingClient) Send(req interface{}) error {
	_args := cs.svcInfo.MethodInfo(serviceinfo.GenericClientStreamingMethod).NewArgs().(*generic.Args)
	_args.Method = cs.method
	_args.Request = req
	return cs.Stream.SendMsg(_args)
}

func (cs *clientStreamingClient) CloseAndRecv() (resp interface{}, err error) {
	if err := cs.Stream.Close(); err != nil {
		return nil, err
	}
	_result := cs.svcInfo.MethodInfo(serviceinfo.GenericClientStreamingMethod).NewResult().(*generic.Result)
	if err = cs.Stream.RecvMsg(_result); err != nil {
		return nil, err
	}
	return _result.GetSuccess(), nil
}

type serverStreamingClient struct {
	streaming.Stream
	svcInfo *serviceinfo.ServiceInfo
}

// TODO: no need servicename?
func NewServerStreaming(ctx context.Context, genericCli Client, method string, req interface{}, callOpts ...callopt.Option) (ServerStreaming, error) {
	gCli, ok := genericCli.(*genericServiceClient)
	if !ok {
		// TODO: error msg
		return nil, errors.New("asdfasdfaf")
	}
	stream, err := getStream(ctx, genericCli, method, callOpts...)
	if err != nil {
		return nil, err
	}
	ss := &serverStreamingClient{Stream: stream, svcInfo: gCli.svcInfo}
	_args := gCli.svcInfo.MethodInfo(serviceinfo.GenericServerStreamingMethod).NewArgs().(*generic.Args)
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

func (ss *serverStreamingClient) Recv() (resp interface{}, err error) {
	_result := ss.svcInfo.MethodInfo(serviceinfo.GenericServerStreamingMethod).NewResult().(*generic.Result)
	if err = ss.Stream.RecvMsg(_result); err != nil {
		return nil, err
	}
	return _result.GetSuccess(), nil
}

type bidirectionalStreamingClient struct {
	streaming.Stream
	svcInfo *serviceinfo.ServiceInfo
	method  string
}

func NewBidirectionalStreaming(ctx context.Context, genericCli Client, method string, callOpts ...callopt.Option) (BidirectionalStreaming, error) {
	gCli, ok := genericCli.(*genericServiceClient)
	if !ok {
		// TODO: err msg
		return nil, errors.New("adsfad")
	}
	stream, err := getStream(ctx, genericCli, method, callOpts...)
	if err != nil {
		return nil, err
	}
	return &bidirectionalStreamingClient{Stream: stream, svcInfo: gCli.svcInfo, method: method}, nil
}

func (bs *bidirectionalStreamingClient) Send(req interface{}) error {
	_args := bs.svcInfo.MethodInfo(serviceinfo.GenericBidirectionalStreamingMethod).NewArgs().(*generic.Args)
	_args.Method = bs.method
	_args.Request = req
	return bs.Stream.SendMsg(_args)
}

func (bs *bidirectionalStreamingClient) Recv() (resp interface{}, err error) {
	_result := bs.svcInfo.MethodInfo(serviceinfo.GenericBidirectionalStreamingMethod).NewResult().(*generic.Result)
	if err = bs.Stream.RecvMsg(_result); err != nil {
		return nil, err
	}
	return _result.GetSuccess(), nil
}

// TODO: should be an option to specify the sub-content-type
func getStream(ctx context.Context, genericCli Client, method string, callOpts ...callopt.Option) (streaming.Stream, error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOpts)
	streamClient, ok := genericCli.(*genericServiceClient).kClient.(client.Streaming)
	if !ok {
		return nil, fmt.Errorf("client not support streaming")
	}
	res := new(streaming.Result)
	err := streamClient.Stream(ctx, method, nil, res)
	if err != nil {
		return nil, err
	}
	return res.Stream, nil
}
