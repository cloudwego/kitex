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
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/bytedance/gopkg/lang/mcache"
	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/thrift/apache"

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

	// fallback to old thrift way (slow)
	buf := bytes.NewBuffer(make([]byte, 0, marshalThriftBufferSize))
	if err := apache.ThriftWrite(buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// verifyMarshalBasicThriftDataType verifies whether data could be marshaled by old thrift way
func verifyMarshalBasicThriftDataType(data interface{}) error {
	if err := apache.CheckTStruct(data); err != nil {
		return errEncodeMismatchMsgType
	}
	return nil
}

func unmarshalThriftException(in io.Reader) error {
	d := thrift.NewSkipDecoder(in)
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
	trans := remote.NewReaderBuffer(buf)
	defer trans.Release(nil)
	return c.unmarshalThriftData(trans, data, len(buf))
}

func (c thriftCodec) fastMessageUnmarshalAvailable(data interface{}, payloadLen int) bool {
	if payloadLen == 0 && c.CodecType&EnableSkipDecoder == 0 {
		return false
	}
	_, ok := data.(thrift.FastCodec)
	return ok
}

func (c thriftCodec) fastUnmarshal(trans remote.ByteBuffer, data interface{}, dataLen int) error {
	msg := data.(thrift.FastCodec)
	if dataLen > 0 {
		buf, err := trans.Next(dataLen)
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		_, err = msg.FastRead(buf)
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		return nil
	}
	buf, err := getSkippedStructBuffer(trans)
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
func (c thriftCodec) unmarshalThriftData(trans remote.ByteBuffer, data interface{}, dataLen int) error {
	// decode with hyper unmarshal
	if c.IsSet(FrugalRead) && c.hyperMessageUnmarshalAvailable(data, dataLen) {
		return c.hyperUnmarshal(trans, data, dataLen)
	}

	// decode with FastRead
	if c.IsSet(FastRead) && c.fastMessageUnmarshalAvailable(data, dataLen) {
		return c.fastUnmarshal(trans, data, dataLen)
	}

	if err := verifyUnmarshalBasicThriftDataType(data); err != nil {
		// if user only wants to use Basic we never try fallback to frugal or fastcodec
		if c.CodecType != Basic {
			// try FrugalRead < - > FastRead fallback
			if c.fastMessageUnmarshalAvailable(data, dataLen) {
				return c.fastUnmarshal(trans, data, dataLen)
			}
			if c.hyperMessageUnmarshalAvailable(data, dataLen) { // slim template?
				return c.hyperUnmarshal(trans, data, dataLen)
			}
		}
		return err
	}

	// fallback to old thrift way (slow)
	return decodeBasicThriftData(trans, data)
}

func (c thriftCodec) hyperUnmarshal(trans remote.ByteBuffer, data interface{}, dataLen int) error {
	if dataLen > 0 {
		buf, err := trans.Next(dataLen)
		if err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		if err = c.hyperMessageUnmarshal(buf, data); err != nil {
			return remote.NewTransError(remote.ProtocolError, err)
		}
		return nil
	}
	buf, err := getSkippedStructBuffer(trans)
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
	if err := apache.CheckTStruct(data); err != nil {
		return errDecodeMismatchMsgType
	}
	return nil
}

// decodeBasicThriftData decode thrift body the old way (slow)
func decodeBasicThriftData(trans remote.ByteBuffer, data interface{}) error {
	if err := verifyUnmarshalBasicThriftDataType(data); err != nil {
		return err
	}
	if err := apache.ThriftRead(trans, data); err != nil {
		return remote.NewTransError(remote.ProtocolError, err)
	}
	return nil
}

func getSkippedStructBuffer(trans remote.ByteBuffer) ([]byte, error) {
	sd := thrift.NewSkipDecoder(trans)
	buf, err := sd.Next(thrift.STRUCT)
	if err != nil {
		return nil, remote.NewTransError(remote.ProtocolError, err).AppendMessage("caught in SkipDecoder Next phase")
	}
	return buf, nil
}
