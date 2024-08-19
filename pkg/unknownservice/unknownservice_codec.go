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

package unknownservice

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	gthrift "github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	unknownservice "github.com/cloudwego/kitex/pkg/unknownservice/service"
)

// UnknownCodec implements PayloadCodec
type unknownServiceCodec struct {
	Codec remote.PayloadCodec
}

// NewUnknownServiceCodec creates the unknown binary codec.
func NewUnknownServiceCodec(code remote.PayloadCodec) remote.PayloadCodec {
	return &unknownServiceCodec{code}
}

// Marshal implements the remote.PayloadCodec interface.
func (c unknownServiceCodec) Marshal(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
	ink := msg.RPCInfo().Invocation()
	data := msg.Data()

	res, ok := data.(*unknownservice.Result)
	if !ok {
		return c.Codec.Marshal(ctx, msg, out)
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

	if res.Success == nil {
		sz := gthrift.Binary.MessageBeginLength(msg.RPCInfo().Invocation().MethodName())
		if msg.ProtocolInfo().CodecType == serviceinfo.Thrift {
			sz += gthrift.Binary.FieldStopLength()
			buf, err := out.Malloc(sz)
			if err != nil {
				return perrors.NewProtocolError(fmt.Errorf("binary thrift generic marshal, remote.ByteBuffer Malloc err: %w", err))
			}
			buf = gthrift.Binary.AppendMessageBegin(buf[:0],
				msg.RPCInfo().Invocation().MethodName(), gthrift.TMessageType(msg.MessageType()), msg.RPCInfo().Invocation().SeqID())
			buf = gthrift.Binary.AppendFieldStop(buf)
			_ = buf
		}

		if msg.ProtocolInfo().CodecType == serviceinfo.Protobuf {
			buf, err := out.Malloc(sz)
			if err != nil {
				return perrors.NewProtocolError(fmt.Errorf("binary thrift generic marshal, remote.ByteBuffer Malloc err: %w", err))
			}
			binary.BigEndian.PutUint32(buf, codec.ProtobufV1Magic+uint32(msg.MessageType()))
			offset := 4
			offset += gthrift.Binary.WriteString(buf[offset:], res.Method)
			offset += gthrift.Binary.WriteI32(buf[offset:], msg.RPCInfo().Invocation().SeqID())
			_ = buf
		}
		return nil
	}
	out.WriteBinary(res.Success)
	return nil
}

// Unmarshal implements the remote.PayloadCodec interface.
func (c unknownServiceCodec) Unmarshal(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	ink := message.RPCInfo().Invocation()
	magicAndMsgType, err := codec.PeekUint32(in)
	if err != nil {
		return err
	}
	msgType := magicAndMsgType & codec.FrontMask
	if msgType == uint32(remote.Exception) {
		return c.Codec.Unmarshal(ctx, message, in)
	}
	if err = codec.UpdateMsgType(msgType, message); err != nil {
		return err
	}
	service, method, err := readDecode(message, in)
	if err != nil {
		return err
	}
	err = codec.SetOrCheckMethodName(method, message)
	var te *remote.TransError
	if errors.As(err, &te) && (te.TypeID() == remote.UnknownMethod || te.TypeID() == remote.UnknownService) {
		svcInfo, err := message.SpecifyServiceInfo(unknownservice.UnknownService, unknownservice.UnknownMethod)
		if err != nil {
			return err
		}

		if ink, ok := ink.(rpcinfo.InvocationSetter); ok {
			ink.SetMethodName(unknownservice.UnknownMethod)
			ink.SetPackageName(svcInfo.GetPackageName())
			ink.SetServiceName(unknownservice.UnknownService)
		} else {
			return errors.New("the interface Invocation doesn't implement InvocationSetter")
		}
		if err = codec.NewDataIfNeeded(unknownservice.UnknownMethod, message); err != nil {
			return err
		}

		data := message.Data()

		if data, ok := data.(*unknownservice.Args); ok {
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
func (c unknownServiceCodec) Name() string {
	return "unknownServiceCodec"
}

func readDecode(message remote.Message, in remote.ByteBuffer) (string, string, error) {
	code := message.ProtocolInfo().CodecType
	if code == serviceinfo.Thrift || code == serviceinfo.Protobuf {
		method, size, err := peekMethod(in)
		if err != nil {
			return "", "", err
		}

		seqID, err := peekSeqID(in, size)
		if err != nil {
			return "", "", err
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
