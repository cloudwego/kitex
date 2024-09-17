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
	"io"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/ktx"
)

type pingpongService struct{}
type streamingService struct{}

func (si *pingpongService) PingPong(ctx context.Context, req *Request) (*Response, error) {
	resp := &Response{Type: req.Type, Message: req.Message}
	klog.Infof("Server PingPong: req={%v} resp={%v}", req, resp)
	return resp, nil
}

func (si *streamingService) Unary(ctx context.Context, req *Request) (*Response, error) {
	resp := &Response{Type: req.Type, Message: req.Message}
	klog.Infof("Server Unary: req={%v} resp={%v}", req, resp)
	return resp, nil
}

func (si *streamingService) ClientStream(ctx context.Context,
	stream streamx.ClientStreamingServer[ttstream.Header, ttstream.Trailer, Request, Response]) (res *Response, err error) {
	var msg string
	klog.Infof("Server ClientStream start")
	defer klog.Infof("Server ClientStream end")
	for {
		req, err := stream.Recv(ctx)
		if err == io.EOF {
			res = new(Response)
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

func (si *streamingService) ServerStream(ctx context.Context, req *Request,
	stream streamx.ServerStreamingServer[ttstream.Header, ttstream.Trailer, Response]) error {
	klog.Infof("Server ServerStream: req={%v}", req)

	_ = stream.SetHeader(map[string]string{"key1": "val1"})
	_ = stream.SendHeader(map[string]string{"key2": "val2"})
	_ = stream.SetTrailer(map[string]string{"key1": "val1"})
	_ = stream.SetTrailer(map[string]string{"key2": "val2"})

	for i := 0; i < 3; i++ {
		resp := new(Response)
		resp.Type = int32(i)
		resp.Message = req.Message
		err := stream.Send(ctx, resp)
		if err != nil {
			return err
		}
		klog.Infof("Server ServerStream: send resp={%v}", resp)
	}
	return nil
}

func (si *streamingService) BidiStream(ctx context.Context,
	stream streamx.BidiStreamingServer[ttstream.Header, ttstream.Trailer, Request, Response]) error {
	ktx.RegisterCancelCallback(ctx, func() {
		klog.Warnf("RegisterCancelCallback work!")
	})
	klog.Warnf("RegisterCancelCallback registered!")
	for {
		req, err := stream.Recv(ctx)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp := new(Response)
		resp.Message = req.Message
		err = stream.Send(ctx, resp)
		if err != nil {
			return err
		}
		klog.Infof("Server BidiStream: req={%v} resp={%v}", req, resp)
	}
}
