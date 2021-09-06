/*
 * Copyright 2021 CloudWeGo Authors
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

package codec

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/jackedelic/kitex/internal/mocks"
	"github.com/jackedelic/kitex/internal/pkg/remote"
	"github.com/jackedelic/kitex/internal/pkg/rpcinfo"
	"github.com/jackedelic/kitex/internal/pkg/serviceinfo"
	"github.com/jackedelic/kitex/internal/test"
	"github.com/jackedelic/kitex/internal/transport"
)

func TestThriftProtocolCheck(t *testing.T) {
	var req interface{}
	var rbf remote.ByteBuffer
	var ttheader bool
	var flagBuf []byte
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())
	var msg = remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)

	// 1. ttheader
	flagBuf = make([]byte, 8*2)
	binary.BigEndian.PutUint32(flagBuf, uint32(10))
	binary.BigEndian.PutUint32(flagBuf[4:8], TTHeaderMagic)
	binary.BigEndian.PutUint32(flagBuf[8:12], ThriftV1Magic)
	ttheader = IsTTHeader(flagBuf)
	test.Assert(t, ttheader)
	if ttheader {
		flagBuf = flagBuf[8:]
	}
	rbf = remote.NewReaderBuffer(flagBuf)
	err := checkPayload(flagBuf, msg, rbf, ttheader)
	test.Assert(t, err == nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeader)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)

	// 2. ttheader framed
	flagBuf = make([]byte, 8*2)
	binary.BigEndian.PutUint32(flagBuf, uint32(10))
	binary.BigEndian.PutUint32(flagBuf[4:8], TTHeaderMagic)
	binary.BigEndian.PutUint32(flagBuf[12:], ThriftV1Magic)
	ttheader = IsTTHeader(flagBuf)
	test.Assert(t, ttheader)
	if ttheader {
		flagBuf = flagBuf[8:]
	}
	rbf = remote.NewReaderBuffer(flagBuf)
	err = checkPayload(flagBuf, msg, rbf, ttheader)
	test.Assert(t, err == nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeaderFramed)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)

	// 3. thrift framed
	flagBuf = make([]byte, 8*2)
	binary.BigEndian.PutUint32(flagBuf, uint32(10))
	binary.BigEndian.PutUint32(flagBuf[4:8], ThriftV1Magic)
	ttheader = IsTTHeader(flagBuf)
	test.Assert(t, !ttheader)
	rbf = remote.NewReaderBuffer(flagBuf)
	err = checkPayload(flagBuf, msg, rbf, ttheader)
	test.Assert(t, err == nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.Framed)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)

	// 3. thrift pure payload
	flagBuf = make([]byte, 8*2)
	binary.BigEndian.PutUint32(flagBuf, uint32(10))
	binary.BigEndian.PutUint32(flagBuf[0:4], ThriftV1Magic)
	ttheader = IsTTHeader(flagBuf)
	test.Assert(t, !ttheader)
	rbf = remote.NewReaderBuffer(flagBuf)
	err = checkPayload(flagBuf, msg, rbf, ttheader)
	test.Assert(t, err == nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.PurePayload)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)
}

func TestProtobufProtocolCheck(t *testing.T) {
	var req interface{}
	var rbf remote.ByteBuffer
	var ttheader bool
	var flagBuf []byte
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())
	var msg = remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)

	// 1. ttheader framed
	flagBuf = make([]byte, 8*2)
	binary.BigEndian.PutUint32(flagBuf, uint32(10))
	binary.BigEndian.PutUint32(flagBuf[4:8], TTHeaderMagic)
	binary.BigEndian.PutUint32(flagBuf[12:], ProtobufV1Magic)
	ttheader = IsTTHeader(flagBuf)
	test.Assert(t, ttheader)
	if ttheader {
		flagBuf = flagBuf[8:]
	}
	rbf = remote.NewReaderBuffer(flagBuf)
	err := checkPayload(flagBuf, msg, rbf, ttheader)
	test.Assert(t, err == nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeaderFramed)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Protobuf)

	// 2. protobuf framed
	flagBuf = make([]byte, 8*2)
	binary.BigEndian.PutUint32(flagBuf, uint32(10))
	binary.BigEndian.PutUint32(flagBuf[4:8], ProtobufV1Magic)
	ttheader = IsTTHeader(flagBuf)
	test.Assert(t, !ttheader)
	rbf = remote.NewReaderBuffer(flagBuf)
	err = checkPayload(flagBuf, msg, rbf, ttheader)
	test.Assert(t, err == nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.Framed)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Protobuf)
}

func TestDefaultCodec_Encode_Decode(t *testing.T) {
	remote.PutPayloadCode(serviceinfo.Thrift, mpc)

	dc := NewDefaultCodec()
	ctx := context.Background()
	intKVInfo := prepareIntKVInfo()
	strKVInfo := prepareStrKVInfo()
	sendMsg := initSendMsg(transport.TTHeader)
	sendMsg.TransInfo().PutTransIntInfo(intKVInfo)
	sendMsg.TransInfo().PutTransStrInfo(strKVInfo)

	// encode
	out := remote.NewWriterBuffer(256)
	err := dc.Encode(ctx, sendMsg, out)
	test.Assert(t, err == nil, err)

	// decode
	var recvMsg = initRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil, err)
	in := remote.NewReaderBuffer(buf)
	err = dc.Decode(ctx, recvMsg, in)
	test.Assert(t, err == nil, err)

	intKVInfoRecv := recvMsg.TransInfo().TransIntInfo()
	strKVInfoRecv := recvMsg.TransInfo().TransStrInfo()
	test.DeepEqual(t, intKVInfoRecv, intKVInfo)
	test.DeepEqual(t, strKVInfoRecv, strKVInfo)
	test.Assert(t, sendMsg.RPCInfo().Invocation().SeqID() == recvMsg.RPCInfo().Invocation().SeqID())
}

var mpc remote.PayloadCodec = mockPayloadCodec{}

type mockPayloadCodec struct {
}

func (m mockPayloadCodec) Marshal(ctx context.Context, message remote.Message, out remote.ByteBuffer) error {
	WriteUint32(ThriftV1Magic+uint32(message.MessageType()), out)
	WriteString(message.RPCInfo().Invocation().MethodName(), out)
	WriteUint32(uint32(message.RPCInfo().Invocation().SeqID()), out)
	return nil
}

func (m mockPayloadCodec) Unmarshal(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	magicAndMsgType, err := ReadUint32(in)
	if err != nil {
		return err
	}
	if magicAndMsgType&MagicMask != ThriftV1Magic {
		return errors.New("bad version")
	}
	msgType := magicAndMsgType & FrontMask
	if err := UpdateMsgType(msgType, message); err != nil {
		return err
	}

	methodName, _, err := ReadString(in)
	if err != nil {
		return err
	}
	if err = SetOrCheckMethodName(methodName, message); err != nil && msgType != uint32(remote.Exception) {
		return err
	}
	seqID, err := ReadUint32(in)
	if err != nil {
		return err
	}
	if err = SetOrCheckSeqID(int32(seqID), message); err != nil && msgType != uint32(remote.Exception) {
		return err
	}
	return nil
}

func (m mockPayloadCodec) Name() string {
	return "mock"
}
