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

package test

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/client/streamxclient"
	istreamx "github.com/cloudwego/kitex/internal/mocks/streamx"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/ktx"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/streamxserver"
	"github.com/cloudwego/kitex/transport"
)

func newGenericStreamingClient(g generic.Generic, targetIPPort string) genericclient.StreamClient[any] {
	svcInfo := genericclient.StreamingServiceInfo(g)
	cp, _ := ttstream.NewClientProvider(svcInfo)
	cli, err := genericclient.NewV2StreamingClient("destService", svcInfo, g,
		client.WithHostPorts(targetIPPort),
		client.WithTransportProtocol(transport.TTHeaderFramed),
		streamxclient.WithProvider(cp),
	)
	if err != nil {
		panic(err)
	}
	return cli
}

func initStreamxTestServer(handler istreamx.TestService) (string, server.Server, error) {
	addr := test.GetLocalAddress()
	ln, err := netpoll.CreateListener("tcp", addr)
	if err != nil {
		return "", nil, err
	}
	sp, _ := ttstream.NewServerProvider(istreamx.ServiceInfo())

	options := []server.Option{
		server.WithListener(ln),
		server.WithExitWaitTime(time.Millisecond * 10),
		streamxserver.WithProvider(sp),
	}
	svr := istreamx.NewServer(handler, options...)
	go func() {
		_ = svr.Run()
	}()
	return addr, svr, nil
}

var (
	testCode  = int32(10001)
	testMsg   = "biz testMsg"
	testExtra = map[string]string{
		"testKey": "testVal",
	}
	normalErrMsg = "normal error"
)

const (
	headerKey  = "header1"
	headerVal  = "value1"
	trailerKey = "trailer1"
	trailerVal = "value1"
)

const (
	normalErr int32 = iota + 1
	bizErr
)

type TestServiceImpl struct{}

func (si *TestServiceImpl) PingPong(ctx context.Context, req *istreamx.Request) (*istreamx.Response, error) {
	resp := &istreamx.Response{Type: req.Type, Message: req.Message}
	klog.Debugf("Server PingPong: req={%v} resp={%v}", req, resp)
	return resp, nil
}

func (si *TestServiceImpl) Unary(ctx context.Context, req *istreamx.Request) (*istreamx.Response, error) {
	resp := &istreamx.Response{Type: req.Type, Message: req.Message}
	klog.Debugf("Server Unary: req={%v} resp={%v}", req, resp)
	return resp, nil
}

func (si *TestServiceImpl) ClientStream(ctx context.Context,
	stream streamx.ClientStreamingServer[istreamx.Request, istreamx.Response],
) (*istreamx.Response, error) {
	var msg string
	klog.Debugf("Server ClientStream start")
	defer klog.Debugf("Server ClientStream end")

	if err := si.setHeaderAndTrailer(stream); err != nil {
		return nil, err
	}
	for {
		req, err := stream.Recv(ctx)
		if err == io.EOF {
			res := new(istreamx.Response)
			res.Message = msg
			return res, nil
		}
		if err != nil {
			return nil, err
		}
		msg = req.Message
		klog.Infof("Server ClientStream: req={%v}", req)
	}
}

func (si *TestServiceImpl) ServerStream(ctx context.Context, req *istreamx.Request,
	stream streamx.ServerStreamingServer[istreamx.Response],
) error {
	klog.Debugf("Server ServerStream: req={%v}", req)

	if err := si.setHeaderAndTrailer(stream); err != nil {
		return err
	}

	for i := 0; i < 5; i++ {
		resp := new(istreamx.Response)
		resp.Type = int32(i)
		resp.Message = req.Message
		err := stream.Send(ctx, resp)
		if err != nil {
			return err
		}
		klog.Debugf("Server ServerStream: send resp={%v}", resp)
	}
	return nil
}

func (si *TestServiceImpl) BidiStream(ctx context.Context,
	stream streamx.BidiStreamingServer[istreamx.Request, istreamx.Response],
) error {
	ktx.RegisterCancelCallback(ctx, func() {
		klog.Debugf("RegisterCancelCallback work!")
	})
	klog.Debugf("RegisterCancelCallback registered!")

	if err := si.setHeaderAndTrailer(stream); err != nil {
		return err
	}

	for {
		req, err := stream.Recv(ctx)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp := new(istreamx.Response)
		resp.Message = req.Message
		err = stream.Send(ctx, resp)
		if err != nil {
			return err
		}
		klog.Debugf("Server BidiStream: req={%v} resp={%v}", req, resp)
	}
}

func buildErr(req *istreamx.Request) error {
	var err error
	switch req.Type {
	case normalErr:
		err = errors.New(normalErrMsg)
	case bizErr:
		err = kerrors.NewBizStatusErrorWithExtra(testCode, testMsg, testExtra)
	default:
		klog.Fatalf("Unsupported Err Type: %d", req.Type)
	}
	return err
}

func (si *TestServiceImpl) UnaryWithErr(ctx context.Context, req *istreamx.Request) (*istreamx.Response, error) {
	err := buildErr(req)
	klog.Debugf("Server UnaryWithErr: req={%v} err={%v}", req, err)
	return nil, err
}

func (si *TestServiceImpl) ClientStreamWithErr(ctx context.Context, stream streamx.ClientStreamingServer[istreamx.Request, istreamx.Response]) (res *istreamx.Response, err error) {
	req, err := stream.Recv(ctx)
	if err != nil {
		klog.Errorf("Server ClientStreamWithErr Recv failed, exception={%v}", err)
		return nil, err
	}
	err = buildErr(req)
	klog.Debugf("Server ClientStreamWithErr: req={%v} err={%v}", req, err)
	return nil, err
}

func (si *TestServiceImpl) ServerStreamWithErr(ctx context.Context, req *istreamx.Request, stream streamx.ServerStreamingServer[istreamx.Response]) error {
	err := buildErr(req)
	klog.Debugf("Server ServerStreamWithErr: req={%v} err={%v}", req, err)
	return err
}

func (si *TestServiceImpl) BidiStreamWithErr(ctx context.Context, stream streamx.BidiStreamingServer[istreamx.Request, istreamx.Response]) error {
	req, err := stream.Recv(ctx)
	if err != nil {
		klog.Errorf("Server BidiStreamWithErr Recv failed, exception={%v}", err)
		return err
	}
	err = buildErr(req)
	klog.Debugf("Server BidiStreamWithErr: req={%v} err={%v}", req, err)
	return err
}

func (si *TestServiceImpl) setHeaderAndTrailer(stream streamx.ServerStreamMetadata) error {
	err := stream.SetTrailer(streamx.Trailer{trailerKey: trailerVal})
	if err != nil {
		return err
	}
	err = stream.SendHeader(streamx.Header{headerKey: headerVal})
	if err != nil {
		klog.Errorf("send header failed: %v", err)
		return err
	}
	return nil
}
