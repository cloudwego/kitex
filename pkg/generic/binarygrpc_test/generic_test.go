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

//import (
//	"context"
//	"fmt"
//	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
//	"github.com/cloudwego/kitex/server"
//	"net"
//	"testing"
//
//	"github.com/cloudwego/kitex/client"
//	"github.com/cloudwego/kitex/client/genericclient"
//	"github.com/cloudwego/kitex/internal/test"
//	"github.com/cloudwego/kitex/pkg/generic"
//	"github.com/cloudwego/kitex/transport"
//)
//
//func TestRawGRPCBinaryClientStreaming2NormalServer(t *testing.T) {
//	svr := initMockServer()
//	g := generic.BinaryGRPCGeneric()
//	streamCli, err := genericclient.NewClientStreamingClient(context.Background(), "destServiceName", "METHOD_NAME", g, client.WithTransportProtocol(transport.GRPC))
//	test.Assert(t, err == nil)
//	// TODO: req binary proto
//	for {
//		streamCli.Send(req)
//	}
//	resp, err := streamCli.CloseAndRecv()
//	test.Assert(t, err == nil)
//	test.Assert(t, resp == "") // TODO
//}
//
//// TODO: move to binarygrpc_init
//func initGRPCBinaryClient() {
//
//}

//import (
//	"github.com/cloudwego/kitex/client/genericclient"
//	"testing"
//)
//
//// 1. clientStreaming to NormalSvr
//func TestRawGRPCBinaryClientStreaming2NormalServer(t *testing.T) {
//	// init normal server
//	// defer svr.Stop()
//
//	// cli := initRawGRPCBinaryClient()
//	// buf := genPBBinaryReqBuf(method) // []byte
//
//	// resp, err := cli.GenericCall(context.Background(), method, buf, callopt.WithRPCTimeout(1*time.Second))
//	// test.Assert(t, err == nil, err)
//	// respBuf, ok := resp.([]byte)
//	// test.Assert(t, ok)
//	// test.Assert(t, .. == respMsg)
//}
//
//func initRawGRPCBinaryClient() genericclient.Client {
//	// g := generic.BinaryGRPCGeneric()
//	// cli := newGenericClient("destServiceName", g, addr)
//	// return cli
//}
//
//// 2. serverStreaming to NormalSvr
//// 3. bidirectionalStreaming to NormalSvr
//// 4. clientStreaming to generic Svr
//// 5. serverStreaming to generic Svr
//// 6. bidirectionalStreaming to generic Svr
