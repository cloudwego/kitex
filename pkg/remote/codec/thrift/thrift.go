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

package thrift

import (
	"context"
	"errors"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"

	internal_stats "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/stats"
)

// CodecType is config of the thrift codec. Priority: Frugal > FastMode > Normal
type CodecType int

const (
	FastWrite CodecType = 1 << iota
	FastRead
)

// NewThriftCodec creates the thrift binary codec.
func NewThriftCodec() remote.PayloadCodec {
	return &thriftCodec{FastWrite | FastRead}
}

// NewThriftFrugalCodec creates the thrift binary codec powered by frugal.
// Eg: xxxservice.NewServer(handler, server.WithPayloadCodec(thrift.NewThriftCodecWithConfig(thrift.FastWrite | thrift.FastRead)))
func NewThriftCodecWithConfig(c CodecType) remote.PayloadCodec {
	return &thriftCodec{c}
}

// NewThriftCodecDisableFastMode creates the thrift binary codec which can control if do fast codec.
// Eg: xxxservice.NewServer(handler, server.WithPayloadCodec(thrift.NewThriftCodecDisableFastMode(true, true)))
func NewThriftCodecDisableFastMode(disableFastWrite, disableFastRead bool) remote.PayloadCodec {
	var c CodecType
	if !disableFastRead {
		c |= FastRead
	}
	if !disableFastWrite {
		c |= FastWrite
	}
	return &thriftCodec{c}
}

// thriftCodec implements PayloadCodec
type thriftCodec struct {
	CodecType
}

// Marshal implements the remote.PayloadCodec interface.
func (c thriftCodec) Marshal(ctx context.Context, message remote.Message, out remote.ByteBuffer) error {
	// 1. prepare info
	methodName := message.RPCInfo().Invocation().MethodName()
	if methodName == "" {
		return errors.New("empty methodName in thrift Marshal")
	}
	data, err := getValidData(methodName, message)
	if err != nil {
		return err
	}

	// 2. encode thrift
	// encode with hyper codec
	// NOTE: to ensure hyperMarshalEnabled is inlined so split the check logic, or it may cause performance loss
	if c.hyperMarshalEnabled() && hyperMarshalAvailable(data) {
		if err := c.hyperMarshal(data, message, out); err != nil {
			return err
		}
		return nil
	}

	msgType := message.MessageType()
	seqID := message.RPCInfo().Invocation().SeqID()
	// encode with FastWrite
	if c.CodecType&FastWrite != 0 {
		if msg, ok := data.(ThriftMsgFastCodec); ok {
			// nocopy write is a special implementation of linked buffer, only bytebuffer implement NocopyWrite do FastWrite
			msgBeginLen := bthrift.Binary.MessageBeginLength(methodName, thrift.TMessageType(msgType), seqID)
			msgEndLen := bthrift.Binary.MessageEndLength()
			buf, err := out.Malloc(msgBeginLen + msg.BLength() + msgEndLen)
			if err != nil {
				return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, Malloc failed: %s", err.Error()))
			}
			offset := bthrift.Binary.WriteMessageBegin(buf, methodName, thrift.TMessageType(msgType), seqID)
			offset += msg.FastWriteNocopy(buf[offset:], nil)
			bthrift.Binary.WriteMessageEnd(buf[offset:])
			return nil
		}
	}

	// encode with normal way
	tProt := NewBinaryProtocol(out)
	if err := tProt.WriteMessageBegin(methodName, thrift.TMessageType(msgType), seqID); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteMessageBegin failed: %s", err.Error()))
	}
	switch msg := data.(type) {
	case MessageWriter:
		if err := msg.Write(tProt); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift marshal, Write failed: %s", err.Error()))
		}
	case MessageWriterWithContext:
		if err := msg.Write(ctx, tProt); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift marshal, Write failed: %s", err.Error()))
		}
	default:
		return remote.NewTransErrorWithMsg(remote.InvalidProtocol, "encode failed, codec msg type not match with thriftCodec")
	}
	if err := tProt.WriteMessageEnd(); err != nil {
		return perrors.NewProtocolErrorWithMsg(fmt.Sprintf("thrift marshal, WriteMessageEnd failed: %s", err.Error()))
	}
	tProt.Recycle()
	return nil
}

