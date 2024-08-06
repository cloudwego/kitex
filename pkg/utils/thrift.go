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
	"errors"
	"fmt"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/thrift/apache"

	athrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
)

// ThriftMessageCodec is used to codec thrift messages.
type ThriftMessageCodec struct{}

// NewThriftMessageCodec creates a new ThriftMessageCodec.
func NewThriftMessageCodec() *ThriftMessageCodec {
	return &ThriftMessageCodec{}
}

// Encode do thrift message encode.
// Notice! msg must be XXXArgs/XXXResult that the wrap struct for args and result, not the actual args or result
// Notice! seqID will be reset in kitex if the buffer is used for generic call in client side, set seqID=0 is suggested
// when you call this method as client.
// Deprecated: use github.com/cloudwego/gopkg/protocol/thrift.MarshalFastMsg
func (t *ThriftMessageCodec) Encode(method string, msgType athrift.TMessageType, seqID int32, msg athrift.TStruct) ([]byte, error) {
	if method == "" {
		return nil, errors.New("empty methodName in thrift RPCEncode")
	}
	b := make([]byte, thrift.Binary.MessageBeginLength(method))
	_ = thrift.Binary.WriteMessageBegin(b, method, thrift.TMessageType(msgType), seqID)
	buf := &bytes.Buffer{}
	buf.Write(b)
	if err := apache.ThriftWrite(apache.NewBufferTransport(buf), msg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode do thrift message decode, notice: msg must be XXXArgs/XXXResult that the wrap struct for args and result, not the actual args or result
// Deprecated: use github.com/cloudwego/gopkg/protocol/thrift.UnmarshalFastMsg
func (t *ThriftMessageCodec) Decode(b []byte, msg athrift.TStruct) (method string, seqID int32, err error) {
	var l int
	var msgType thrift.TMessageType
	method, msgType, seqID, l, err = thrift.Binary.ReadMessageBegin(b)
	if err != nil {
		return
	}
	b = b[l:]
	if msgType == thrift.EXCEPTION {
		ex := thrift.NewApplicationException(thrift.UNKNOWN_APPLICATION_EXCEPTION, "")
		if _, err = ex.FastRead(b); err == nil {
			err = ex // ApplicationException as err
		}
		return
	}
	err = apache.ThriftRead(apache.NewBufferTransport(bytes.NewBuffer(b)), msg)
	return
}

// Serialize serialize message into bytes. This is normal thrift serialize func.
// Notice: Binary generic use Encode instead of Serialize.
func (t *ThriftMessageCodec) Serialize(msg athrift.TStruct) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := apache.ThriftWrite(apache.NewBufferTransport(buf), msg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Deserialize deserialize bytes into message. This is normal thrift deserialize func.
// Notice: Binary generic use Decode instead of Deserialize.
func (t *ThriftMessageCodec) Deserialize(msg athrift.TStruct, b []byte) (err error) {
	buf := bytes.NewBuffer(b)
	return apache.ThriftRead(apache.NewBufferTransport(buf), msg)
}

// MarshalError convert go error to thrift exception, and encode exception over buffered binary transport.
func MarshalError(method string, err error) []byte {
	ex := thrift.NewApplicationException(thrift.INTERNAL_ERROR, err.Error())
	n := thrift.Binary.MessageBeginLength(method)
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
	ex := thrift.NewApplicationException(thrift.INTERNAL_ERROR, "")
	if _, err := ex.FastRead(b[off:]); err != nil {
		return err
	}
	return ex
}
