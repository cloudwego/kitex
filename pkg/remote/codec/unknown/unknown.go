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

package unknown

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	thrif "github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/dynamicgo/thrift"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	unknowns "github.com/cloudwego/kitex/pkg/remote/codec/unknown/service"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// UnknownCodec implements PayloadCodec
type unknownCodec struct {
	Codec remote.PayloadCodec
}

// NewUnknownCodec creates the unknown binary codec.
func NewUnknownCodec(code remote.PayloadCodec) remote.PayloadCodec {
	return &unknownCodec{code}
}

// Marshal implements the remote.PayloadCodec interface.
func (c unknownCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	ink := msg.RPCInfo().Invocation()
	data := msg.Data()

	res, ok := data.(*unknowns.Result)
	if !ok {
		return c.Codec.Marshal(ctx, msg, out)
	}
	if len(res.Success) == 0 {
		return errors.New("unknown messages cannot be empty")
	}
	if msg.MessageType() == remote.Exception {
		return c.Codec.Marshal(ctx, msg, out)
	}
	if ink, ok := ink.(rpcinfo.InvocationSetter); ok {
		ink.SetMethodName(res.Method)
		ink.SetServiceName(res.ServiceName)
	} else {
		return errors.New("the interface Invocation doesn't implement InvocationSetter")
	}
	if err := encode(res, msg, out); err != nil {
		return c.Codec.Marshal(ctx, msg, out)
	}
	return nil
}

// Unmarshal implements the remote.PayloadCodec interface.
func (c unknownCodec) Unmarshal(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	ink := message.RPCInfo().Invocation()
	service, method, size, err := decode(message, in)
	if err != nil {
		return c.Codec.Unmarshal(ctx, message, in)
	}
	err = codec.SetOrCheckMethodName(method, message)
	if te, ok := err.(*remote.TransError); ok && te.TypeID() == remote.UnknownMethod {
		svcInfo, err := message.SpecifyServiceInfo(unknowns.UnknownService, unknowns.UnknownMethod)
		if err != nil {
			return err
		}

		if ink, ok := ink.(rpcinfo.InvocationSetter); ok {
			ink.SetMethodName(unknowns.UnknownMethod)
			ink.SetPackageName(svcInfo.GetPackageName())
			ink.SetServiceName(unknowns.UnknownService)
		} else {
			return errors.New("the interface Invocation doesn't implement InvocationSetter")
		}
		if err = codec.NewDataIfNeeded(unknowns.UnknownMethod, message); err != nil {
			return err
		}

		data := message.Data()

		if data, ok := data.(*unknowns.Args); ok {
			data.Method = method
			data.ServiceName = service
			err := in.Skip(int(size))
			if err != nil {
				return err
			}
			buf, err := in.Next(in.ReadableLen())
			if err != nil {
				return err
			}
			data.Request = buf
		}
		return nil
	}

	return c.Codec.Unmarshal(ctx, message, in)
}

// Name implements the remote.PayloadCodec interface.
func (c unknownCodec) Name() string {
	return "unknownMethodCodec"
}

func write(dst, src []byte) {
	copy(dst, src)
}

func decode(message remote.Message, in remote.ByteBuffer) (string, string, int32, error) {
	code := message.ProtocolInfo().CodecType
	if code == serviceinfo.Thrift {
		return decodeThrift(message, in)
	} else if code == serviceinfo.Protobuf {
		return decodeProtobuf(message, in)
	}
	return "", "", 0, nil
}

// decodeThrift  Thrift decoder
func decodeThrift(message remote.Message, in remote.ByteBuffer) (string, string, int32, error) {
	buf, err := in.Peek(4)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	size := int32(binary.BigEndian.Uint32(buf))
	if size > 0 {
		return "", "", 0, perrors.NewProtocolErrorWithType(perrors.BadVersion, "Missing version in ReadMessageBegin")
	}
	msgType := thrift.TMessageType(size & 0x0ff)
	if err = codec.UpdateMsgType(uint32(msgType), message); err != nil {
		return "", "", 0, err
	}
	version := int64(int64(size) & thrift.VERSION_MASK)
	if version != thrift.VERSION_1 {
		return "", "", 0, perrors.NewProtocolErrorWithType(perrors.BadVersion, "Bad version in ReadMessageBegin")
	}
	// exception message
	if message.MessageType() == remote.Exception {
		return "", "", 0, perrors.NewProtocolErrorWithMsg("thrift unmarshal")
	}
	// 获取method
	method, size, err := peekMethod(in)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	seqID, err := peekSeqID(in, size)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	if err = codec.SetOrCheckSeqID(seqID, message); err != nil {
		return "", "", 0, err
	}
	return message.RPCInfo().Invocation().ServiceName(), method, size + 4, nil
}

