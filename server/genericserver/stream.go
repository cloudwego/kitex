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

package genericserver

import (
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// TODO: NewStreamingServer for multi-service
//func NewStreamingServer(handler generic.StreamingService, g generic.Generic, opts ...client.Option) {
//	svcInfo := generic.StreamingServiceInfo(g.PayloadCodecType(), g.CodecInfo())
//	return NewStreamingServerWithServiceInfo(handler, g, svcInfo, opts...)
//}
//
//func NewStreamingServerWithServiceInfo(handler generic.StreamingService, g generic.Generic, svcInfo *serviceinfo.ServiceInfo, opts ...server.Option) {
//
//}

type ClientStreaming interface {
	streaming.Stream
	Recv() (req interface{}, err error)
	SendAndClose(resp interface{}) error
}

type ServerStreaming interface {
	streaming.Stream
	Send(resp interface{}) error
}

type BidirectionalStreaming interface {
	streaming.Stream
	Recv() (req interface{}, err error)
	Send(resp interface{}) error
}

type clientStreamingServer struct {
	streaming.Stream
}

func NewClientStreamingServer(stream streaming.Stream) *clientStreamingServer {
	return &clientStreamingServer{stream}
}

func (cs *clientStreamingServer) Recv() (req interface{}, err error) {
	var _args generic.Args
	if err = cs.Stream.RecvMsg(&_args); err != nil {
		return nil, err
	}
	return _args.Request, nil
}

func (cs *clientStreamingServer) SendAndClose(resp interface{}) error {
	var _result generic.Result
	_result.Success = resp
	return cs.Stream.SendMsg(&_result)
}

type serverStreamingServer struct {
	streaming.Stream
}

func NewServerStreamingServer(stream streaming.Stream) *serverStreamingServer {
	return &serverStreamingServer{stream}
}

func (ss *serverStreamingServer) Send(resp interface{}) error {
	var _result generic.Result
	_result.Success = resp
	return ss.Stream.SendMsg(&_result)
}

type bidirectionalStreamingServer struct {
	streaming.Stream
}

func NewBidirectionalStreamingServer(stream streaming.Stream) *bidirectionalStreamingServer {
	return &bidirectionalStreamingServer{stream}
}

func (bs *bidirectionalStreamingServer) Recv() (req interface{}, err error) {
	var _args generic.Args
	if err = bs.Stream.RecvMsg(&_args); err != nil {
		return nil, err
	}
	return _args.Request, nil
}

func (bs *bidirectionalStreamingServer) Send(resp interface{}) error {
	var _result generic.Result
	_result.Success = resp
	return bs.Stream.SendMsg(&_result)
}

/*
	svr := genericserver.NewStreamServer()
	svr.RegisterService()
*/
