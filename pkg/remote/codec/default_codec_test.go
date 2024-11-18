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
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/netpoll"
	"github.com/golang/mock/gomock"

	"github.com/cloudwego/kitex/internal/mocks"
	mocksremote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	netpolltrans "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

var transportBuffers = []struct {
	Name      string
	NewBuffer func() remote.ByteBuffer
}{
	{
		Name: "BytesBuffer",
		NewBuffer: func() remote.ByteBuffer {
			return remote.NewReaderWriterBuffer(1024)
		},
	},
	{
		Name: "NetpollBuffer",
		NewBuffer: func() remote.ByteBuffer {
			return netpolltrans.NewReaderWriterByteBuffer(netpoll.NewLinkBuffer(1024))
		},
	},
}

func TestThriftProtocolCheck(t *testing.T) {
	for _, tb := range transportBuffers {
		t.Run(tb.Name, func(t *testing.T) {
			var req interface{}
			var rbf remote.ByteBuffer
			var isTTheader bool
			var flagBuf []byte
			var ri rpcinfo.RPCInfo
			var msg remote.Message

			resetRIAndMSG := func() {
				ri = rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
				msg = remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
			}

			// 1. isTTheader
			resetRIAndMSG()
			flagBuf = make([]byte, 8*2)
			binary.BigEndian.PutUint32(flagBuf, uint32(10))
			binary.BigEndian.PutUint32(flagBuf[4:8], ttheader.TTHeaderMagic)
			binary.BigEndian.PutUint32(flagBuf[8:12], ThriftV1Magic)
			isTTheader = IsTTHeader(flagBuf)
			test.Assert(t, isTTheader)
			if isTTheader {
				flagBuf = flagBuf[8:]
			}
			rbf = tb.NewBuffer()
			rbf.WriteBinary(flagBuf)
			rbf.Flush()
			err := checkPayload(flagBuf, msg, rbf, isTTheader, 10)
			test.Assert(t, err == nil, err)
			test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeader)
			test.Assert(t, msg.RPCInfo().Config().TransportProtocol()&transport.TTHeader == transport.TTHeader)
			test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)

			// 2. isTTheader framed
			resetRIAndMSG()
			flagBuf = make([]byte, 8*2)
			binary.BigEndian.PutUint32(flagBuf, uint32(10))
			binary.BigEndian.PutUint32(flagBuf[4:8], ttheader.TTHeaderMagic)
			binary.BigEndian.PutUint32(flagBuf[12:], ThriftV1Magic)
			isTTheader = IsTTHeader(flagBuf)
			test.Assert(t, isTTheader)
			if isTTheader {
				flagBuf = flagBuf[8:]
			}
			rbf = tb.NewBuffer()
			rbf.WriteBinary(flagBuf)
			rbf.Flush()
			err = checkPayload(flagBuf, msg, rbf, isTTheader, 10)
			test.Assert(t, err == nil, err)
			test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeaderFramed)
			test.Assert(t, msg.RPCInfo().Config().TransportProtocol()&transport.TTHeaderFramed == transport.TTHeaderFramed)
			test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)

			// 3. thrift framed
			resetRIAndMSG()
			flagBuf = make([]byte, 8*2)
			binary.BigEndian.PutUint32(flagBuf, uint32(10))
			binary.BigEndian.PutUint32(flagBuf[4:8], ThriftV1Magic)
			isTTheader = IsTTHeader(flagBuf)
			test.Assert(t, !isTTheader)
			rbf = tb.NewBuffer()
			rbf.WriteBinary(flagBuf)
			rbf.Flush()
			err = checkPayload(flagBuf, msg, rbf, isTTheader, 10)
			test.Assert(t, err == nil, err)
			err = checkPayload(flagBuf, msg, rbf, isTTheader, 9)
			test.Assert(t, err != nil, err)
			test.Assert(t, msg.ProtocolInfo().TransProto == transport.Framed)
			test.Assert(t, msg.RPCInfo().Config().TransportProtocol()&transport.Framed == transport.Framed)
			test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)

			// 4. thrift pure payload
			// resetRIAndMSG() // the logic below needs to check payload length set by the front case, so we don't reset ri
			flagBuf = make([]byte, 8*2)
			binary.BigEndian.PutUint32(flagBuf, uint32(10))
			binary.BigEndian.PutUint32(flagBuf[0:4], ThriftV1Magic)
			isTTheader = IsTTHeader(flagBuf)
			test.Assert(t, !isTTheader)
			rbf = tb.NewBuffer()
			rbf.WriteBinary(flagBuf)
			rbf.Flush()
			err = checkPayload(flagBuf, msg, rbf, isTTheader, 10)
			test.Assert(t, err == nil, err)
			err = checkPayload(flagBuf, msg, rbf, isTTheader, 9)
			test.Assert(t, err != nil, err)
			test.Assert(t, msg.ProtocolInfo().TransProto == transport.PurePayload)
			test.Assert(t, msg.RPCInfo().Config().TransportProtocol()&transport.PurePayload == transport.PurePayload)
			test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Thrift)
		})
	}
}

