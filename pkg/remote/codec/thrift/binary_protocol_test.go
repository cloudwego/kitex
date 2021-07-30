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
	"encoding/binary"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
)

func TestMessage(t *testing.T) {
	trans := remote.NewReaderWriterBuffer(-1)
	prot := NewBinaryProtocol(trans)

	// check write
	name := "name"
	var seqID int32 = 1
	typeID := thrift.CALL
	err := prot.WriteMessageBegin(name, typeID, seqID)
	test.Assert(t, err == nil, err)

	tmp, _ := trans.Bytes()
	test.Assert(t, binary.BigEndian.Uint32(tmp[:4]) == 0x80010001)
	test.Assert(t, binary.BigEndian.Uint32(tmp[4:8]) == uint32(len(name)))
	test.Assert(t, string(tmp[8:8+len(name)]) == name)
	test.Assert(t, binary.BigEndian.Uint32(tmp[8+len(name):12+len(name)]) == uint32(seqID))

	err = prot.WriteMessageEnd()
	test.Assert(t, err == nil, err)

	err = prot.Flush(context.Background())
	test.Assert(t, err == nil, err)

	// check read
	_name, _typeID, _seqID, _err := prot.ReadMessageBegin()
	test.Assert(t, _err == nil, _err)
	test.Assert(t, _name == name, _name, name)
	test.Assert(t, _typeID == typeID, _typeID, typeID)
	test.Assert(t, _seqID == seqID, _seqID, seqID)
	err = prot.ReadMessageEnd()
	test.Assert(t, err == nil, err)
}

func TestStruct(t *testing.T) {
	trans := remote.NewReaderWriterBuffer(-1)
	prot := NewBinaryProtocol(trans)

	name := "struct"
	err := prot.WriteStructBegin(name)
	test.Assert(t, err == nil, err)
	err = prot.WriteStructEnd()
	test.Assert(t, err == nil, err)

	_name, _err := prot.ReadStructBegin()
	test.Assert(t, _err == nil, _err)
	test.Assert(t, _name == "", _name)
	err = prot.ReadStructEnd()
	test.Assert(t, err == nil, err)
}

func TestField(t *testing.T) {
	trans := remote.NewReaderWriterBuffer(-1)
	prot := NewBinaryProtocol(trans)

	name := "name"
	var fieldID int16 = 1
	err := prot.WriteFieldBegin(name, thrift.STRUCT, fieldID)
	test.Assert(t, err == nil, err)
	err = prot.WriteFieldEnd()
	test.Assert(t, err == nil, err)
	err = prot.WriteFieldStop()
	test.Assert(t, err == nil, err)
	tmp, _ := trans.Bytes()
	test.Assert(t, len(tmp) == 4)
	test.Assert(t, tmp[0] == thrift.STRUCT)
	test.Assert(t, binary.BigEndian.Uint16(tmp[1:]) == uint16(fieldID))
	test.Assert(t, tmp[3] == thrift.STOP)
	err = prot.Flush(context.Background())
	test.Assert(t, err == nil, err)

	_name, _typeID, _fieldID, _err := prot.ReadFieldBegin()
	test.Assert(t, _err == nil, _err)
	test.Assert(t, _name == "", _name)
	test.Assert(t, _typeID == thrift.STRUCT, _typeID)
	test.Assert(t, _fieldID == fieldID, _fieldID, fieldID)
	err = prot.ReadFieldEnd()
	test.Assert(t, err == nil, err)
	// this is stop
	_name, _typeID, _fieldID, _err = prot.ReadFieldBegin()
	test.Assert(t, _err == nil, _err)
	test.Assert(t, _name == "", _name)
	test.Assert(t, _typeID == thrift.STOP, _typeID)
	test.Assert(t, _fieldID == 0, _fieldID)
}

func TestMap(t *testing.T) {
	trans := remote.NewReaderWriterBuffer(-1)
	prot := NewBinaryProtocol(trans)

	size := 10
	err := prot.WriteMapBegin(thrift.I32, thrift.BOOL, size)
	test.Assert(t, err == nil, err)
	err = prot.WriteMapEnd()
	test.Assert(t, err == nil, err)
	tmp, _ := trans.Bytes()
	test.Assert(t, len(tmp) == 6)
	test.Assert(t, tmp[0] == thrift.I32)
	test.Assert(t, tmp[1] == thrift.BOOL)
	test.Assert(t, binary.BigEndian.Uint32(tmp[2:]) == uint32(size))

	err = prot.Flush(context.Background())
	test.Assert(t, err == nil, err)

	_kType, _vType, _size, _err := prot.ReadMapBegin()
	test.Assert(t, _err == nil, _err)
	test.Assert(t, _size == size, _size, size)
	test.Assert(t, _kType == thrift.I32, _kType)
	test.Assert(t, _vType == thrift.BOOL, _vType)
	err = prot.ReadMapEnd()
	test.Assert(t, err == nil, err)
}

