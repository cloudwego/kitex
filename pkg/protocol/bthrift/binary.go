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

// Package bthrift .
package bthrift

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/cloudwego/kitex/pkg/remote/codec/perrors"
	"github.com/cloudwego/kitex/pkg/utils"
)

// Binary protocol for bthrift.
var Binary binaryProtocol

var _ BTProtocol = binaryProtocol{}

type binaryProtocol struct{}

func (binaryProtocol) WriteMessageBegin(buf []byte, name string, typeID thrift.TMessageType, seqid int32) int {
	offset := 0
	version := uint32(thrift.VERSION_1) | uint32(typeID)
	offset += Binary.WriteI32(buf, int32(version))
	offset += Binary.WriteString(buf[offset:], name)
	offset += Binary.WriteI32(buf[offset:], seqid)
	return offset
}

func (binaryProtocol) WriteMessageEnd(buf []byte) int {
	return 0
}

func (binaryProtocol) WriteStructBegin(buf []byte, name string) int {
	return 0
}

func (binaryProtocol) WriteStructEnd(buf []byte) int {
	return 0
}

func (binaryProtocol) WriteFieldBegin(buf []byte, name string, typeID thrift.TType, id int16) int {
	return Binary.WriteByte(buf, int8(typeID)) + Binary.WriteI16(buf[1:], id)
}

func (binaryProtocol) WriteFieldEnd(buf []byte) int {
	return 0
}

func (binaryProtocol) WriteFieldStop(buf []byte) int {
	return Binary.WriteByte(buf, thrift.STOP)
}

func (binaryProtocol) WriteMapBegin(buf []byte, keyType, valueType thrift.TType, size int) int {
	return Binary.WriteByte(buf, int8(keyType)) +
		Binary.WriteByte(buf[1:], int8(valueType)) +
		Binary.WriteI32(buf[2:], int32(size))
}

func (binaryProtocol) WriteMapEnd(buf []byte) int {
	return 0
}

func (binaryProtocol) WriteListBegin(buf []byte, elemType thrift.TType, size int) int {
	return Binary.WriteByte(buf, int8(elemType)) +
		Binary.WriteI32(buf[1:], int32(size))
}

func (binaryProtocol) WriteListEnd(buf []byte) int {
	return 0
}

func (binaryProtocol) WriteSetBegin(buf []byte, elemType thrift.TType, size int) int {
	return Binary.WriteByte(buf, int8(elemType)) +
		Binary.WriteI32(buf[1:], int32(size))
}

func (binaryProtocol) WriteSetEnd(buf []byte) int {
	return 0
}

func (binaryProtocol) WriteBool(buf []byte, value bool) int {
	if value {
		return Binary.WriteByte(buf, 1)
	}
	return Binary.WriteByte(buf, 0)
}

func (binaryProtocol) WriteByte(buf []byte, value int8) int {
	buf[0] = byte(value)
	return 1
}

func (binaryProtocol) WriteI16(buf []byte, value int16) int {
	binary.BigEndian.PutUint16(buf, uint16(value))
	return 2
}

func (binaryProtocol) WriteI32(buf []byte, value int32) int {
	binary.BigEndian.PutUint32(buf, uint32(value))
	return 4
}

func (binaryProtocol) WriteI64(buf []byte, value int64) int {
	binary.BigEndian.PutUint64(buf, uint64(value))
	return 8
}

func (binaryProtocol) WriteDouble(buf []byte, value float64) int {
	return Binary.WriteI64(buf, int64(math.Float64bits(value)))
}

func (binaryProtocol) WriteString(buf []byte, value string) int {
	l := Binary.WriteI32(buf, int32(len(value)))
	copy(buf[l:], value)
	return l + len(value)
}

func (binaryProtocol) WriteBinary(buf, value []byte) int {
	l := Binary.WriteI32(buf, int32(len(value)))
	copy(buf[l:], value)
	return l + len(value)
}

func (binaryProtocol) WriteStringNocopy(buf []byte, binaryWriter BinaryWriter, value string) int {
	return Binary.WriteBinaryNocopy(buf, binaryWriter, utils.StringToSliceByte(value))
}

func (binaryProtocol) WriteBinaryNocopy(buf []byte, binaryWriter BinaryWriter, value []byte) int {
	l := Binary.WriteI32(buf, int32(len(value)))
	copy(buf[l:], value)
	return l + len(value)
}

