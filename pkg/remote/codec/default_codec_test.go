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

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

func TestThriftProtocolCheck(t *testing.T) {
	var req interface{}
	var rbf remote.ByteBuffer
	var ttheader bool
	var flagBuf []byte
	var ri rpcinfo.RPCInfo
	var msg remote.Message

	resetRIAndMSG := func() {
		ri = rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		msg = remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
	}

	// 1. ttheader
	resetRIAndMSG()
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
	err := checkPayload(flagBuf, msg, rbf, ttheader, 10)
	test.Assert(t, err == nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeader)
	test.Assert(t, msg.RPCInfo().Config().TransportProtocol()&transport.TTHeader == transport.TTHeader)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)

	// 2. ttheader framed
	resetRIAndMSG()
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
	err = checkPayload(flagBuf, msg, rbf, ttheader, 10)
	test.Assert(t, err == nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeaderFramed)
	test.Assert(t, msg.RPCInfo().Config().TransportProtocol()&transport.TTHeaderFramed == transport.TTHeaderFramed)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)

	// 3. thrift framed
	resetRIAndMSG()
	flagBuf = make([]byte, 8*2)
	binary.BigEndian.PutUint32(flagBuf, uint32(10))
	binary.BigEndian.PutUint32(flagBuf[4:8], ThriftV1Magic)
	ttheader = IsTTHeader(flagBuf)
	test.Assert(t, !ttheader)
	rbf = remote.NewReaderBuffer(flagBuf)
	err = checkPayload(flagBuf, msg, rbf, ttheader, 10)
	test.Assert(t, err == nil, err)
	err = checkPayload(flagBuf, msg, rbf, ttheader, 9)
	test.Assert(t, err != nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.Framed)
	test.Assert(t, msg.RPCInfo().Config().TransportProtocol()&transport.Framed == transport.Framed)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)

	// 4. thrift pure payload
	// resetRIAndMSG() // the logic below needs to check payload length set by the front case, so we don't reset ri
	flagBuf = make([]byte, 8*2)
	binary.BigEndian.PutUint32(flagBuf, uint32(10))
	binary.BigEndian.PutUint32(flagBuf[0:4], ThriftV1Magic)
	ttheader = IsTTHeader(flagBuf)
	test.Assert(t, !ttheader)
	rbf = remote.NewReaderBuffer(flagBuf)
	err = checkPayload(flagBuf, msg, rbf, ttheader, 10)
	test.Assert(t, err == nil, err)
	err = checkPayload(flagBuf, msg, rbf, ttheader, 9)
	test.Assert(t, err != nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.PurePayload)
	test.Assert(t, msg.RPCInfo().Config().TransportProtocol()&transport.PurePayload == transport.PurePayload)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)
}

func TestProtobufProtocolCheck(t *testing.T) {
	var req interface{}
	var rbf remote.ByteBuffer
	var ttheader bool
	var flagBuf []byte
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)

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
	err := checkPayload(flagBuf, msg, rbf, ttheader, 10)
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
	err = checkPayload(flagBuf, msg, rbf, ttheader, 10)
	test.Assert(t, err == nil, err)
	err = checkPayload(flagBuf, msg, rbf, ttheader, 9)
	test.Assert(t, err != nil, err)
	test.Assert(t, msg.ProtocolInfo().TransProto == transport.Framed)
	test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Protobuf)
}

func TestDefaultCodec_Encode_Decode(t *testing.T) {
	remote.PutPayloadCode(serviceinfo.Thrift, mpc)

	dc := NewDefaultCodec()
	ctx := context.Background()
	intKVInfo := prepareIntKVInfo()
	strKVInfo := prepareStrKVInfo()
	sendMsg := initClientSendMsg(transport.TTHeader)
	sendMsg.TransInfo().PutTransIntInfo(intKVInfo)
	sendMsg.TransInfo().PutTransStrInfo(strKVInfo)

	// encode
	out := remote.NewWriterBuffer(256)
	err := dc.Encode(ctx, sendMsg, out)
	test.Assert(t, err == nil, err)

	// decode
	recvMsg := initServerRecvMsg()
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

func TestDefaultSizedCodec_Encode_Decode(t *testing.T) {
	remote.PutPayloadCode(serviceinfo.Thrift, mpc)

	smallDc := NewDefaultCodecWithSizeLimit(1)
	largeDc := NewDefaultCodecWithSizeLimit(1024)
	ctx := context.Background()
	intKVInfo := prepareIntKVInfo()
	strKVInfo := prepareStrKVInfo()
	sendMsg := initClientSendMsg(transport.TTHeader)
	sendMsg.TransInfo().PutTransIntInfo(intKVInfo)
	sendMsg.TransInfo().PutTransStrInfo(strKVInfo)

	// encode
	smallOut := remote.NewWriterBuffer(256)
	largeOut := remote.NewWriterBuffer(256)
	err := smallDc.Encode(ctx, sendMsg, smallOut)
	test.Assert(t, err != nil, err)
	err = largeDc.Encode(ctx, sendMsg, largeOut)
	test.Assert(t, err == nil, err)

	// decode
	recvMsg := initServerRecvMsg()
	smallBuf, _ := smallOut.Bytes()
	largeBuf, _ := largeOut.Bytes()
	err = smallDc.Decode(ctx, recvMsg, remote.NewReaderBuffer(smallBuf))
	test.Assert(t, err != nil, err)
	err = largeDc.Decode(ctx, recvMsg, remote.NewReaderBuffer(largeBuf))
	test.Assert(t, err == nil, err)
}

func TestCodecTypeNotMatchWithServiceInfoPayloadCodec(t *testing.T) {
	var req interface{}
	remote.PutPayloadCode(serviceinfo.Thrift, mpc)
	remote.PutPayloadCode(serviceinfo.Protobuf, mpc)
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	codec := NewDefaultCodec()

	// case 1: the payloadCodec of svcInfo is Protobuf, CodecType of message is Thrift
	svcInfo := &serviceinfo.ServiceInfo{
		PayloadCodec: serviceinfo.Protobuf,
	}
	msg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Server)
	msg.SetProtocolInfo(remote.ProtocolInfo{TransProto: transport.TTHeader, CodecType: serviceinfo.Thrift})
	err := codec.Encode(context.Background(), msg, remote.NewWriterBuffer(256))
	test.Assert(t, err == nil, err)

	// case 2: the payloadCodec of svcInfo is Thrift, CodecType of message is Protobuf
	svcInfo = &serviceinfo.ServiceInfo{
		PayloadCodec: serviceinfo.Thrift,
	}
	msg = remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Server)
	msg.SetProtocolInfo(remote.ProtocolInfo{TransProto: transport.TTHeader, CodecType: serviceinfo.Protobuf})
	err = codec.Encode(context.Background(), msg, remote.NewWriterBuffer(256))
	test.Assert(t, err != nil)
	msg.SetProtocolInfo(remote.ProtocolInfo{TransProto: transport.Framed, CodecType: serviceinfo.Protobuf})
	err = codec.Encode(context.Background(), msg, remote.NewWriterBuffer(256))
	test.Assert(t, err == nil)
}

var mpc remote.PayloadCodec = mockPayloadCodec{}

type mockPayloadCodec struct{}

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
