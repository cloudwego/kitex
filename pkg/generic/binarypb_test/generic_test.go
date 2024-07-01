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

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/binarypb_test/kitex_gen/base"
	"github.com/cloudwego/kitex/pkg/generic/binarypb_test/kitex_gen/example2"
	"google.golang.org/protobuf/proto"
)

func TestRun(t *testing.T) {
	t.Run("TestBasicServiceExampleMethod", TestBasicServiceExampleMethod)
	t.Run("TestExample2ExampleMethod", TestExample2ExampleMethod)
}

func TestBasicServiceExampleMethod(t *testing.T) {
	buf := readBasicExampleReqProtoBufData()

	// Create client and server
	svr := initRawProtobufBinaryServer(new(GenericServiceImpl))
	defer svr.Stop()
	cli := initRawProtobufBinaryClient()

	// Make generic call
	method := "ExampleMethod"
	resp, err := cli.GenericCall(context.Background(), method, buf, callopt.WithRPCTimeout(1*time.Second))
	test.Assert(t, err == nil, err)
	respBytes, ok := resp.([]byte)
	test.Assert(t, ok)

	// Check the response
	var act base.BasicExample
	exp := constructBasicExampleReqObject()
	// Unmarshal the binary response into the act struct
	err = proto.Unmarshal(respBytes, &act)
	if err != nil {
		log.Fatalf("Failed to unmarshal Protobuf data: %v", err)
	}
	// compare exp and act structs
	if !proto.Equal(exp, &act) {
		log.Fatalf("Expected and actual Protobuf data are not equal")
	}
}

func TestExample2ExampleMethod(t *testing.T) {
	buf := readExample2ReqProtoBufData()

	// Create client and server
	svr := initRawProtobufBinaryServer(new(Example2ExampleMethodService))
	defer svr.Stop()
	cli := initRawProtobufBinaryClient()

	// Make generic call
	method := "ExampleMethod"
	resp, err := cli.GenericCall(context.Background(), method, buf, callopt.WithRPCTimeout(1*time.Second))
	test.Assert(t, err == nil, err)
	respBytes, ok := resp.([]byte)
	test.Assert(t, ok)

	// Check the response
	var act example2.ExampleResp
	exp := constructExample2RespObject()
	// Unmarshal the binary response into the act struct
	err = proto.Unmarshal(respBytes, &act)
	if err != nil {
		log.Fatalf("Failed to unmarshal Protobuf data: %v", err)
	}
	// compare exp and act structs
	if !proto.Equal(exp, &act) {
		log.Fatalf("Expected and actual Protobuf data are not equal")
	}
}