// decodeProtobuf  Protobuf decoder
func decodeProtobuf(message remote.Message, in remote.ByteBuffer) (string, string, int32, error) {
	magicAndMsgType, err := codec.PeekUint32(in)
	if err != nil {
		return "", "", 0, err
	}
	if magicAndMsgType&codec.MagicMask != codec.ProtobufV1Magic {
		return "", "", 0, perrors.NewProtocolErrorWithType(perrors.BadVersion, "Bad version in protobuf Unmarshal")
	}
	msgType := magicAndMsgType & codec.FrontMask
	if err = codec.UpdateMsgType(msgType, message); err != nil {
		return "", "", 0, err
	}

	method, size, err := peekMethod(in)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	seqID, err := peekSeqID(in, size)
	if err != nil {
		return "", "", 0, perrors.NewProtocolError(err)
	}
	if err = codec.SetOrCheckSeqID(seqID, message); err != nil && msgType != uint32(remote.Exception) {
		return "", "", 0, err
	}
	// exception message
	if message.MessageType() == remote.Exception {
		return "", "", 0, perrors.NewProtocolErrorWithMsg("protobuf unmarshal")
	}
	return message.RPCInfo().Invocation().ServiceName(), method, size + 4, nil
}

func peekMethod(in remote.ByteBuffer) (string, int32, error) {
	buf, err := in.Peek(8)
	if err != nil {
		return "", 0, err
	}
	buf = buf[4:]
	size := int32(binary.BigEndian.Uint32(buf))
	buf, err = in.Peek(int(size + 8))
	if err != nil {
		return "", 0, perrors.NewProtocolError(err)
	}
	buf = buf[8:]
	method := string(buf)
	return method, size + 8, nil
}

func peekSeqID(in remote.ByteBuffer, size int32) (int32, error) {
	buf, err := in.Peek(int(size + 4))
	if err != nil {
		return 0, perrors.NewProtocolError(err)
	}
	buf = buf[size:]
	seqID := int32(binary.BigEndian.Uint32(buf))
	return seqID, nil
}

func encode(res *unknowns.Result, msg remote.Message, out remote.ByteBuffer) error {

	if msg.ProtocolInfo().CodecType == serviceinfo.Thrift {
		return encodeThrift(res, msg, out)
	} else if msg.ProtocolInfo().CodecType == serviceinfo.Protobuf {
		return encodeProtobuf(res, msg, out)
	}
	return nil
}

// encodeThrift  Thrift encoder
func encodeThrift(res *unknowns.Result, msg remote.Message, out remote.ByteBuffer) error {
	nw, _ := out.(remote.NocopyWrite)
	msgType := msg.MessageType()
	ink := msg.RPCInfo().Invocation()
	msgBeginLen := bthrift.Binary.MessageBeginLength(res.Method, thrif.TMessageType(msgType), ink.SeqID())
	msgEndLen := bthrift.Binary.MessageEndLength()

	buf, err := out.Malloc(msgBeginLen + len(res.Success) + msgEndLen)
	if err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, Malloc failed: %s", err.Error()))
	}
	offset := bthrift.Binary.WriteMessageBegin(buf, res.Method, thrif.TMessageType(msgType), ink.SeqID())
	write(buf[offset:], res.Success)
	bthrift.Binary.WriteMessageEnd(buf[offset:])
	if nw == nil {
		// if nw is nil, FastWrite will act in Copy mode.
		return nil
	}
	return nw.MallocAck(out.MallocLen())
}

// encodeProtobuf  Protobuf encoder
func encodeProtobuf(res *unknowns.Result, msg remote.Message, out remote.ByteBuffer) error {
	ink := msg.RPCInfo().Invocation()
	// 3.1 magic && msgType
	if err := codec.WriteUint32(codec.ProtobufV1Magic+uint32(msg.MessageType()), out); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("protobuf marshal, write meta info failed: %s", err.Error()))
	}
	// 3.2 methodName
	if _, err := codec.WriteString(res.Method, out); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("protobuf marshal, write method name failed: %s", err.Error()))
	}
	// 3.3 seqID
	if err := codec.WriteUint32(uint32(ink.SeqID()), out); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("protobuf marshal, write seqID failed: %s", err.Error()))
	}
	dataLen := len(res.Success)
	buf, err := out.Malloc(dataLen)
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("protobuf malloc size %d failed: %s", dataLen, err.Error()))
	}
	write(buf, res.Success)
	return nil
}
