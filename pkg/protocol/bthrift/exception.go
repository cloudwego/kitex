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

package bthrift

import (
	"errors"
	"fmt"

	thrift "github.com/cloudwego/kitex/pkg/protocol/bthrift/apache"
)

// ApplicationException is for replacing apache.TApplicationException
// it implements ThriftMsgFastCodec interface.
type ApplicationException struct {
	t int32
	m string
}

// check interface only. TO BE REMOVED in the future
var _ thrift.TApplicationException = &ApplicationException{}

// NewApplicationException creates an ApplicationException instance
func NewApplicationException(t int32, msg string) *ApplicationException {
	return &ApplicationException{t: t, m: msg}
}

// Msg ...
func (e *ApplicationException) Msg() string { return e.m }

// TypeID ...
func (e *ApplicationException) TypeID() int32 { return e.t }

// TypeId ... for apache ApplicationException compatibility
func (e *ApplicationException) TypeId() int32 { return e.t }

// BLength returns the len of encoded buffer.
func (e *ApplicationException) BLength() int {
	// Msg Field: 1 (type) + 2 (id) + 4(strlen) + len(m)
	// Type Field: 1 (type) + 2 (id) + 4(ex type)
	// STOP: 1 byte
	return (1 + 2 + 4 + len(e.m)) + (1 + 2 + 4) + 1
}

// FastRead ...
func (e *ApplicationException) FastRead(b []byte) (off int, err error) {
	for i := 0; i < 2; i++ {
		_, tp, id, l, err := Binary.ReadFieldBegin(b[off:])
		if err != nil {
			return 0, err
		}
		off += l
		switch {
		case id == 1 && tp == thrift.STRING: // Msg
			e.m, l, err = Binary.ReadString(b[off:])
		case id == 2 && tp == thrift.I32: // TypeID
			e.t, l, err = Binary.ReadI32(b[off:])
		default:
			l, err = Binary.Skip(b, tp)
		}
		if err != nil {
			return 0, err
		}
		off += l
	}
	v, l, err := Binary.ReadByte(b[off:])
	if err != nil {
		return 0, err
	}
	if v != thrift.STOP {
		return 0, fmt.Errorf("expects thrift.STOP, found: %d", v)
	}
	off += l
	return off, nil
}

// FastWrite ...
func (e *ApplicationException) FastWrite(b []byte) (off int) {
	off += Binary.WriteFieldBegin(b[off:], "", thrift.STRING, 1)
	off += Binary.WriteString(b[off:], e.m)
	off += Binary.WriteFieldBegin(b[off:], "", thrift.I32, 2)
	off += Binary.WriteI32(b[off:], e.t)
	off += Binary.WriteByte(b[off:], thrift.STOP)
	return off
}

// FastWriteNocopy ... XXX: we deprecated XXXNocopy, simply using FastWrite is OK.
func (e *ApplicationException) FastWriteNocopy(b []byte, binaryWriter BinaryWriter) int {
	return e.FastWrite(b)
}

// Read implements Read interface of TStruct
// it only supports binary protocol.
// Deprecated: use FastRead instead
func (e *ApplicationException) Read(in thrift.TProtocol) error {
	for {
		_, ttype, id, err := in.ReadFieldBegin()
		if err != nil {
			return err
		}
		if ttype == thrift.STOP {
			break
		}
		switch {
		case id == 1 && ttype == thrift.STRING:
			e.m, err = in.ReadString()
			if err != nil {
				return err
			}
		case id == 2 && ttype == thrift.I32:
			e.t, err = in.ReadI32()
			if err != nil {
				return err
			}
		default:
			if err = thrift.SkipDefaultDepth(in, ttype); err != nil {
				return err
			}
		}
	}
	return nil
}

// Write implements Write interface of TStruct
// it only supports binary protocol.
// Deprecated: use FastWrite instead
func (e *ApplicationException) Write(out thrift.TProtocol) error {
	if err := out.WriteFieldBegin("message", thrift.STRING, 1); err != nil {
		return err
	}
	if err := out.WriteString(e.m); err != nil {
		return err
	}
	if err := out.WriteFieldBegin("type", thrift.I32, 2); err != nil {
		return err
	}
	if err := out.WriteI32(e.t); err != nil {
		return err
	}
	return out.WriteFieldStop()
}

// originally from github.com/apache/thrift@v0.13.0/lib/go/thrift/exception.go
var defaultApplicationExceptionMessage = map[int32]string{
	thrift.UNKNOWN_APPLICATION_EXCEPTION:  "unknown application exception",
	thrift.UNKNOWN_METHOD:                 "unknown method",
	thrift.INVALID_MESSAGE_TYPE_EXCEPTION: "invalid message type",
	thrift.WRONG_METHOD_NAME:              "wrong method name",
	thrift.BAD_SEQUENCE_ID:                "bad sequence ID",
	thrift.MISSING_RESULT:                 "missing result",
	thrift.INTERNAL_ERROR:                 "unknown internal error",
	thrift.PROTOCOL_ERROR:                 "unknown protocol error",
	thrift.INVALID_TRANSFORM:              "Invalid transform",
	thrift.INVALID_PROTOCOL:               "Invalid protocol",
	thrift.UNSUPPORTED_CLIENT_TYPE:        "Unsupported client type",
}

// Error implements apache.Exception
func (e *ApplicationException) Error() string {
	if e.m != "" {
		return e.m
	}
	if m, ok := defaultApplicationExceptionMessage[e.t]; ok {
		return m
	}
	return fmt.Sprintf("unknown exception type [%d]", e.t)
}

// TransportException is for replacing apache.TransportException
// it implements ThriftMsgFastCodec interface.
type TransportException struct {
	ApplicationException // same implementation ...
}

// NewTransportException ...
func NewTransportException(t int32, m string) *TransportException {
	ret := TransportException{}
	ret.t = t
	ret.m = m
	return &ret
}

// ProtocolException is for replacing apache.ProtocolException
// it implements ThriftMsgFastCodec interface.
type ProtocolException struct {
	ApplicationException // same implementation ...
}

// NewTransportException ...
func NewProtocolException(t int32, m string) *ProtocolException {
	ret := ProtocolException{}
	ret.t = t
	ret.m = m
	return &ret
}

// Generic Thrift exception with TypeId method
type tException interface {
	Error() string
	TypeId() int32
}

// Prepends additional information to an error without losing the Thrift exception interface
func PrependError(prepend string, err error) error {
	if t, ok := err.(*TransportException); ok {
		return NewTransportException(t.TypeID(), prepend+t.Error())
	}
	if t, ok := err.(*ProtocolException); ok {
		return NewProtocolException(t.TypeID(), prepend+err.Error())
	}
	if t, ok := err.(*ApplicationException); ok {
		return NewApplicationException(t.TypeID(), prepend+t.Error())
	}
	if t, ok := err.(tException); ok { // apache thrift exception?
		return NewApplicationException(t.TypeId(), prepend+t.Error())
	}
	return errors.New(prepend + err.Error())
}
