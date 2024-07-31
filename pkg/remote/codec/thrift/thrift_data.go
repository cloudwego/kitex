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

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/gopkg/protocol/thrift"

	athrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
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
	// TODO(xiaost): Refactor the code after v0.11.0 is released. Unifying checking and fallback logic.

	// encode with hyper codec
	if c.IsSet(FrugalWrite) && hyperMarshalAvailable(data) {
		return c.hyperMarshalBody(data)
	}

	if c.IsSet(FastWrite) {
		if msg, ok := data.(thrift.FastCodec); ok {
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

	// TODO(xiaost): Deprecate the code by using cloudwebgo/gopkg in v0.12.0
	// fallback to old thrift way (slow)
	transport := athrift.NewTMemoryBufferLen(marshalThriftBufferSize)
	tProt := athrift.NewTBinaryProtocol(transport, true, true)
	if err := marshalBasicThriftData(tProt, data); err != nil {
		return nil, err
	}
	return transport.Bytes(), nil
}

// verifyMarshalBasicThriftDataType verifies whether data could be marshaled by old thrift way
func verifyMarshalBasicThriftDataType(data interface{}) error {
	switch data.(type) {
	case MessageWriter:
	default:
		return errEncodeMismatchMsgType
	}
	return nil
}

// marshalBasicThriftData only encodes the data (without the prepending method, msgType, seqId)
// It uses the old thrift way which is much slower than FastCodec and Frugal
func marshalBasicThriftData(tProt athrift.TProtocol, data interface{}) error {
	var err error
	switch msg := data.(type) {
	case MessageWriter:
		err = msg.Write(tProt)
	default:
		return errEncodeMismatchMsgType
	}
	if err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift marshal, Write failed: %s", err.Error()))
	}
	return nil
}

// UnmarshalThriftException decode thrift exception from tProt
// TODO: this func should be removed in the future. it's exposed accidentally.
// Deprecated: Use `SkipDecoder` + `ApplicationException` of `cloudwego/gopkg/protocol/thrift` instead.
func UnmarshalThriftException(tProt athrift.TProtocol) error {
	d := thrift.NewSkipDecoder(tProt.Transport())
	defer d.Release()
	b, err := d.Next(thrift.STRUCT)
	if err != nil {
		return err
	}
	ex := thrift.NewApplicationException(thrift.UNKNOWN_APPLICATION_EXCEPTION, "")
	if _, err := ex.FastRead(b); err != nil {
		return perrors.NewProtocolErrorWithErrMsg(err, fmt.Sprintf("thrift unmarshal Exception failed: %s", err.Error()))
	}
	return remote.NewTransError(ex.TypeId(), ex)
}

// UnmarshalThriftData only decodes the data (after methodName, msgType and seqId)
// It will decode from the given buffer.
// NOTE: `method` is required for generic calls
func UnmarshalThriftData(ctx context.Context, codec remote.PayloadCodec, method string, buf []byte, data interface{}) error {
	c, ok := codec.(*thriftCodec)
	if !ok {
		c = defaultCodec
	}
	tProt := NewBinaryProtocol(remote.NewReaderBuffer(buf))
	err := c.unmarshalThriftData(tProt, data, len(buf))
	if err == nil {
		tProt.Recycle()
	}
	return err
}

func (c thriftCodec) fastMessageUnmarshalAvailable(data interface{}, payloadLen int) bool {
	if payloadLen == 0 && c.CodecType&EnableSkipDecoder == 0 {
		return false
	}
	_, ok := data.(thrift.FastCodec)
	return ok
}

func (c thriftCodec) fastUnmarshal(tProt *BinaryProtocol, data interface{}, dataLen int) error {
	msg := data.(thrift.FastCodec)
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
func (c thriftCodec) unmarshalThriftData(tProt *BinaryProtocol, data interface{}, dataLen int) error {
	// decode with hyper unmarshal
	if c.IsSet(FrugalRead) && c.hyperMessageUnmarshalAvailable(data, dataLen) {
		return c.hyperUnmarshal(tProt, data, dataLen)
	}

	// decode with FastRead
	if c.IsSet(FastRead) && c.fastMessageUnmarshalAvailable(data, dataLen) {
		return c.fastUnmarshal(tProt, data, dataLen)
	}

	if err := verifyUnmarshalBasicThriftDataType(data); err != nil {
		// if user only wants to use Basic we never try fallback to frugal or fastcodec
		if c.CodecType != Basic {
			// try FrugalRead < - > FastRead fallback
			if c.fastMessageUnmarshalAvailable(data, dataLen) {
				return c.fastUnmarshal(tProt, data, dataLen)
			}
			if c.hyperMessageUnmarshalAvailable(data, dataLen) { // slim template?
				return c.hyperUnmarshal(tProt, data, dataLen)
			}
		}
		return err
	}

	// fallback to old thrift way (slow)
	return decodeBasicThriftData(tProt, data)
}

func (c thriftCodec) hyperUnmarshal(tProt *BinaryProtocol, data interface{}, dataLen int) error {
	if dataLen > 0 {
		buf, err := tProt.next(dataLen)
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
	default:
		return errDecodeMismatchMsgType
	}
	return nil
}

// decodeBasicThriftData decode thrift body the old way (slow)
func decodeBasicThriftData(tProt athrift.TProtocol, data interface{}) error {
	var err error
	switch t := data.(type) {
	case MessageReader:
		err = t.Read(tProt)
	default:
		return errDecodeMismatchMsgType
	}
	if err != nil {
		return remote.NewTransError(remote.ProtocolError, err)
	}
	return nil
}

func getSkippedStructBuffer(tProt *BinaryProtocol) ([]byte, error) {
	sd := thrift.NewSkipDecoder(tProt.trans)
	buf, err := sd.Next(thrift.STRUCT)
	if err != nil {
		return nil, remote.NewTransError(remote.ProtocolError, err).AppendMessage("caught in SkipDecoder Next phase")
	}
	return buf, nil
}
