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

package ttheader_test

// another package to avoid cycle import

import (
	"context"
	"net"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/remote/codec/ttheader"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func newRPCInfo() rpcinfo.RPCInfo {
	fromAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9000")
	toAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:9001")
	from := rpcinfo.NewEndpointInfo("from-svc", "from-method", fromAddr, nil)
	to := rpcinfo.NewEndpointInfo("to-svc", "to-method", toAddr, nil)
	ink := rpcinfo.NewInvocation("idl-service-name", "to-method")
	config := rpcinfo.NewRPCConfig()
	stats := rpcinfo.NewRPCStats()
	return rpcinfo.NewRPCInfo(from, to, ink, config, stats)
}

func init() {
	remote.PutPayloadCode(serviceinfo.Thrift, thrift.NewThriftCodec())
}

type thriftStruct struct {
	N int
}

func Test_streamCodec_Encode(t *testing.T) {
	c := ttheader.NewStreamCodec()
	t.Run("encode-data-failed", func(t *testing.T) {
		msg := remote.NewMessage(&thriftStruct{}, nil, newRPCInfo(), remote.Stream, remote.Server)
		msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeData
		out := remote.NewWriterBuffer(1024)
		err := c.Encode(context.Background(), msg, out)
		test.Assert(t, err != nil, err)
	})
	t.Run("encode-header-failed", func(t *testing.T) {
		msg := remote.NewMessage(&thriftStruct{}, nil, newRPCInfo(), remote.Stream, remote.Server)
		msg.TransInfo().TransStrInfo()["xxx"] = string(make([]byte, 65536)) // longer than TTHeader size limit
		out := remote.NewWriterBuffer(1024)
		err := c.Encode(context.Background(), msg, out)
		test.Assert(t, err != nil, err)
		test.Assert(t, msg.RPCInfo().Stats().LastSendSize() == 0)
	})
}

// todo(DMwangnima): use cloudwego/gopkg to replace this bthrift package usage
//func Test_streamCodec_Decode(t *testing.T) {
//	c := ttheader.NewStreamCodec()
//	t.Run("decode-header-failed", func(t *testing.T) {
//		msg := remote.NewMessage(&thriftStruct{}, nil, newRPCInfo(), remote.Stream, remote.Server)
//		in := remote.NewReaderBuffer(nil)
//		err := c.Decode(context.Background(), msg, in)
//		test.Assert(t, err != nil, err)
//	})
//	t.Run("decode-non-data-frame", func(t *testing.T) {
//		wMsg := remote.NewMessage(nil, nil, newRPCInfo(), remote.Stream, remote.Server)
//		wMsg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
//		out := remote.NewWriterBuffer(1024)
//		err := c.Encode(context.Background(), wMsg, out)
//		test.Assert(t, err == nil, err)
//		test.Assert(t, wMsg.RPCInfo().Stats().LastSendSize() == 0)
//
//		rMsg := remote.NewMessage(&thriftStruct{}, nil, newRPCInfo(), remote.Stream, remote.Server)
//		buf, _ := out.Bytes()
//		in := remote.NewReaderBuffer(buf)
//		err = c.Decode(context.Background(), rMsg, in)
//		test.Assert(t, err == nil, err)
//		test.Assert(t, rMsg.RPCInfo().Stats().LastRecvSize() == 0)
//		test.Assert(t, rMsg.TransInfo().TransIntInfo()[transmeta.FrameType] == codec.FrameTypeHeader)
//	})
//
//	t.Run("decode-data-frame", func(t *testing.T) {
//		wMsg := remote.NewMessage(&ftest.Local{L: 1}, nil, newRPCInfo(), remote.Stream, remote.Server)
//		wMsg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeData
//		out := remote.NewWriterBuffer(1024)
//		err := c.Encode(context.Background(), wMsg, out)
//		test.Assert(t, err == nil, err)
//		test.Assert(t, wMsg.RPCInfo().Stats().LastSendSize() > 0)
//
//		rData := &ftest.Local{}
//		rMsg := remote.NewMessage(rData, nil, newRPCInfo(), remote.Stream, remote.Server)
//		buf, _ := out.Bytes()
//		in := remote.NewReaderBuffer(buf)
//		err = c.Decode(context.Background(), rMsg, in)
//		test.Assert(t, err == nil, err)
//		test.Assert(t, rMsg.RPCInfo().Stats().LastRecvSize() > 0)
//		test.Assert(t, rData.L == 1, rData)
//		test.Assert(t, rMsg.TransInfo().TransIntInfo()[transmeta.FrameType] == codec.FrameTypeData)
//	})
//
//	t.Run("decode-data-frame-skip-payload", func(t *testing.T) {
//		wMsg := remote.NewMessage(&ftest.Local{L: 1}, nil, newRPCInfo(), remote.Stream, remote.Server)
//		wMsg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeData
//		out := remote.NewWriterBuffer(1024)
//		err := c.Encode(context.Background(), wMsg, out)
//		test.Assert(t, err == nil, err)
//		test.Assert(t, wMsg.RPCInfo().Stats().LastSendSize() > 0)
//
//		rMsg := remote.NewMessage(nil, nil, newRPCInfo(), remote.Stream, remote.Server)
//		buf, _ := out.Bytes()
//		in := remote.NewReaderBuffer(buf)
//		err = c.Decode(context.Background(), rMsg, in)
//		test.Assert(t, err == nil, err)
//		test.Assert(t, rMsg.RPCInfo().Stats().LastRecvSize() == 0)
//		test.Assert(t, rMsg.TransInfo().TransIntInfo()[transmeta.FrameType] == codec.FrameTypeData)
//	})
//
//	t.Run("decode-data-frame-payload-short-read", func(t *testing.T) {
//		wMsg := remote.NewMessage(&ftest.Local{L: 1}, nil, newRPCInfo(), remote.Stream, remote.Server)
//		wMsg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeData
//		out := remote.NewWriterBuffer(1024)
//		err := c.Encode(context.Background(), wMsg, out)
//		test.Assert(t, err == nil, err)
//		test.Assert(t, wMsg.RPCInfo().Stats().LastSendSize() > 0)
//
//		rMsg := remote.NewMessage(nil, nil, newRPCInfo(), remote.Stream, remote.Server)
//		buf, _ := out.Bytes()
//		in := remote.NewReaderBuffer(buf[0 : len(buf)-1])
//		err = c.Decode(context.Background(), rMsg, in)
//		test.Assert(t, err != nil, err)
//	})
//}
