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

package genericserver

import (
	"context"
	"errors"

	iserver "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/server"
)

// RegisterUnknownServiceOrMethodHandler registers a service handler for unknown service or method traffic.
//
// After invoking this interface, the impact on server traffic processing is as follows:
//
// - Unknown service scenario:
//  1. Match the service info which has the same service name as the requested service name.
//  2. If no match exists, a unique service info will be created using the requested service name
//     and returned as the result of service searching.
//
// - Unknown method scenario:
//  1. Match the method info which has the same method name as the requested method name.
//  2. If no match exists,
//     - For default ping pong traffic (ttheader/framed e.g.): Requests are processed via the UnknownServiceOrMethodHandler.DefaultHandler.
//     - For streaming traffic (including grpc unary): Requests are handled by the UnknownServiceOrMethodHandler.StreamingHandler.
//
// This implementation relies on binary generic invocation.
// User-processed request/response objects through UnknownServiceOrMethodHandler
// are of type []byte, containing serialized data of the original request/response.
// And note that for Thrift Unary methods, the serialized data includes encapsulation of the Arg or Result structure.
//
// Code example:
//
//	svr := server.NewServer(opts...)
//	err := genericserver.RegisterUnknownServiceOrMethodHandler(svr, &UnknownServiceOrMethodHandler{
//		DefaultHandler:  defaultHandler,
//		StreamingHandler: streamingHandler,
//	})
func RegisterUnknownServiceOrMethodHandler(svr server.Server, unknownHandler *UnknownServiceOrMethodHandler) error {
	if unknownHandler.DefaultHandler == nil && unknownHandler.StreamingHandler == nil {
		return errors.New("neither default nor streaming handler registered")
	}
	return svr.RegisterService(&serviceinfo.ServiceInfo{
		ServiceName: "$UnknownService", // meaningless service info
	}, &generic.ServiceV2{
		GenericCall:   unknownHandler.DefaultHandler,
		BidiStreaming: unknownHandler.StreamingHandler,
	}, iserver.WithUnknownService())
}

// NewUnknownServiceOrMethodServer creates a server which registered UnknownServiceOrMethodHandler
// through RegisterUnknownServiceOrMethodHandler. For more details, see RegisterUnknownServiceOrMethodHandler.
func NewUnknownServiceOrMethodServer(unknownHandler *UnknownServiceOrMethodHandler, options ...server.Option) server.Server {
	svr := server.NewServer(options...)
	if err := RegisterUnknownServiceOrMethodHandler(svr, unknownHandler); err != nil {
		panic(err)
	}
	return svr
}

// UnknownServiceOrMethodHandler handles unknown service or method traffic.
type UnknownServiceOrMethodHandler struct {
	// Optional, handles default ping pong traffic (ttheader/framed e.g.), support both thrift binary and protobuf binary.
	DefaultHandler func(ctx context.Context, service, method string, request interface{}) (response interface{}, err error)

	// Optional, handles all streaming traffic (including grpc unary), support both thrift binary and protobuf binary.
	StreamingHandler func(ctx context.Context, service, method string, stream generic.BidiStreamingServer) (err error)
}