func (binaryProtocol) MessageBeginLength(name string, typeID thrift.TMessageType, seqid int32) int {
	version := uint32(thrift.VERSION_1) | uint32(typeID)
	return Binary.I32Length(int32(version)) + Binary.StringLength(name) + Binary.I32Length(seqid)
}

func (binaryProtocol) MessageEndLength() int {
	return 0
}

func (binaryProtocol) StructBeginLength(name string) int {
	return 0
}

func (binaryProtocol) StructEndLength() int {
	return 0
}

func (binaryProtocol) FieldBeginLength(name string, typeID thrift.TType, id int16) int {
	return Binary.ByteLength(int8(typeID)) + Binary.I16Length(id)
}

func (binaryProtocol) FieldEndLength() int {
	return 0
}

func (binaryProtocol) FieldStopLength() int {
	return Binary.ByteLength(thrift.STOP)
}

func (binaryProtocol) MapBeginLength(keyType, valueType thrift.TType, size int) int {
	return Binary.ByteLength(int8(keyType)) +
		Binary.ByteLength(int8(valueType)) +
		Binary.I32Length(int32(size))
}

func (binaryProtocol) MapEndLength() int {
	return 0
}

func (binaryProtocol) ListBeginLength(elemType thrift.TType, size int) int {
	return Binary.ByteLength(int8(elemType)) +
		Binary.I32Length(int32(size))
}

func (binaryProtocol) ListEndLength() int {
	return 0
}

func (binaryProtocol) SetBeginLength(elemType thrift.TType, size int) int {
	return Binary.ByteLength(int8(elemType)) +
		Binary.I32Length(int32(size))
}

func (binaryProtocol) SetEndLength() int {
	return 0
}

func (binaryProtocol) BoolLength(value bool) int {
	if value {
		return Binary.ByteLength(1)
	}
	return Binary.ByteLength(0)
}

func (binaryProtocol) ByteLength(value int8) int {
	return 1
}

func (binaryProtocol) I16Length(value int16) int {
	return 2
}

func (binaryProtocol) I32Length(value int32) int {
	return 4
}

func (binaryProtocol) I64Length(value int64) int {
	return 8
}

func (binaryProtocol) DoubleLength(value float64) int {
	return Binary.I64Length(int64(math.Float64bits(value)))
}

func (binaryProtocol) StringLength(value string) int {
	return Binary.I32Length(int32(len(value))) + len(value)
}

func (binaryProtocol) BinaryLength(value []byte) int {
	return Binary.I32Length(int32(len(value))) + len(value)
}

func (binaryProtocol) StringLengthNocopy(value string) int {
	return Binary.BinaryLengthNocopy(utils.StringToSliceByte(value))
}

func (binaryProtocol) BinaryLengthNocopy(value []byte) int {
	l := Binary.I32Length(int32(len(value)))
	return l + len(value)
}

func (binaryProtocol) ReadMessageBegin(buf []byte) (name string, typeID thrift.TMessageType, seqid int32, length int, err error) {
	size, l, e := Binary.ReadI32(buf)
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	if size > 0 {
		err = perrors.NewProtocolErrorWithType(perrors.BadVersion, "Missing version in ReadMessageBegin")
		return
	}
	typeID = thrift.TMessageType(size & 0x0ff)
	version := int64(size) & thrift.VERSION_MASK
	if version != thrift.VERSION_1 {
		err = perrors.NewProtocolErrorWithType(perrors.BadVersion, "Bad version in ReadMessageBegin")
		return
	}
	name, l, e = Binary.ReadString(buf[length:])
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	seqid, l, e = Binary.ReadI32(buf[length:])
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	return
}

func (binaryProtocol) ReadMessageEnd(buf []byte) (int, error) {
	return 0, nil
}

func (binaryProtocol) ReadStructBegin(buf []byte) (name string, length int, err error) {
	return
}

func (binaryProtocol) ReadStructEnd(buf []byte) (int, error) {
	return 0, nil
}

func (binaryProtocol) ReadFieldBegin(buf []byte) (name string, typeID thrift.TType, id int16, length int, err error) {
	t, l, e := Binary.ReadByte(buf)
	length += l
	typeID = thrift.TType(t)
	if e != nil {
		err = e
		return
	}
	if t != thrift.STOP {
		id, l, err = Binary.ReadI16(buf[length:])
		length += l
	}
	return
}