// Unmarshal implements the remote.PayloadCodec interface.
func (c thriftCodec) Unmarshal(ctx context.Context, message remote.Message, in remote.ByteBuffer) error {
	tProt := NewBinaryProtocol(in)
	methodName, msgType, seqID, err := tProt.ReadMessageBegin()
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal, ReadMessageBegin failed: %s", err.Error()))
	}
	if err = codec.UpdateMsgType(uint32(msgType), message); err != nil {
		return err
	}
	// exception message
	if message.MessageType() == remote.Exception {
		exception := thrift.NewTApplicationException(thrift.UNKNOWN_APPLICATION_EXCEPTION, "")
		if err := exception.Read(tProt); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal Exception failed: %s", err.Error()))
		}
		if err := tProt.ReadMessageEnd(); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal, ReadMessageEnd failed: %s", err.Error()))
		}
		return remote.NewTransError(exception.TypeId(), exception)
	}
	// Must check after Exception handle.
	// For server side, the following error can be sent back and 'SetSeqID' should be executed first to ensure the seqID
	// is right when return Exception back.
	if err = codec.SetOrCheckSeqID(seqID, message); err != nil {
		return err
	}
	if err = codec.SetOrCheckMethodName(methodName, message); err != nil {
		return err
	}

	if err = codec.NewDataIfNeeded(methodName, message); err != nil {
		return err
	}
	// decode thrift
	data := message.Data()

	// decode with hyper unmarshal
	if c.hyperMessageUnmarshalEnabled() && hyperMessageUnmarshalAvailable(data, message) {
		msgBeginLen := bthrift.Binary.MessageBeginLength(methodName, msgType, seqID)
		ri := message.RPCInfo()
		internal_stats.Record(ctx, ri, stats.WaitReadStart, nil)
		buf, err := tProt.next(message.PayloadLen() - msgBeginLen - bthrift.Binary.MessageEndLength())
		internal_stats.Record(ctx, ri, stats.WaitReadFinish, err)
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		err = c.hyperMessageUnmarshal(buf, data)
		if err != nil {
			return err
		}
		err = tProt.ReadMessageEnd()
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		tProt.Recycle()
		return nil
	}

	// decode with FastRead
	if c.CodecType&FastRead != 0 {
		if msg, ok := data.(ThriftMsgFastCodec); ok && message.PayloadLen() != 0 {
			msgBeginLen := bthrift.Binary.MessageBeginLength(methodName, msgType, seqID)
			ri := message.RPCInfo()
			internal_stats.Record(ctx, ri, stats.WaitReadStart, nil)
			buf, err := tProt.next(message.PayloadLen() - msgBeginLen - bthrift.Binary.MessageEndLength())
			internal_stats.Record(ctx, ri, stats.WaitReadFinish, err)
			if err != nil {
				return remote.NewTransError(remote.ProtocolError, err)
			}
			_, err = msg.FastRead(buf)
			if err != nil {
				return remote.NewTransError(remote.ProtocolError, err)
			}
			err = tProt.ReadMessageEnd()
			if err != nil {
				return remote.NewTransError(remote.ProtocolError, err)
			}
			tProt.Recycle()
			return err
		}
	}

	// decode with normal way
	switch t := data.(type) {
	case MessageReader:
		if err = t.Read(tProt); err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
	case MessageReaderWithMethodWithContext:
		if err = t.Read(ctx, methodName, tProt); err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
	default:
		return remote.NewTransErrorWithMsg(remote.InvalidProtocol, "decode failed, codec msg type not match with thriftCodec")
	}
	tProt.ReadMessageEnd()
	tProt.Recycle()
	return err
}

// Name implements the remote.PayloadCodec interface.
func (c thriftCodec) Name() string {
	return "thrift"
}

// MessageWriterWithContext write to thrift.TProtocol
type MessageWriterWithContext interface {
	Write(ctx context.Context, oprot thrift.TProtocol) error
}

// MessageWriter write to thrift.TProtocol
type MessageWriter interface {
	Write(oprot thrift.TProtocol) error
}

// MessageReader read from thrift.TProtocol
type MessageReader interface {
	Read(oprot thrift.TProtocol) error
}

// MessageReaderWithMethodWithContext read from thrift.TProtocol with method
type MessageReaderWithMethodWithContext interface {
	Read(ctx context.Context, method string, oprot thrift.TProtocol) error
}

type ThriftMsgFastCodec interface {
	BLength() int
	FastWriteNocopy(buf []byte, binaryWriter bthrift.BinaryWriter) int
	FastRead(buf []byte) (int, error)
}

func getValidData(methodName string, message remote.Message) (interface{}, error) {
	if err := codec.NewDataIfNeeded(methodName, message); err != nil {
		return nil, err
	}
	data := message.Data()
	if message.MessageType() != remote.Exception {
		return data, nil
	}
	transErr, isTransErr := data.(*remote.TransError)
	if !isTransErr {
		if err, isError := data.(error); isError {
			encodeErr := thrift.NewTApplicationException(remote.InternalError, err.Error())
			return encodeErr, nil
		}
		return nil, errors.New("exception relay need error type data")
	}
	encodeErr := thrift.NewTApplicationException(transErr.TypeID(), transErr.Error())
	return encodeErr, nil
}
