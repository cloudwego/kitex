/*
 * Copyright 2023 CloudWeGo Authors
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
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/bytedance/gopkg/lang/mcache"

	"github.com/cloudwego/kitex/pkg/protocol/bthrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
)

const marshalThriftBufferSize = 1024

// MarshalThriftData only encodes the data (without the prepending methodName, msgType, seqId)
// It will allocate a new buffer and encode to it
func MarshalThriftData(ctx context.Context, codec remote.PayloadCodec, data interface{}) ([]byte, error) {
	c, ok := codec.(*thriftCodec)
	if !ok {
		c = defaultCodec
	}
	return c.marshalThriftData(ctx, data)
}

// marshalBasicThriftData only encodes the data (without the prepending method, msgType, seqId)
// It will allocate a new buffer and encode to it
func (c thriftCodec) marshalThriftData(ctx context.Context, data interface{}) ([]byte, error) {
	// encode with hyper codec
	// NOTE: to ensure hyperMarshalEnabled is inlined so split the check logic, or it may cause performance loss
	if c.hyperMarshalEnabled() && hyperMarshalAvailable(data) {
		return c.hyperMarshalBody(data)
	}

	// encode with FastWrite
	if c.CodecType&FastWrite != 0 {
		if msg, ok := data.(ThriftMsgFastCodec); ok {
			payloadSize := msg.BLength()
			payload := mcache.Malloc(payloadSize)
			msg.FastWriteNocopy(payload, nil)
			return payload, nil
		}
	}

	if err := verifyMarshalBasicThriftDataType(data); err != nil {
		// Basic can be used for disabling frugal, we need to check it
		if c.CodecType != Basic && hyperMarshalAvailable(data) {
			// fallback to frugal when the generated code is using slim template
			return c.hyperMarshalBody(data)
		}
		return nil, err
	}

	// fallback to old thrift way (slow)
	transport := thrift.NewTMemoryBufferLen(marshalThriftBufferSize)
	tProt := thrift.NewTBinaryProtocol(transport, true, true)
	if err := marshalBasicThriftData(ctx, tProt, data, "", -1); err != nil {
		return nil, err
	}
	return transport.Bytes(), nil
}

// verifyMarshalBasicThriftDataType verifies whether data could be marshaled by old thrift way
func verifyMarshalBasicThriftDataType(data interface{}) error {
	switch data.(type) {
	case MessageWriter:
	case MessageWriterWithContext:
	default:
		return errEncodeMismatchMsgType
	}
	return nil
}

// marshalBasicThriftData only encodes the data (without the prepending method, msgType, seqId)
// It uses the old thrift way which is much slower than FastCodec and Frugal
func marshalBasicThriftData(ctx context.Context, tProt thrift.TProtocol, data interface{}, method string, rpcRole remote.RPCRole) error {
	switch msg := data.(type) {
	case MessageWriter:
		if err := msg.Write(tProt); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift marshal, Write failed: %s", err.Error()))
		}
	case MessageWriterWithContext:
		if err := msg.Write(ctx, method, rpcRole == remote.Client, tProt); err != nil {
			return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift marshal, Write failed: %s", err.Error()))
		}
	default:
		return errEncodeMismatchMsgType
	}
	return nil
}

// UnmarshalThriftException decode thrift exception from tProt
// If your input is []byte, you can wrap it with `NewBinaryProtocol(remote.NewReaderBuffer(buf))`
func UnmarshalThriftException(tProt thrift.TProtocol) error {
	exception := thrift.NewTApplicationException(thrift.UNKNOWN_APPLICATION_EXCEPTION, "")
	if err := exception.Read(tProt); err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal Exception failed: %s", err.Error()))
	}
	if err := tProt.ReadMessageEnd(); err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal, ReadMessageEnd failed: %s", err.Error()))
	}
	return remote.NewTransError(exception.TypeId(), exception)
}

// UnmarshalThriftData only decodes the data (after methodName, msgType and seqId)
// It will decode from the given buffer.
// Note:
// 1. `method` is only used for generic calls
// 2. if the buf contains an exception, you should call UnmarshalThriftException instead.
func UnmarshalThriftData(ctx context.Context, codec remote.PayloadCodec, method string, buf []byte, data interface{}) error {
	c, ok := codec.(*thriftCodec)
	if !ok {
		c = defaultCodec
	}
	tProt := NewBinaryProtocol(remote.NewReaderBuffer(buf))
	err := c.unmarshalThriftData(ctx, tProt, method, data, -1, len(buf))
	if err == nil {
		tProt.Recycle()
	}
	return err
}

func (c thriftCodec) fastMessageUnmarshalEnabled() bool {
	return c.CodecType&FastRead != 0
}

func (c thriftCodec) fastMessageUnmarshalAvailable(data interface{}, payloadLen int) bool {
	if payloadLen == 0 && c.CodecType&EnableSkipDecoder == 0 {
		return false
	}
	_, ok := data.(ThriftMsgFastCodec)
	return ok
}

func (c thriftCodec) fastUnmarshal(tProt *BinaryProtocol, data interface{}, dataLen int) error {
	msg := data.(ThriftMsgFastCodec)
	if dataLen > 0 {
		buf, err := tProt.next(dataLen)
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		_, err = msg.FastRead(buf)
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		return nil
	}
	buf, err := getSkippedStructBuffer(tProt)
	if err != nil {
		return err
	}
	_, err = msg.FastRead(buf)
	if err != nil {
		return remote.NewTransError(remote.ProtocolError, err).AppendMessage("caught in FastCodec using SkipDecoder Buffer")
	}
	return err
}

// unmarshalThriftData only decodes the data (after methodName, msgType and seqId)
// method is only used for generic calls
func (c thriftCodec) unmarshalThriftData(ctx context.Context, tProt *BinaryProtocol, method string, data interface{}, rpcRole remote.RPCRole, dataLen int) error {
	// decode with hyper unmarshal
	if c.hyperMessageUnmarshalEnabled() && c.hyperMessageUnmarshalAvailable(data, dataLen) {
		return c.hyperUnmarshal(tProt, data, dataLen)
	}

	// decode with FastRead
	if c.fastMessageUnmarshalEnabled() && c.fastMessageUnmarshalAvailable(data, dataLen) {
		return c.fastUnmarshal(tProt, data, dataLen)
	}

	if err := verifyUnmarshalBasicThriftDataType(data); err != nil {
		// Basic can be used for disabling frugal, we need to check it
		if c.CodecType != Basic && c.hyperMessageUnmarshalAvailable(data, dataLen) {
			// fallback to frugal when the generated code is using slim template
			return c.hyperUnmarshal(tProt, data, dataLen)
		}
		return err
	}

	// fallback to old thrift way (slow)
	return decodeBasicThriftData(ctx, tProt, method, rpcRole, dataLen, data)
}

func (c thriftCodec) hyperUnmarshal(tProt *BinaryProtocol, data interface{}, dataLen int) error {
	if dataLen > 0 {
		buf, err := tProt.next(dataLen - bthrift.Binary.MessageEndLength())
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		if err = c.hyperMessageUnmarshal(buf, data); err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		return nil
	}
	buf, err := getSkippedStructBuffer(tProt)
	if err != nil {
		return err
	}
	if err = c.hyperMessageUnmarshal(buf, data); err != nil {
		return remote.NewTransError(remote.ProtocolError, err).AppendMessage("caught in Frugal using SkipDecoder Buffer")
	}

	return nil
}

// verifyUnmarshalBasicThriftDataType verifies whether data could be unmarshal by old thrift way
func verifyUnmarshalBasicThriftDataType(data interface{}) error {
	switch data.(type) {
	case MessageReader:
	case MessageReaderWithMethodWithContext:
	default:
		return errDecodeMismatchMsgType
	}
	return nil
}

// decodeBasicThriftData decode thrift body the old way (slow)
func decodeBasicThriftData(ctx context.Context, tProt thrift.TProtocol, method string, rpcRole remote.RPCRole, dataLen int, data interface{}) error {
	var err error
	switch t := data.(type) {
	case MessageReader:
		if err = t.Read(tProt); err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
	case MessageReaderWithMethodWithContext:
		// methodName is necessary for generic calls to methodInfo from serviceInfo
		if err = t.Read(ctx, method, rpcRole == remote.Client, dataLen, tProt); err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
	default:
		return errDecodeMismatchMsgType
	}
	return nil
}

func getSkippedStructBuffer(tProt *BinaryProtocol) ([]byte, error) {
	sd := skipDecoder{ByteBuffer: tProt.trans}
	buf, err := sd.NextStruct()
	if err != nil {
		return nil, remote.NewTransError(remote.ProtocolError, err).AppendMessage("caught in SkipDecoder NextStruct phase")
	}
	return buf, nil
}
