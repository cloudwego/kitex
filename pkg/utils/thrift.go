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
	"errors"
	"fmt"

	"github.com/cloudwego/gopkg/protocol/thrift"

	athrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
)

// ThriftMessageCodec is used to codec thrift messages.
type ThriftMessageCodec struct {
	tb    *athrift.TMemoryBuffer
	tProt athrift.TProtocol
}

// NewThriftMessageCodec creates a new ThriftMessageCodec.
func NewThriftMessageCodec() *ThriftMessageCodec {
	// TODO: use remote.ByteBuffer & remote/codec/thrift.BinaryProtocol
	transport := athrift.NewTMemoryBufferLen(1024)
	tProt := athrift.NewTBinaryProtocol(transport, true, true)

	return &ThriftMessageCodec{
		tb:    transport,
		tProt: tProt,
	}
}

// Encode do thrift message encode.
// Notice! msg must be XXXArgs/XXXResult that the wrap struct for args and result, not the actual args or result
// Notice! seqID will be reset in kitex if the buffer is used for generic call in client side, set seqID=0 is suggested
// when you call this method as client.
func (t *ThriftMessageCodec) Encode(method string, msgType athrift.TMessageType, seqID int32, msg athrift.TStruct) (b []byte, err error) {
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
func (t *ThriftMessageCodec) Decode(b []byte, msg athrift.TStruct) (method string, seqID int32, err error) {
	t.tb.Reset()
	if _, err = t.tb.Write(b); err != nil {
		return
	}
	var msgType athrift.TMessageType
	if method, msgType, seqID, err = t.tProt.ReadMessageBegin(); err != nil {
		return
	}
	if msgType == athrift.EXCEPTION {
		b = b[thrift.Binary.MessageBeginLength(method, 0, 0):] // for reusing fast read
		ex := thrift.NewApplicationException(athrift.UNKNOWN_APPLICATION_EXCEPTION, "")
		if _, err = ex.FastRead(b); err != nil {
			return
		}
		if err = t.tProt.ReadMessageEnd(); err != nil {
			return
		}
		err = ex
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
func (t *ThriftMessageCodec) Serialize(msg athrift.TStruct) (b []byte, err error) {
	t.tb.Reset()

	if err = msg.Write(t.tProt); err != nil {
		return
	}
	b = append(b, t.tb.Bytes()...)
	return
}

// Deserialize deserialize bytes into message. This is normal thrift deserialize func.
// Notice: Binary generic use Decode instead of Deserialize.
func (t *ThriftMessageCodec) Deserialize(msg athrift.TStruct, b []byte) (err error) {
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
	ex := thrift.NewApplicationException(athrift.INTERNAL_ERROR, err.Error())
	n := thrift.Binary.MessageBeginLength(method, 0, 0)
	n += ex.BLength()
	b := make([]byte, n)
	// Write message header
	off := thrift.Binary.WriteMessageBegin(b, method, thrift.EXCEPTION, 0)
	// Write Ex body
	off += ex.FastWrite(b[off:])
	return b[:off]
}

// UnmarshalError decode binary and return error message
func UnmarshalError(b []byte) error {
	// Read message header
	_, tp, _, l, err := thrift.Binary.ReadMessageBegin(b)
	if err != nil {
		return err
	}
	if tp != thrift.EXCEPTION {
		return fmt.Errorf("expects thrift.EXCEPTION, found: %d", tp)
	}
	// Read Ex body
	off := l
	ex := thrift.NewApplicationException(athrift.INTERNAL_ERROR, "")
	if _, err := ex.FastRead(b[off:]); err != nil {
		return err
	}
	return ex
}
