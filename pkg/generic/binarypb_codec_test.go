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

package generic

import (
	"context"
	"io/ioutil"
	"log"
	"math"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/binarypb_test/kitex_gen/base"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"google.golang.org/protobuf/proto"
)

func TestBinaryProtobufCodec(t *testing.T) {
	bpc := &binaryProtobufCodec{protobufCodec: pbCodec}

	test.Assert(t, bpc.Name() == "RawProtobufBinary", bpc.Name())

	// client side
	buf := readBasicExampleReqProtoBufData()
	rwbuf := remote.NewReaderWriterBuffer(1024)
	cliMsg := &mockMessage{
		RPCInfoFunc: func() rpcinfo.RPCInfo {
			return newMockRPCInfo()
		},
		RPCRoleFunc: func() remote.RPCRole {
			return remote.Client
		},
		DataFunc: func() interface{} {
			return &Args{
				Request: buf,
				Method:  "ExampleMethod",
			}
		},
	}
	err := bpc.Marshal(context.Background(), cliMsg, rwbuf)
	test.Assert(t, err == nil, err)

	// server side
	arg := &Args{}
	svrMsg := &mockMessage{
		RPCInfoFunc: func() rpcinfo.RPCInfo {
			return newMockRPCInfo()
		},
		RPCRoleFunc: func() remote.RPCRole {
			return remote.Server
		},
		DataFunc: func() interface{} {
			return arg
		},
		PayloadLenFunc: func() int {
			return rwbuf.ReadableLen()
		},
		ServiceInfoFunc: func() *serviceinfo.ServiceInfo {
			return ServiceInfo(serviceinfo.Protobuf)
		},
	}
	err = bpc.Unmarshal(context.Background(), svrMsg, rwbuf)
	test.Assert(t, err == nil, err)
	reqBuf := svrMsg.Data().(*Args).Request.(binaryReqType)

	// Check if the binary data is unmarshalled correctly
	exp := constructBasicExampleReqObject()
	var act base.BasicExample
	// Unmarshal the binary data into the act struct
	err = proto.Unmarshal(reqBuf, &act)
	if err != nil {
		log.Fatalf("Failed to unmarshal Protobuf data: %v", err)
	}
	// compare exp and act structs
	if !proto.Equal(exp, &act) {
		log.Fatalf("Expected and actual Protobuf data are not equal")
	}
}

func TestBinaryProtobufCodecExceptionError(t *testing.T) {
	ctx := context.Background()
	bpc := &binaryProtobufCodec{protobufCodec: pbCodec}
	cliMsg := &mockMessage{
		RPCInfoFunc: func() rpcinfo.RPCInfo {
			return newEmptyMethodRPCInfo()
		},
		RPCRoleFunc: func() remote.RPCRole {
			return remote.Server
		},
		MessageTypeFunc: func() remote.MessageType {
			return remote.Exception
		},
	}

	rwbuf := remote.NewReaderWriterBuffer(1024)
	// test data is empty
	err := bpc.Marshal(ctx, cliMsg, rwbuf)
	test.Assert(t, err.Error() == "invalid marshal data in rawProtobufBinaryCodec: nil")
	cliMsg.DataFunc = func() interface{} {
		return &remote.TransError{}
	}

	// empty method
	err = bpc.Marshal(ctx, cliMsg, rwbuf)
	test.Assert(t, err.Error() == "rawProtobufBinaryCodec Marshal exception failed, err: empty methodName in protobuf Marshal")

	cliMsg.RPCInfoFunc = func() rpcinfo.RPCInfo {
		return newMockRPCInfo()
	}
	err = bpc.Marshal(ctx, cliMsg, rwbuf)
	test.Assert(t, err == nil)

	// test server role
	cliMsg.MessageTypeFunc = func() remote.MessageType {
		return remote.Call
	}
	cliMsg.DataFunc = func() interface{} {
		return &Result{
			Success: binaryPbReqType{},
		}
	}
	err = bpc.Marshal(ctx, cliMsg, rwbuf)
	test.Assert(t, err == nil)
}

func readBasicExampleReqProtoBufData() []byte {
	out, err := ioutil.ReadFile("./binarypb_test/data/basic_example_pb.bin")
	if err != nil {
		panic(err)
	}
	return out
}

