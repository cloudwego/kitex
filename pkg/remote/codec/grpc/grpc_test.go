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

package grpc

import (
	"context"
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestNewGRPCCodec(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		g := NewGRPCCodec().(*grpcCodec)
		thriftCodec := g.ThriftCodec
		codec := reflect.ValueOf(thriftCodec).Elem().Field(0).Interface().(thrift.CodecType)
		test.Assert(t, codec == thrift.FastReadWrite)
		test.Assert(t, g.thriftStructCodec != nil)
		test.Assert(t, g.protobufStructCodec != nil)
	})

	t.Run("with-config", func(t *testing.T) {
		codecWithConfig := WithThriftCodec(thrift.NewThriftCodecWithConfig(thrift.FastRead))
		g := NewGRPCCodec(codecWithConfig).(*grpcCodec)
		thriftCodec := g.ThriftCodec
		codec := reflect.ValueOf(thriftCodec).Elem().Field(0).Interface().(thrift.CodecType)
		test.Assert(t, codec == thrift.FastRead)
	})
}

var _ thrift.ThriftMsgFastCodec = (*thriftStruct)(nil)

type thriftStruct struct {
	A int
}

func (t *thriftStruct) BLength() int {
	return 4
}

// FastWriteNocopy not real implementation of thrift encoding, just for test
func (t *thriftStruct) FastWriteNocopy(buf []byte, binaryWriter bthrift.BinaryWriter) int {
	binary.BigEndian.PutUint32(buf, uint32(t.A))
	return 4
}

// FastRead not real implementation of thrift decoding, just for test
func (t *thriftStruct) FastRead(buf []byte) (int, error) {
	t.A = int(binary.BigEndian.Uint32(buf))
	return 4, nil
}

type pbStruct struct {
	A int
}

var _ protobuf.ProtobufV2MsgCodec = (*pbStruct)(nil)

// not real implementation of pb decoding, just for test
func (p *pbStruct) XXX_Unmarshal(b []byte) error {
	p.A = int(binary.BigEndian.Uint32(b))
	return nil
}

// not real implementation of pb encoding, just for test
func (p *pbStruct) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(p.A))
	return buf, nil
}

func Test_grpcCodec_Encode(t *testing.T) {
	t.Run("thrift", func(t *testing.T) {
		g := NewGRPCCodec().(*grpcCodec)
		data := &thriftStruct{A: 1}
		stat := rpcinfo.NewRPCStats()
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		msg := remote.NewMessage(data, nil, ri, remote.Call, remote.Client)
		out := remote.NewWriterBuffer(1024)

		err := g.Encode(context.Background(), msg, out)

		test.Assert(t, err == nil, err)
		buf, err := out.Bytes()
		test.Assert(t, err == nil, err)
		test.Assert(t, len(buf) > 0)
	})

	t.Run("protobuf", func(t *testing.T) {
		g := NewGRPCCodec().(*grpcCodec)
		data := &pbStruct{}
		stat := rpcinfo.NewRPCStats()
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		msg := remote.NewMessage(data, nil, ri, remote.Call, remote.Client)
		msg.SetProtocolInfo(remote.ProtocolInfo{CodecType: serviceinfo.Protobuf})
		out := remote.NewWriterBuffer(1024)

		err := g.Encode(context.Background(), msg, out)

		test.Assert(t, err == nil, err)
		buf, err := out.Bytes()
		test.Assert(t, err == nil, err)
		test.Assert(t, len(buf) > 0, len(buf))
	})
}

func Test_grpcCodec_Decode(t *testing.T) {
	t.Run("thrift", func(t *testing.T) {
		g := NewGRPCCodec().(*grpcCodec)
		buf := make([]byte, 9)
		buf[0] = 0                             // frame: not compressed
		binary.BigEndian.PutUint32(buf[1:], 4) // frame: data size
		binary.BigEndian.PutUint32(buf[5:], 1) // frame: data
		data := &thriftStruct{}
		stat := rpcinfo.NewRPCStats()
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		msg := remote.NewMessage(data, nil, ri, remote.Call, remote.Client)
		in := remote.NewReaderBuffer(buf)

		err := g.Decode(context.Background(), msg, in)

		test.Assert(t, err == nil, err)
		got := msg.Data().(*thriftStruct)
		test.Assert(t, got.A == 1, got)
	})
	t.Run("protobuf", func(t *testing.T) {
		g := NewGRPCCodec().(*grpcCodec)
		buf := make([]byte, 9)
		buf[0] = 0                             // frame: not compressed
		binary.BigEndian.PutUint32(buf[1:], 4) // frame: data size
		binary.BigEndian.PutUint32(buf[5:], 1) // frame: data
		data := &pbStruct{}
		stat := rpcinfo.NewRPCStats()
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		msg := remote.NewMessage(data, nil, ri, remote.Call, remote.Client)
		msg.SetProtocolInfo(remote.ProtocolInfo{CodecType: serviceinfo.Protobuf})
		in := remote.NewReaderBuffer(buf)

		err := g.Decode(context.Background(), msg, in)

		test.Assert(t, err == nil, err)
		got := msg.Data().(*pbStruct)
		test.Assert(t, got.A == 1, got)
	})
}
