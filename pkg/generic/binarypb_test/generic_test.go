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
	"log"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/binarypb_test/kitex_gen/api"
	"github.com/cloudwego/kitex/pkg/generic/binarypb_test/kitex_gen/base"
	"github.com/cloudwego/kitex/pkg/generic/binarypb_test/kitex_gen/example2"
	"github.com/cloudwego/kitex/pkg/kerrors"
)

func TestRun(t *testing.T) {
	t.Run("TestBasicServiceExampleMethod", TestBasicServiceExampleMethod)
	t.Run("TestExample2ExampleMethod", TestExample2ExampleMethod)
	t.Run("TestEchoNormalServer", TestEchoNormalServer)
	t.Run("TestBizErrReq", TestBizErrReq)
}

func TestBasicServiceExampleMethod(t *testing.T) {
	buf := getBasicExampleReq()

	// Create client and server
	svr := initRawProtobufBinaryServer(new(GenericServiceImpl))
	defer svr.Stop()
	cli := initRawProtobufBinaryClient("BasicService")

	// Make generic call
	method := "ExampleMethod"
	resp, err := cli.GenericCall(context.Background(), method, buf, callopt.WithRPCTimeout(1*time.Second))
	test.Assert(t, err == nil, err)
	respBytes, ok := resp.([]byte)
	pbRespBytes := respBytes[12+len(method):]
	test.Assert(t, ok)

	// Check the response
	var act base.BasicExample
	exp := getBasicExampleReqObject()
	// Unmarshal the binary response into the act struct
	err = proto.Unmarshal(pbRespBytes, &act)
	if err != nil {
		log.Fatalf("Failed to unmarshal Protobuf data: %v", err)
	}
	// compare exp and act structs
	if !proto.Equal(exp, &act) {
		log.Fatalf("Expected and actual Protobuf data are not equal")
	}
}

func TestExample2ExampleMethod(t *testing.T) {
	buf := getExample2Req()

	// Create client and server
	svr := initRawProtobufBinaryServer(new(Example2ExampleMethodService))
	defer svr.Stop()
	cli := initRawProtobufBinaryClient("BasicService")

	// Make generic call
	method := "ExampleMethod"
	resp, err := cli.GenericCall(context.Background(), method, buf, callopt.WithRPCTimeout(1*time.Second))
	test.Assert(t, err == nil, err)
	respBytes, ok := resp.([]byte)
	pbRespBytes := respBytes[12+len(method):]
	test.Assert(t, ok)

	// Check the response
	var act example2.ExampleResp
	exp := getExample2RespObject()
	// Unmarshal the binary response into the act struct
	err = proto.Unmarshal(pbRespBytes, &act)
	if err != nil {
		log.Fatalf("Failed to unmarshal Protobuf data: %v", err)
	}
	// compare exp and act structs
	if !proto.Equal(exp, &act) {
		log.Fatalf("Expected and actual Protobuf data are not equal")
	}
}

func TestEchoNormalServer(t *testing.T) {
	buf := getEchoReq()

	// Create client and server
	svr := initEchoServer()
	defer svr.Stop()
	cli := initRawProtobufBinaryClient("Echo")

	// Make generic call
	method := "EchoPB"
	resp, err := cli.GenericCall(context.Background(), method, buf, callopt.WithRPCTimeout(1*time.Second))
	test.Assert(t, err == nil, err)
	respBytes, ok := resp.([]byte)
	pbRespBytes := respBytes[12+len(method):]
	test.Assert(t, ok)

	// Check the response
	var act api.Response
	exp := getEchoRespObject()
	// Unmarshal the binary response into the act struct
	err = proto.Unmarshal(pbRespBytes, &act)
	if err != nil {
		log.Fatalf("Failed to unmarshal Protobuf data: %v", err)
	}
	// compare exp and act structs
	if !proto.Equal(exp, &act) {
		log.Fatalf("Expected and actual Protobuf data are not equal")
	}
}

func TestBizErrReq(t *testing.T) {
	buf := getBizErrReq()

	// Create client and server
	svr := initEchoServer()
	defer svr.Stop()
	cli := initRawProtobufBinaryClient("Echo")

	// Make generic call
	method := "EchoPB"
	_, err := cli.GenericCall(context.Background(), method, buf, callopt.WithRPCTimeout(1*time.Second))

	// Check response is an error
	bizerr, ok := kerrors.FromBizStatusError(err)
	if !ok {
		log.Fatalf("Failed to get biz status error: %v", err)
	}
	test.Assert(t, ok)
	test.Assert(t, bizerr.BizStatusCode() == 404)
	test.Assert(t, bizerr.BizMessage() == "not found")
}