func constructBasicExampleReqObject() *base.BasicExample {
	req := new(base.BasicExample)
	req = &base.BasicExample{
		Int32:        123,
		Int64:        123,
		Uint32:       uint32(math.MaxInt32),
		Uint64:       uint64(math.MaxInt64),
		Sint32:       123,
		Sint64:       123,
		Sfixed32:     123,
		Sfixed64:     123,
		Fixed32:      123,
		Fixed64:      123,
		Float:        123.123,
		Double:       123.123,
		Bool:         true,
		Str:          "hello world!",
		Bytes:        []byte{0x1, 0x2, 0x3, 0x4},
		ListInt32:    []int32{100, 200, 300, 400, 500},
		ListInt64:    []int64{100, 200, 300, 400, 500},
		ListUint32:   []uint32{100, 200, 300, 400, 500},
		ListUint64:   []uint64{100, 200, 300, 400, 500},
		ListSint32:   []int32{100, 200, 300, 400, 500},
		ListSint64:   []int64{100, 200, 300, 400, 500},
		ListSfixed32: []int32{100, 200, 300, 400, 500},
		ListSfixed64: []int64{100, 200, 300, 400, 500},
		ListFixed32:  []uint32{100, 200, 300, 400, 500},
		ListFixed64:  []uint64{100, 200, 300, 400, 500},
		ListFloat:    []float32{1.1, 2.2, 3.3, 4.4, 5.5},
		ListDouble:   []float64{1.1, 2.2, 3.3, 4.4, 5.5},
		ListBool:     []bool{true, false, true, false, true},
		ListString:   []string{"a1", "b2", "c3", "d4", "e5"},
		ListBytes:    [][]byte{{0x1, 0x2, 0x3, 0x4}, {0x5, 0x6, 0x7, 0x8}},
		MapInt64SINT32: map[int64]int32{
			0:             0,
			math.MaxInt64: math.MaxInt32,
			math.MinInt64: math.MinInt32,
		},
		MapInt64Sfixed32: map[int64]int32{
			0:             0,
			math.MaxInt64: math.MaxInt32,
			math.MinInt64: math.MinInt32,
		},
		MapInt64Fixed32: map[int64]uint32{
			math.MaxInt64: uint32(math.MaxInt32),
			math.MinInt64: 0,
		},
		MapInt64Uint32: map[int64]uint32{
			0:             0,
			math.MaxInt64: uint32(math.MaxInt32),
			math.MinInt64: 0,
		},
		MapInt64Double: map[int64]float64{
			0:             0,
			math.MaxInt64: math.MaxFloat64,
			math.MinInt64: math.SmallestNonzeroFloat64,
		},
		MapInt64Bool: map[int64]bool{
			0:             false,
			math.MaxInt64: true,
			math.MinInt64: false,
		},
		MapInt64String: map[int64]string{
			0:             "0",
			math.MaxInt64: "max",
			math.MinInt64: "min",
		},
		MapInt64Bytes: map[int64][]byte{
			0:             {0x0},
			math.MaxInt64: {0x1, 0x2, 0x3, 0x4},
			math.MinInt64: {0x5, 0x6, 0x7, 0x8},
		},
		MapInt64Float: map[int64]float32{
			0:             0,
			math.MaxInt64: math.MaxFloat32,
			math.MinInt64: math.SmallestNonzeroFloat32,
		},
		MapInt64Int32: map[int64]int32{
			0:             0,
			math.MaxInt64: math.MaxInt32,
			math.MinInt64: math.MinInt32,
		},
		MapstringSINT64: map[string]int64{
			"0":   0,
			"max": math.MaxInt64,
			"min": math.MinInt64,
		},
		MapstringSfixed64: map[string]int64{
			"0":   0,
			"max": math.MaxInt64,
			"min": math.MinInt64,
		},
		MapstringFixed64: map[string]uint64{
			"max": uint64(math.MaxInt64),
			"min": 0,
		},
		MapstringUint64: map[string]uint64{
			"max": uint64(math.MaxInt64),
			"min": 0,
		},
		MapstringDouble: map[string]float64{
			"0":   0,
			"max": math.MaxFloat64,
			"min": math.SmallestNonzeroFloat64,
		},
		MapstringBool: map[string]bool{
			"0":   false,
			"max": true,
		},
		MapstringString: map[string]string{
			"0":   "0",
			"max": "max",
			"min": "min",
		},
		MapstringBytes: map[string][]byte{
			"0":   {0x0},
			"max": {0x1, 0x2, 0x3, 0x4},
			"min": {0x5, 0x6, 0x7, 0x8},
		},
		MapstringFloat: map[string]float32{
			"0":   0,
			"max": math.MaxFloat32,
			"min": math.SmallestNonzeroFloat32,
		},
		MapstringInt64: map[string]int64{
			"0":   0,
			"max": math.MaxInt64,
			"min": math.MinInt64,
		},
	}

	return req
}