func (binaryProtocol) ReadFieldEnd(buf []byte) (int, error) {
	return 0, nil
}

func (binaryProtocol) ReadMapBegin(buf []byte) (keyType, valueType thrift.TType, size, length int, err error) {
	k, l, e := Binary.ReadByte(buf)
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	keyType = thrift.TType(k)
	v, l, e := Binary.ReadByte(buf[length:])
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	valueType = thrift.TType(v)
	size32, l, e := Binary.ReadI32(buf[length:])
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	if size32 < 0 {
		err = perrors.InvalidDataLength
		return
	}
	size = int(size32)
	return
}

func (binaryProtocol) ReadMapEnd(buf []byte) (int, error) {
	return 0, nil
}

func (binaryProtocol) ReadListBegin(buf []byte) (elemType thrift.TType, size, length int, err error) {
	b, l, e := Binary.ReadByte(buf)
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	elemType = thrift.TType(b)
	size32, l, e := Binary.ReadI32(buf[length:])
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	if size32 < 0 {
		err = perrors.InvalidDataLength
		return
	}
	size = int(size32)

	return
}

func (binaryProtocol) ReadListEnd(buf []byte) (int, error) {
	return 0, nil
}

func (binaryProtocol) ReadSetBegin(buf []byte) (elemType thrift.TType, size, length int, err error) {
	b, l, e := Binary.ReadByte(buf)
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	elemType = thrift.TType(b)
	size32, l, e := Binary.ReadI32(buf[length:])
	length += l
	if e != nil {
		err = perrors.NewProtocolError(e)
		return
	}
	if size32 < 0 {
		err = perrors.InvalidDataLength
		return
	}
	size = int(size32)
	return
}

func (binaryProtocol) ReadSetEnd(buf []byte) (int, error) {
	return 0, nil
}

func (binaryProtocol) ReadBool(buf []byte) (value bool, length int, err error) {
	b, l, e := Binary.ReadByte(buf)
	v := true
	if b != 1 {
		v = false
	}
	return v, l, e
}

func (binaryProtocol) ReadByte(buf []byte) (value int8, length int, err error) {
	if len(buf) < 1 {
		return value, length, perrors.NewProtocolErrorWithType(thrift.INVALID_DATA, "[ReadByte] buf length less than 1")
	}
	return int8(buf[0]), 1, err
}

func (binaryProtocol) ReadI16(buf []byte) (value int16, length int, err error) {
	if len(buf) < 2 {
		return value, length, perrors.NewProtocolErrorWithType(thrift.INVALID_DATA, "[ReadI16] buf length less than 2")
	}
	value = int16(binary.BigEndian.Uint16(buf))
	return value, 2, err
}

func (binaryProtocol) ReadI32(buf []byte) (value int32, length int, err error) {
	if len(buf) < 4 {
		return value, length, perrors.NewProtocolErrorWithType(thrift.INVALID_DATA, "[ReadI32] buf length less than 4")
	}
	value = int32(binary.BigEndian.Uint32(buf))
	return value, 4, err
}

func (binaryProtocol) ReadI64(buf []byte) (value int64, length int, err error) {
	if len(buf) < 8 {
		return value, length, perrors.NewProtocolErrorWithType(thrift.INVALID_DATA, "[ReadI64] buf length less than 8")
	}
	value = int64(binary.BigEndian.Uint64(buf))
	return value, 8, err
}

func (binaryProtocol) ReadDouble(buf []byte) (value float64, length int, err error) {
	if len(buf) < 8 {
		return value, length, perrors.NewProtocolErrorWithType(thrift.INVALID_DATA, "[ReadDouble] buf length less than 8")
	}
	value = math.Float64frombits(binary.BigEndian.Uint64(buf))
	return value, 8, err
}

func (binaryProtocol) ReadString(buf []byte) (value string, length int, err error) {
	size, l, e := Binary.ReadI32(buf)
	length += l
	if e != nil {
		err = e
		return
	}
	if size < 0 || int(size) > len(buf) {
		return value, length, perrors.NewProtocolErrorWithType(thrift.INVALID_DATA, "[ReadString] the string size greater than buf length")
	}
	value = string(buf[length : length+int(size)])
	length += int(size)
	return
}