func TestProtobufProtocolCheck(t *testing.T) {
	for _, tb := range transportBuffers {
		t.Run(tb.Name, func(t *testing.T) {
			var req interface{}
			var rbf remote.ByteBuffer
			var isTTHeader bool
			var flagBuf []byte
			ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
			msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)

			// 1. isTTHeader framed
			flagBuf = make([]byte, 8*2)
			binary.BigEndian.PutUint32(flagBuf, uint32(10))
			binary.BigEndian.PutUint32(flagBuf[4:8], ttheader.TTHeaderMagic)
			binary.BigEndian.PutUint32(flagBuf[12:], ProtobufV1Magic)
			isTTHeader = IsTTHeader(flagBuf)
			test.Assert(t, isTTHeader)
			if isTTHeader {
				flagBuf = flagBuf[8:]
			}
			rbf = tb.NewBuffer()
			rbf.WriteBinary(flagBuf)
			rbf.Flush()
			err := checkPayload(flagBuf, msg, rbf, isTTHeader, 10)
			test.Assert(t, err == nil, err)
			test.Assert(t, msg.ProtocolInfo().TransProto == transport.TTHeaderFramed)
			test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Protobuf)

			// 2. protobuf framed
			flagBuf = make([]byte, 8*2)
			binary.BigEndian.PutUint32(flagBuf, uint32(10))
			binary.BigEndian.PutUint32(flagBuf[4:8], ProtobufV1Magic)
			isTTHeader = IsTTHeader(flagBuf)
			test.Assert(t, !isTTHeader)
			rbf = tb.NewBuffer()
			rbf.WriteBinary(flagBuf)
			rbf.Flush()
			err = checkPayload(flagBuf, msg, rbf, isTTHeader, 10)
			test.Assert(t, err == nil, err)
			err = checkPayload(flagBuf, msg, rbf, isTTHeader, 9)
			test.Assert(t, err != nil, err)
			test.Assert(t, msg.ProtocolInfo().TransProto == transport.Framed)
			test.Assert(t, msg.ProtocolInfo().CodecType == serviceinfo.Protobuf)
		})
	}
}