func TestList(t *testing.T) {
	trans := remote.NewReaderWriterBuffer(-1)
	prot := NewBinaryProtocol(trans)

	size := 10
	err := prot.WriteListBegin(thrift.I64, size)
	test.Assert(t, err == nil, err)
	err = prot.WriteListEnd()
	test.Assert(t, err == nil, err)
	tmp, _ := trans.Bytes()
	test.Assert(t, len(tmp) == 5)
	test.Assert(t, tmp[0] == thrift.I64)
	test.Assert(t, binary.BigEndian.Uint32(tmp[1:]) == uint32(size))

	err = prot.Flush(context.Background())
	test.Assert(t, err == nil, err)

	_type, _size, _err := prot.ReadListBegin()
	test.Assert(t, _err == nil, _err)
	test.Assert(t, _size == size, _size, size)
	test.Assert(t, _type == thrift.I64, _type)
	err = prot.ReadListEnd()
	test.Assert(t, err == nil, err)
}

func TestSet(t *testing.T) {
	trans := remote.NewReaderWriterBuffer(-1)
	prot := NewBinaryProtocol(trans)

	size := 10
	err := prot.WriteSetBegin(thrift.STRING, size)
	test.Assert(t, err == nil, err)
	err = prot.WriteSetEnd()
	test.Assert(t, err == nil, err)
	tmp, _ := trans.Bytes()
	test.Assert(t, len(tmp) == 5)
	test.Assert(t, tmp[0] == thrift.STRING)
	test.Assert(t, binary.BigEndian.Uint32(tmp[1:]) == uint32(size))

	err = prot.Flush(context.Background())
	test.Assert(t, err == nil, err)

	_type, _size, _err := prot.ReadSetBegin()
	test.Assert(t, _err == nil, _err)
	test.Assert(t, _size == size, _size, size)
	test.Assert(t, _type == thrift.STRING, _type)
	err = prot.ReadSetEnd()
	test.Assert(t, err == nil, err)
}

func TestConst(t *testing.T) {
	trans := remote.NewReaderWriterBuffer(-1)
	prot := NewBinaryProtocol(trans)

	n := 0
	err := prot.WriteBool(false)
	n++
	test.Assert(t, err == nil, err)
	err = prot.WriteByte(0x1)
	n++
	test.Assert(t, err == nil, err)
	err = prot.WriteI16(0x2)
	n += 2
	test.Assert(t, err == nil, err)
	err = prot.WriteI32(0x3)
	n += 4
	test.Assert(t, err == nil, err)
	err = prot.WriteI64(0x4)
	n += 8
	test.Assert(t, err == nil, err)
	err = prot.WriteDouble(5.0)
	n += 8
	test.Assert(t, err == nil, err)
	err = prot.WriteString("6")
	n += 4 + 1
	test.Assert(t, err == nil, err)
	err = prot.WriteBinary([]byte{7})
	n += 4 + 1
	test.Assert(t, err == nil, err)
	err = prot.Flush(context.Background())
	test.Assert(t, err == nil, err)

	tmp, _ := trans.Bytes()
	test.Assert(t, len(tmp) == n, len(tmp))
	test.Assert(t, tmp[0] == 0x0)
	test.Assert(t, tmp[1] == 0x1)
	test.Assert(t, tmp[3] == 0x2)
	test.Assert(t, tmp[7] == 0x3)
	test.Assert(t, tmp[15] == 0x4)
	test.Assert(t, string(tmp[28:29]) == "6")
	test.Assert(t, tmp[33] == 0x7)

	err = prot.Flush(context.Background())
	test.Assert(t, err == nil, err)

	_bool, err := prot.ReadBool()
	test.Assert(t, err == nil, err)
	test.Assert(t, _bool == false)
	_byte, err := prot.ReadByte()
	test.Assert(t, err == nil, err)
	test.Assert(t, _byte == 0x1)
	_i16, err := prot.ReadI16()
	test.Assert(t, err == nil, err)
	test.Assert(t, _i16 == 0x2)
	_i32, err := prot.ReadI32()
	test.Assert(t, err == nil, err)
	test.Assert(t, _i32 == 0x3)
	_i64, err := prot.ReadI64()
	test.Assert(t, err == nil, err)
	test.Assert(t, _i64 == 0x4)
	_double, err := prot.ReadDouble()
	test.Assert(t, err == nil, err)
	test.Assert(t, _double == 5.0)
	_string, err := prot.ReadString()
	test.Assert(t, err == nil, err)
	test.Assert(t, _string == "6")
	_binary, err := prot.ReadBinary()
	test.Assert(t, err == nil, err)
	test.Assert(t, len(_binary) == 1)
	test.Assert(t, _binary[0] == 0x7)
}
