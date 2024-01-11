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

	// fallback to old thrift way (slow)
	transport := thrift.NewTMemoryBufferLen(marshalThriftBufferSize)
	tProt := thrift.NewTBinaryProtocol(transport, true, true)
	if err := marshalBasicThriftData(ctx, tProt, data); err != nil {
		return nil, err
	}
	return transport.Bytes(), nil
}

// marshalBasicThriftData only encodes the data (without the prepending method, msgType, seqId)
// It uses the old thrift way which is much slower than FastCodec and Frugal
func marshalBasicThriftData(ctx context.Context, tProt thrift.TProtocol, data interface{}) error {
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
	err := c.unmarshalThriftData(ctx, tProt, method, data, len(buf))
	if err == nil {
		tProt.Recycle()
	}
	return err
}

// unmarshalThriftData only decodes the data (after methodName, msgType and seqId)
// method is only used for generic calls
func (c thriftCodec) unmarshalThriftData(ctx context.Context, tProt *BinaryProtocol, method string, data interface{}, dataLen int) error {
	// decode with hyper unmarshal
	if c.hyperMessageUnmarshalEnabled() && hyperMessageUnmarshalAvailable(data, dataLen) {
		buf, err := tProt.next(dataLen - bthrift.Binary.MessageEndLength())
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		return c.hyperMessageUnmarshal(buf, data)
	}

	// decode with FastRead
	if c.CodecType&FastRead != 0 {
		if msg, ok := data.(ThriftMsgFastCodec); ok && dataLen > 0 {
			buf, err := tProt.next(dataLen)
			if err != nil {
				return remote.NewTransError(remote.ProtocolError, err)
			}
			_, err = msg.FastRead(buf)
			return err
		}
	}

	// fallback to old thrift way (slow)
	return decodeBasicThriftData(ctx, tProt, method, data)
}

// decodeBasicThriftData decode thrift body the old way (slow)
func decodeBasicThriftData(ctx context.Context, tProt thrift.TProtocol, method string, data interface{}) error {
	var err error
	switch t := data.(type) {
	case MessageReader:
		if err = t.Read(tProt); err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
	case MessageReaderWithMethodWithContext:
		// methodName is necessary for generic calls to methodInfo from serviceInfo
		if err = t.Read(ctx, method, tProt); err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
	default:
		return remote.NewTransErrorWithMsg(remote.InvalidProtocol,
			"decode failed, codec msg type not match with thriftCodec")
	}
	return nil
}