func TestDefaultCodec_Encode_Decode(t *testing.T) {
	remote.PutPayloadCode(serviceinfo.Thrift, mpc)

	for _, tb := range transportBuffers {
		t.Run(tb.Name, func(t *testing.T) {
			dc := NewDefaultCodec()
			ctx := context.Background()
			intKVInfo := prepareIntKVInfo()
			strKVInfo := prepareStrKVInfo()
			sendMsg := initClientSendMsg(transport.TTHeader)
			sendMsg.TransInfo().PutTransIntInfo(intKVInfo)
			sendMsg.TransInfo().PutTransStrInfo(strKVInfo)

			// encode
			buf := tb.NewBuffer()
			err := dc.Encode(ctx, sendMsg, buf)
			test.Assert(t, err == nil, err)
			buf.Flush()

			// decode
			recvMsg := initServerRecvMsg()
			test.Assert(t, err == nil, err)
			err = dc.Decode(ctx, recvMsg, buf)
			test.Assert(t, err == nil, err)

			intKVInfoRecv := recvMsg.TransInfo().TransIntInfo()
			strKVInfoRecv := recvMsg.TransInfo().TransStrInfo()
			test.DeepEqual(t, intKVInfoRecv, intKVInfo)
			test.DeepEqual(t, strKVInfoRecv, strKVInfo)
			test.Assert(t, sendMsg.RPCInfo().Invocation().SeqID() == recvMsg.RPCInfo().Invocation().SeqID())
		})
	}
}

func TestDefaultSizedCodec_Encode_Decode(t *testing.T) {
	remote.PutPayloadCode(serviceinfo.Thrift, mpc)

	for _, tb := range transportBuffers {
		t.Run(tb.Name, func(t *testing.T) {
			smallDc := NewDefaultCodecWithSizeLimit(1)
			largeDc := NewDefaultCodecWithSizeLimit(1024)
			ctx := context.Background()
			intKVInfo := prepareIntKVInfo()
			strKVInfo := prepareStrKVInfo()
			sendMsg := initClientSendMsg(transport.TTHeader)
			sendMsg.TransInfo().PutTransIntInfo(intKVInfo)
			sendMsg.TransInfo().PutTransStrInfo(strKVInfo)

			// encode
			smallBuf := tb.NewBuffer()
			largeBuf := tb.NewBuffer()
			err := smallDc.Encode(ctx, sendMsg, smallBuf)
			test.Assert(t, err != nil, err)
			err = largeDc.Encode(ctx, sendMsg, largeBuf)
			test.Assert(t, err == nil, err)
			smallBuf.Flush()
			largeBuf.Flush()

			// decode
			recvMsg := initServerRecvMsg()
			err = smallDc.Decode(ctx, recvMsg, largeBuf)
			test.Assert(t, err != nil, err)
			err = largeDc.Decode(ctx, recvMsg, largeBuf)
			test.Assert(t, err == nil, err)
		})
	}
}

func TestDefaultCodecWithCRC32_Encode_Decode(t *testing.T) {
	remote.PutPayloadCode(serviceinfo.Thrift, mpc)

	dc := NewDefaultCodecWithConfig(CodecConfig{CRC32Check: true})
	ctx := context.Background()
	intKVInfo := prepareIntKVInfo()
	strKVInfo := prepareStrKVInfo()
	payloadLen := 32 * 1024
	sendMsg := initClientSendMsg(transport.TTHeaderFramed, payloadLen)
	sendMsg.TransInfo().PutTransIntInfo(intKVInfo)
	sendMsg.TransInfo().PutTransStrInfo(strKVInfo)

	// test encode err
	badOut := netpolltrans.NewReaderByteBuffer(netpoll.NewLinkBuffer())
	err := dc.Encode(ctx, sendMsg, badOut)
	test.Assert(t, err != nil)

	// encode with netpollBytebuffer
	writer := netpoll.NewLinkBuffer()
	npBuffer := netpolltrans.NewReaderWriterByteBuffer(writer)
	err = dc.Encode(ctx, sendMsg, npBuffer)
	test.Assert(t, err == nil, err)

	// decode, succeed
	recvMsg := initServerRecvMsg()
	buf, err := getWrittenBytes(writer)
	test.Assert(t, err == nil, err)
	in := remote.NewReaderBuffer(buf)
	err = dc.Decode(ctx, recvMsg, in)
	test.Assert(t, err == nil, err)
	intKVInfoRecv := recvMsg.TransInfo().TransIntInfo()
	strKVInfoRecv := recvMsg.TransInfo().TransStrInfo()
	test.DeepEqual(t, intKVInfoRecv, intKVInfo)
	test.DeepEqual(t, strKVInfoRecv, strKVInfo)
	test.Assert(t, sendMsg.RPCInfo().Invocation().SeqID() == recvMsg.RPCInfo().Invocation().SeqID())

	// decode, crc32c check failed
	test.Assert(t, err == nil, err)
	bufLen := len(buf)
	modifiedBuf := make([]byte, bufLen)
	copy(modifiedBuf, buf)
	for i := bufLen - 1; i > bufLen-10; i-- {
		modifiedBuf[i] = 123
	}
	in = remote.NewReaderBuffer(modifiedBuf)
	err = dc.Decode(ctx, recvMsg, in)
	test.Assert(t, err != nil, err)
}

