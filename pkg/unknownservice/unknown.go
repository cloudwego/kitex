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

package unknownservice

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	thrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	service2 "github.com/cloudwego/kitex/pkg/unknownservice/service"
)

// UnknownCodec implements PayloadCodec
type unknownCodec struct {
	Codec remote.PayloadCodec
}

// NewUnknownServiceCodec creates the unknown binary codec.
func NewUnknownServiceCodec(code remote.PayloadCodec) remote.PayloadCodec {
	return &unknownCodec{code}
}

// Marshal implements the remote.PayloadCodec interface.
func (c unknownCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	ink := msg.RPCInfo().Invocation()
	data := msg.Data()

	res, ok := data.(*service2.Result)
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
	service, method, err := decode(message, in)
	if err != nil {
		return c.Codec.Unmarshal(ctx, message, in)
	}
	err = codec.SetOrCheckMethodName(method, message)
	var te *remote.TransError
	if errors.As(err, &te) && (te.TypeID() == remote.UnknownMethod || te.TypeID() == remote.UnknownService) {
		svcInfo, err := message.SpecifyServiceInfo(service2.UnknownService, service2.UnknownMethod)
		if err != nil {
			return err
		}

		if ink, ok := ink.(rpcinfo.InvocationSetter); ok {
			ink.SetMethodName(service2.UnknownMethod)
			ink.SetPackageName(svcInfo.GetPackageName())
			ink.SetServiceName(service2.UnknownService)
		} else {
			return errors.New("the interface Invocation doesn't implement InvocationSetter")
		}
		if err = codec.NewDataIfNeeded(service2.UnknownMethod, message); err != nil {
			return err
		}

		data := message.Data()

		if data, ok := data.(*service2.Args); ok {
			data.Method = method
			data.ServiceName = service
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

func decode(message remote.Message, in remote.ByteBuffer) (string, string, error) {
	code := message.ProtocolInfo().CodecType
	if code == serviceinfo.Thrift || code == serviceinfo.Protobuf {
		method, size, err := peekMethod(in)
		if err != nil {
			return "", "", perrors.NewProtocolError(err)
		}
		seqID, err := peekSeqID(in, size)
		if err != nil {
			return "", "", perrors.NewProtocolError(err)
		}
		if err = codec.SetOrCheckSeqID(seqID, message); err != nil {
			return "", "", err
		}
		return message.RPCInfo().Invocation().ServiceName(), method, nil
	}
	return "", "", nil
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

func encode(res *service2.Result, msg remote.Message, out remote.ByteBuffer) error {
	if msg.ProtocolInfo().CodecType == serviceinfo.Thrift {
		return encodeThrift(res, msg, out)
	}
	if msg.ProtocolInfo().CodecType == serviceinfo.Protobuf {
		return encodeKitexProtobuf(res, msg, out)
	}
	return nil
}

// encodeThrift  Thrift encoder
func encodeThrift(res *service2.Result, msg remote.Message, out remote.ByteBuffer) error {
	nw, _ := out.(remote.NocopyWrite)
	msgType := msg.MessageType()
	ink := msg.RPCInfo().Invocation()
	msgBeginLen := bthrift.Binary.MessageBeginLength(res.Method, thrift.TMessageType(msgType), ink.SeqID())
	msgEndLen := bthrift.Binary.MessageEndLength()

	buf, err := out.Malloc(msgBeginLen + len(res.Success) + msgEndLen)
	if err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, Malloc failed: %s", err.Error()))
	}
	offset := bthrift.Binary.WriteMessageBegin(buf, res.Method, thrift.TMessageType(msgType), ink.SeqID())
	write(buf[offset:], res.Success)
	bthrift.Binary.WriteMessageEnd(buf[offset:])
	if nw == nil {
		// if nw is nil, FastWrite will act in Copy mode.
		return nil
	}
	return nw.MallocAck(out.MallocLen())
}

// encodeKitexProtobuf  Kitex Protobuf encoder
func encodeKitexProtobuf(res *service2.Result, msg remote.Message, out remote.ByteBuffer) error {
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