func (binaryProtocol) ReadBinary(buf []byte) (value []byte, length int, err error) {
	size, l, e := Binary.ReadI32(buf)
	length += l
	if e != nil {
		err = e
		return
	}
	if size < 0 || int(size) > len(buf) {
		return value, length, perrors.NewProtocolErrorWithType(thrift.INVALID_DATA, "[ReadBinary] the binary size greater than buf length")
	}
	value = make([]byte, size)
	copy(value, buf[length:length+int(size)])
	length += int(size)
	return
}

// Skip .
func (binaryProtocol) Skip(buf []byte, fieldType thrift.TType) (length int, err error) {
	return SkipDefaultDepth(buf, Binary, fieldType)
}

// SkipDefaultDepth skips over the next data element from the provided input TProtocol object.
func SkipDefaultDepth(buf []byte, prot BTProtocol, typeID thrift.TType) (int, error) {
	return Skip(buf, prot, typeID, thrift.DEFAULT_RECURSION_DEPTH)
}

// Skip skips over the next data element from the provided input TProtocol object.
func Skip(buf []byte, self BTProtocol, fieldType thrift.TType, maxDepth int) (length int, err error) {
	if maxDepth <= 0 {
		return 0, thrift.NewTProtocolExceptionWithType(thrift.DEPTH_LIMIT, errors.New("depth limit exceeded"))
	}

	var l int
	switch fieldType {
	case thrift.BOOL:
		_, l, err = self.ReadBool(buf)
		length += l
		return
	case thrift.BYTE:
		_, l, err = self.ReadByte(buf)
		length += l
		return
	case thrift.I16:
		_, l, err = self.ReadI16(buf)
		length += l
		return
	case thrift.I32:
		_, l, err = self.ReadI32(buf)
		length += l
		return
	case thrift.I64:
		_, l, err = self.ReadI64(buf)
		length += l
		return
	case thrift.DOUBLE:
		_, l, err = self.ReadDouble(buf)
		length += l
		return
	case thrift.STRING:
		_, l, err = self.ReadString(buf)
		length += l
		return
	case thrift.STRUCT:
		_, l, err = self.ReadStructBegin(buf)
		length += l
		if err != nil {
			return
		}
		for {
			_, typeID, _, l, e := self.ReadFieldBegin(buf[length:])
			length += l
			if e != nil {
				err = e
				return
			}
			if typeID == thrift.STOP {
				break
			}
			l, e = Skip(buf[length:], self, typeID, maxDepth-1)
			length += l
			if e != nil {
				err = e
				return
			}
			l, e = self.ReadFieldEnd(buf[length:])
			length += l
			if e != nil {
				err = e
				return
			}
		}
		l, e := self.ReadStructEnd(buf[length:])
		length += l
		if e != nil {
			err = e
		}
		return
	case thrift.MAP:
		keyType, valueType, size, l, e := self.ReadMapBegin(buf)
		length += l
		if e != nil {
			err = e
			return
		}
		for i := 0; i < size; i++ {
			l, e := Skip(buf[length:], self, keyType, maxDepth-1)
			length += l
			if e != nil {
				err = e
				return
			}
			l, e = self.Skip(buf[length:], valueType)
			length += l
			if e != nil {
				err = e
				return
			}
		}
		l, e = self.ReadMapEnd(buf[length:])
		length += l
		if e != nil {
			err = e
		}
		return
	case thrift.SET:
		elemType, size, l, e := self.ReadSetBegin(buf)
		length += l
		if e != nil {
			err = e
			return
		}
		for i := 0; i < size; i++ {
			l, e = Skip(buf[length:], self, elemType, maxDepth-1)
			length += l
			if e != nil {
				err = e
				return
			}
		}
		l, e = self.ReadSetEnd(buf[length:])
		length += l
		if e != nil {
			err = e
		}
		return
	case thrift.LIST:
		elemType, size, l, e := self.ReadListBegin(buf)
		length += l
		if e != nil {
			err = e
			return
		}
		for i := 0; i < size; i++ {
			l, e = Skip(buf[length:], self, elemType, maxDepth-1)
			length += l
			if e != nil {
				err = e
				return
			}
		}
		l, e = self.ReadListEnd(buf[length:])
		length += l
		if e != nil {
			err = e
		}
		return
	default:
		return 0, thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, fmt.Errorf("unknown data type %d", fieldType))
	}
}