func TestDefaultCodecWithCustomizedValidator(t *testing.T) {
	remote.PutPayloadCode(serviceinfo.Thrift, mpc)

	dc := NewDefaultCodecWithConfig(CodecConfig{PayloadValidator: &mockPayloadValidator{}})
	ctx := context.Background()
	intKVInfo := prepareIntKVInfo()
	strKVInfo := prepareStrKVInfo()
	payloadLen := 32 * 1024
	sendMsg := initClientSendMsg(transport.TTHeaderFramed, payloadLen)
	sendMsg.TransInfo().PutTransIntInfo(intKVInfo)
	sendMsg.TransInfo().PutTransStrInfo(strKVInfo)

	// test encode err
	badOut := netpolltrans.NewReaderByteBuffer(netpoll.NewLinkBuffer())
	err := dc.Encode(ctx, sendMsg, badOut)
	test.Assert(t, err != nil)

	// test encode err because of limit
	exceedLimitCtx := context.WithValue(ctx, mockExceedLimitKey, "true")
	npBuffer := netpolltrans.NewReaderWriterByteBuffer(netpoll.NewLinkBuffer())
	err = dc.Encode(exceedLimitCtx, sendMsg, npBuffer)
	test.Assert(t, err != nil, err)
	test.Assert(t, strings.Contains(err.Error(), "limit"), err)

	// encode with netpollBytebuffer
	writer := netpoll.NewLinkBuffer()
	npBuffer = netpolltrans.NewReaderWriterByteBuffer(writer)
	err = dc.Encode(ctx, sendMsg, npBuffer)
	test.Assert(t, err == nil, err)

	// decode, succeed
	recvMsg := initServerRecvMsg()
	buf, err := getWrittenBytes(writer)
	test.Assert(t, err == nil, err)
	in := remote.NewReaderBuffer(buf)
	err = dc.Decode(ctx, recvMsg, in)
	test.Assert(t, err == nil, err)
	intKVInfoRecv := recvMsg.TransInfo().TransIntInfo()
	strKVInfoRecv := recvMsg.TransInfo().TransStrInfo()
	test.DeepEqual(t, intKVInfoRecv, intKVInfo)
	test.DeepEqual(t, strKVInfoRecv, strKVInfo)
	test.Assert(t, sendMsg.RPCInfo().Invocation().SeqID() == recvMsg.RPCInfo().Invocation().SeqID())

	// decode, check failed
	test.Assert(t, err == nil, err)
	bufLen := len(buf)
	modifiedBuf := make([]byte, bufLen)
	copy(modifiedBuf, buf)
	for i := bufLen - 1; i > bufLen-10; i-- {
		modifiedBuf[i] = 123
	}
	in = remote.NewReaderBuffer(modifiedBuf)
	err = dc.Decode(ctx, recvMsg, in)
	test.Assert(t, err != nil, err)
}

