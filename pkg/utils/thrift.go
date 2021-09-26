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

package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/apache/thrift/lib/go/thrift"
)

// ThriftMessageCodec is used to codec thrift messages.
type ThriftMessageCodec struct {
	tb    *thrift.TMemoryBuffer
	tProt thrift.TProtocol
}

// NewThriftMessageCodec creates a new ThriftMessageCodec.
func NewThriftMessageCodec() *ThriftMessageCodec {
	transport := thrift.NewTMemoryBufferLen(1024)
	tProt := thrift.NewTBinaryProtocol(transport, true, true)

	return &ThriftMessageCodec{
		tb:    transport,
		tProt: tProt,
	}
}

// Encode do thrift message encode.
// Notice! msg must be XXXArgs/XXXResult that the wrap struct for args and result, not the actual args or result
// Notice! seqID will be reset in kitex if the buffer is used for generic call in client side, set seqID=0 is suggested
// when you call this method as client.
func (t *ThriftMessageCodec) Encode(method string, msgType thrift.TMessageType, seqID int32, msg thrift.TStruct) (b []byte, err error) {
	if method == "" {
		return nil, errors.New("empty methodName in thrift RPCEncode")
	}
	t.tb.Reset()
	if err = t.tProt.WriteMessageBegin(method, msgType, seqID); err != nil {
		return
	}
	if err = msg.Write(t.tProt); err != nil {
		return
	}
	if err = t.tProt.WriteMessageEnd(); err != nil {
		return
	}
	b = append(b, t.tb.Bytes()...)
	return
}

// Decode do thrift message decode, notice: msg must be XXXArgs/XXXResult that the wrap struct for args and result, not the actual args or result
func (t *ThriftMessageCodec) Decode(b []byte, msg thrift.TStruct) (method string, seqID int32, err error) {
	t.tb.Reset()
	if _, err = t.tb.Write(b); err != nil {
		return
	}
	var msgType thrift.TMessageType
	if method, msgType, seqID, err = t.tProt.ReadMessageBegin(); err != nil {
		return
	}
	if msgType == thrift.EXCEPTION {
		exception := thrift.NewTApplicationException(thrift.UNKNOWN_APPLICATION_EXCEPTION, "")
		if err = exception.Read(t.tProt); err != nil {
			return
		}
		if err = t.tProt.ReadMessageEnd(); err != nil {
			return
		}
		err = exception
		return
	}
	if err = msg.Read(t.tProt); err != nil {
		return
	}
	t.tProt.ReadMessageEnd()
	return
}

// Serialize serialize message into bytes. This is normal thrift serialize func.
// Notice: Binary generic use Encode instead of Serialize.
func (t *ThriftMessageCodec) Serialize(msg thrift.TStruct) (b []byte, err error) {
	t.tb.Reset()

	if err = msg.Write(t.tProt); err != nil {
		return
	}
	b = append(b, t.tb.Bytes()...)
	return
}

// Deserialize deserialize bytes into message. This is normal thrift deserialize func.
// Notice: Binary generic use Decode instead of Deserialize.
func (t *ThriftMessageCodec) Deserialize(msg thrift.TStruct, b []byte) (err error) {
	t.tb.Reset()
	if _, err = t.tb.Write(b); err != nil {
		return
	}
	if err = msg.Read(t.tProt); err != nil {
		return
	}
	return nil
}

// MarshalError convert go error to thrift exception, and encode exception over buffered binary transport.
func MarshalError(method string, err error) []byte {
	e := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, err.Error())
	var buf bytes.Buffer
	trans := thrift.NewStreamTransportRW(&buf)
	proto := thrift.NewTBinaryProtocol(trans, true, true)
	if err := proto.WriteMessageBegin(method, thrift.EXCEPTION, 0); err != nil {
		return nil
	}
	if err := e.Write(proto); err != nil {
		return nil
	}
	if err := proto.WriteMessageEnd(); err != nil {
		return nil
	}
	if err := proto.Flush(context.Background()); err != nil {
		return nil
	}
	return buf.Bytes()
}

// UnmarshalError decode binary and return error message
func UnmarshalError(b []byte) error {
	trans := thrift.NewStreamTransportR(bytes.NewReader(b))
	proto := thrift.NewTBinaryProtocolTransport(trans)
	if _, _, _, err := proto.ReadMessageBegin(); err != nil {
		return fmt.Errorf("read message begin error: %w", err)
	}
	e := thrift.NewTApplicationException(0, "")
	if err := e.Read(proto); err != nil {
		return fmt.Errorf("read exception error: %w", err)
	}
	if err := proto.ReadMessageEnd(); err != nil {
		return fmt.Errorf("read message end error: %w", err)
	}
	return e
}
