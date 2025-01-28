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
	"encoding/binary"
	"io/ioutil"
	"log"
	"math"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
	"google.golang.org/protobuf/proto"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/binarypb_test/kitex_gen/base"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestBinaryProtobufCodec(t *testing.T) {
	bpc := &binaryProtobufCodec{protobufCodec: pbCodec}

	test.Assert(t, bpc.Name() == "RawProtobufBinary", bpc.Name())
	svcInfo := ServiceInfoWithGeneric(BinaryProtobufGeneric())
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation(serviceinfo.GenericMethod, "ExampleMethod"), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())

	// client side
	conn := mocks.NewIOConn()
	bw := bufiox.NewDefaultWriter(conn)
	br := bufiox.NewDefaultReader(conn)
	bb := remote.NewByteBufferFromBufiox(bw, br)
	req := &Args{
		Request: readBasicExampleReqProtoBufDataMetainfo(),
		Method:  "ExampleMethod",
	}
	cliMsg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
	err := bpc.Marshal(context.Background(), cliMsg, bb)
	test.Assert(t, err == nil, err)
	// Check Seq ID
	seqID, err := PbGetSeqID(cliMsg.Data().(*Args).Request.(binaryReqType))
	test.Assert(t, err == nil, err)
	test.Assert(t, seqID == 1, seqID)

	wl := bw.WrittenLen()
	bw.Flush()

	// server side
	svrMsg := remote.NewMessage(&Args{}, svcInfo, ri, remote.Call, remote.Server)
	svrMsg.SetPayloadLen(wl)
	err = bpc.Unmarshal(context.Background(), svrMsg, bb)
	test.Assert(t, err == nil, err)
	// Check Seq ID
	seqID, err = PbGetSeqID(cliMsg.Data().(*Args).Request.(binaryReqType))
	test.Assert(t, err == nil, err)
	test.Assert(t, seqID == 1, seqID)
	reqBuf := svrMsg.Data().(*Args).Request.(binaryReqType)
	pbReqBuf := reqBuf[12+len("ExampleMethod"):]

	// Check if the binary data is unmarshalled correctly
	exp := constructBasicExampleReqObject()
	var act base.BasicExample
	// Unmarshal the binary data into the act struct
	err = proto.Unmarshal(pbReqBuf, &act)
	if err != nil {
		log.Fatalf("Failed to unmarshal Protobuf data: %v", err)
	}
	// compare exp and act structs
	if !proto.Equal(exp, &act) {
		log.Fatalf("Expected and actual Protobuf data are not equal")
	}
}

func TestBinaryProtobufCodecExceptionError(t *testing.T) {
	bpc := &binaryProtobufCodec{protobufCodec: pbCodec}

	conn := mocks.NewIOConn()
	bw := bufiox.NewDefaultWriter(conn)
	br := bufiox.NewDefaultReader(conn)
	bb := remote.NewByteBufferFromBufiox(bw, br)
	svcInfo := ServiceInfoWithGeneric(BinaryProtobufGeneric())
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation(serviceinfo.GenericMethod, "ExampleMethod"), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())

	// test data is empty
	cliMsg := remote.NewMessage(nil, svcInfo, ri, remote.Exception, remote.Client)
	err := bpc.Marshal(context.Background(), cliMsg, bb)
	test.Assert(t, err.Error() == "invalid marshal data in rawProtobufBinaryCodec: nil")

	bw.Flush()

	// empty rpcinfo method
	cliMsg = remote.NewMessage(&remote.TransError{}, svcInfo, newEmptyMethodRPCInfo(), remote.Exception, remote.Server)
	err = bpc.Marshal(context.Background(), cliMsg, bb)
	test.Assert(t, err.Error() == "rawProtobufBinaryCodec Marshal exception failed, err: empty methodName in protobuf Marshal")
}

func readBasicExampleReqProtoBufData() []byte {
	out, err := ioutil.ReadFile("./binarypb_test/data/basicexamplereq_pb.bin")
	if err != nil {
		panic(err)
	}
	return out
}

func readBasicExampleReqProtoBufDataMetainfo() []byte {
	methodName := "ExampleMethod"
	pb := readBasicExampleReqProtoBufData()
	idx := 0
	buf := make([]byte, 12+len(methodName)+len(pb))
	binary.BigEndian.PutUint32(buf, codec.ProtobufV1Magic+uint32(remote.Call))
	idx += 4
	binary.BigEndian.PutUint32(buf[idx:idx+4], uint32(len(methodName)))
	idx += 4
	copy(buf[idx:idx+len(methodName)], methodName)
	idx += len(methodName)
	binary.BigEndian.PutUint32(buf[idx:idx+4], 100)
	idx += 4
	copy(buf[idx:idx+len(pb)], pb)
	return buf
}

func constructBasicExampleReqObject() *base.BasicExample {
	req := &base.BasicExample{
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