func TestCodecTypeNotMatchWithServiceInfoPayloadCodec(t *testing.T) {
	for _, tb := range transportBuffers {
		t.Run(tb.Name, func(t *testing.T) {
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
			err := codec.Encode(context.Background(), msg, tb.NewBuffer())
			test.Assert(t, err == nil, err)

			// case 2: the payloadCodec of svcInfo is Thrift, CodecType of message is Protobuf
			svcInfo = &serviceinfo.ServiceInfo{
				PayloadCodec: serviceinfo.Thrift,
			}
			msg = remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Server)
			msg.SetProtocolInfo(remote.ProtocolInfo{TransProto: transport.TTHeader, CodecType: serviceinfo.Protobuf})
			err = codec.Encode(context.Background(), msg, tb.NewBuffer())
			test.Assert(t, err != nil)
			msg.SetProtocolInfo(remote.ProtocolInfo{TransProto: transport.Framed, CodecType: serviceinfo.Protobuf})
			err = codec.Encode(context.Background(), msg, tb.NewBuffer())
			test.Assert(t, err == nil)
		})
	}
}

func BenchmarkDefaultEncodeDecode(b *testing.B) {
	ctx := context.Background()
	remote.PutPayloadCode(serviceinfo.Thrift, mpc)
	type factory func() remote.Codec
	testCases := map[string]factory{"normal": NewDefaultCodec, "crc32c": func() remote.Codec { return NewDefaultCodecWithConfig(CodecConfig{CRC32Check: true}) }}

	for name, f := range testCases {
		b.Run(name, func(b *testing.B) {
			msgLen := 1
			for i := 0; i < 6; i++ {
				b.ReportAllocs()
				b.ResetTimer()
				b.Run(fmt.Sprintf("payload-%d", msgLen), func(b *testing.B) {
					for j := 0; j < b.N; j++ {
						codec := f()
						sendMsg := initClientSendMsg(transport.TTHeader, msgLen)
						// encode
						writer := netpoll.NewLinkBuffer()
						out := netpolltrans.NewWriterByteBuffer(writer)
						err := codec.Encode(ctx, sendMsg, out)
						test.Assert(b, err == nil, err)

						// decode
						recvMsg := initServerRecvMsgWithMockMsg()
						buf, err := getWrittenBytes(writer)
						test.Assert(b, err == nil, err)
						in := remote.NewReaderBuffer(buf)
						err = codec.Decode(ctx, recvMsg, in)
						test.Assert(b, err == nil, err)
					}
				})
				msgLen *= 10
			}
		})
	}
}

var mpc remote.PayloadCodec = mockPayloadCodec{}

type mockPayloadCodec struct{}

func (m mockPayloadCodec) Marshal(ctx context.Context, message remote.Message, out remote.ByteBuffer) error {
	WriteUint32(ThriftV1Magic+uint32(message.MessageType()), out)
	WriteString(message.RPCInfo().Invocation().MethodName(), out)
	WriteUint32(uint32(message.RPCInfo().Invocation().SeqID()), out)
	var (
		dataLen uint32
		dataStr string
	)
	// write data
	if data := message.Data(); data != nil {
		if mm, ok := data.(*mockMsg); ok {
			if len(mm.msg) != 0 {
				dataStr = mm.msg
				dataLen = uint32(len(mm.msg))
			}
		}
	}
	WriteUint32(dataLen, out)
	if dataLen > 0 {
		WriteString(dataStr, out)
	}
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
	// read data
	dataLen, err := PeekUint32(in)
	if err != nil {
		return err
	}
	if dataLen == 0 {
		// no data
		return nil
	}
	if _, _, err = ReadString(in); err != nil {
		return err
	}
	return nil
}

func (m mockPayloadCodec) Name() string {
	return "mock"
}

func TestCornerCase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sendMsg := initClientSendMsg(transport.TTHeader)
	sendMsg.SetProtocolInfo(remote.NewProtocolInfo(transport.Framed, serviceinfo.Thrift))

	buffer := mocksremote.NewMockByteBuffer(ctrl)
	buffer.EXPECT().WrittenLen().Return(1024).AnyTimes()
	buffer.EXPECT().Malloc(gomock.Any()).Return(nil, errors.New("error malloc")).AnyTimes()
	err := (&defaultCodec{}).EncodePayload(context.Background(), sendMsg, buffer)
	test.Assert(t, err.Error() == "error malloc")
}
