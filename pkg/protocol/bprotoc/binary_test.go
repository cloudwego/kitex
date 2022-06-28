/*
 * Copyright 2022 CloudWeGo Authors
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

package bprotoc

import (
	"fmt"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestBool(t *testing.T) {
	var num int32 = 255
	value := true

	exceptSize := 3
	size := Binary.SizeBool(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeBool[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f80f01"
	wn := Binary.WriteBool(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteBool[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.VarintType
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadBool", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadBool(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadBool[%d][%t] != except[%d][%t]", rn, rv, wn, value))
	}
}

func TestInt32(t *testing.T) {
	var num int32 = 255
	var value int32 = 0xFFFF

	exceptSize := 5
	size := Binary.SizeInt32(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeInt32[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f80fffff03"
	wn := Binary.WriteInt32(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteInt32[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.VarintType
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadInt32", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadInt32(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadInt32[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestInt64(t *testing.T) {
	var num int32 = 255
	var value int64 = 0xFFFF

	exceptSize := 5
	size := Binary.SizeInt64(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeInt64[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f80fffff03"
	wn := Binary.WriteInt64(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteInt64[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.VarintType
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadInt64", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadInt64(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadInt64[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestUint32(t *testing.T) {
	var num int32 = 255
	var value uint32 = 0xFFFF

	exceptSize := 5
	size := Binary.SizeUint32(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeUint32[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f80fffff03"
	wn := Binary.WriteUint32(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteUint32[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.VarintType
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadUint32", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadUint32(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadUint32[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestUint64(t *testing.T) {
	var num int32 = 255
	var value uint64 = 0xFFFF

	exceptSize := 5
	size := Binary.SizeUint64(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeUint64[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f80fffff03"
	wn := Binary.WriteUint64(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteUint64[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.VarintType
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadUint64", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadUint64(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadUint64[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestSint32(t *testing.T) {
	var num int32 = 255
	var value int32 = 0xFFFF

	exceptSize := 5
	size := Binary.SizeSint32(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeSint32[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f80ffeff07"
	wn := Binary.WriteSint32(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteSint32[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.VarintType
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadSint32", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadSint32(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadSint32[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestSint64(t *testing.T) {
	var num int32 = 255
	var value int64 = 0xFFFF

	exceptSize := 5
	size := Binary.SizeSint64(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeSint64[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f80ffeff07"
	wn := Binary.WriteSint64(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteSint64[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.VarintType
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadSint64", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadSint64(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadSint64[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestFloat(t *testing.T) {
	var num int32 = 255
	var value float32 = 0xFFFF

	exceptSize := 6
	size := Binary.SizeFloat(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeFloat[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "fd0f00ff7f47"
	wn := Binary.WriteFloat(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteFloat[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.Fixed32Type
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadFloat", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadFloat(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadFloat[%d][%.2f] != except[%d][%.2f]", rn, rv, wn, value))
	}
}

func TestDouble(t *testing.T) {
	var num int32 = 255
	var value float64 = 0xFFFF

	exceptSize := 10
	size := Binary.SizeDouble(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeDouble[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f90f00000000e0ffef40"
	wn := Binary.WriteDouble(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteDouble[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.Fixed64Type
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadDouble", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadDouble(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadDouble[%d][%.2f] != except[%d][%.2f]", rn, rv, wn, value))
	}
}

func TestFixed32(t *testing.T) {
	var num int32 = 255
	var value uint32 = 0xFFFF

	exceptSize := 6
	size := Binary.SizeFixed32(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeFixed32[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "fd0fffff0000"
	wn := Binary.WriteFixed32(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteFixed32[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.Fixed32Type
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadFixed32", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadFixed32(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadFixed32[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestFixed64(t *testing.T) {
	var num int32 = 255
	var value uint64 = 0xFFFF

	exceptSize := 10
	size := Binary.SizeFixed64(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeFixed64[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f90fffff000000000000"
	wn := Binary.WriteFixed64(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteFixed64[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.Fixed64Type
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadFixed64", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadFixed64(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadFixed64[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestSfixed32(t *testing.T) {
	var num int32 = 255
	var value int32 = 0xFFFF

	exceptSize := 6
	size := Binary.SizeSfixed32(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeSfixed32[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "fd0fffff0000"
	wn := Binary.WriteSfixed32(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteSfixed32[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.Fixed32Type
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadSfixed32", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadSfixed32(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadSfixed32[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestSfixed64(t *testing.T) {
	var num int32 = 255
	var value int64 = 0xFFFF

	exceptSize := 10
	size := Binary.SizeSfixed64(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeSfixed64[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "f90fffff000000000000"
	wn := Binary.WriteSfixed64(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteSfixed64[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.Fixed64Type
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadSfixed64", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadSfixed64(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadSfixed64[%d][%d] != except[%d][%d]", rn, rv, wn, value))
	}
}

func TestString(t *testing.T) {
	var num int32 = 255
	value := "hello world"

	exceptSize := 14
	size := Binary.SizeString(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeString[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "fa0f0b68656c6c6f20776f726c64"
	wn := Binary.WriteString(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteString[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.BytesType
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadString", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadString(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || rv != value {
		panic(fmt.Errorf("ReadString[%d][%s] != except[%d][%s]", rn, rv, wn, value))
	}
}

func TestBytes(t *testing.T) {
	var num int32 = 255
	value := make([]byte, 0)

	exceptSize := 3
	size := Binary.SizeBytes(num, value)
	if size != exceptSize {
		panic(fmt.Errorf("SizeBytes[%d] != except[%d]", size, exceptSize))
	}

	buf := make([]byte, 64)
	exceptWs := "fa0f00"
	wn := Binary.WriteBytes(buf, num, value)
	ws := fmt.Sprintf("%x", buf[:wn])
	if wn != size || ws != exceptWs {
		panic(fmt.Errorf("WriteBytes[%d][%s] != except[%d][%s]", wn, ws, size, exceptWs))
	}

	_type := protowire.BytesType
	gotRn, gotRt, offset := protowire.ConsumeTag(buf)
	AssertConsumeTag("ReadBytes", gotRn, gotRt, num, _type)
	rv, rn, err := Binary.ReadBytes(buf[offset:], int8(_type))
	if err != nil {
		panic(err)
	}
	rn += offset
	if rn != wn || string(rv) != string(value) {
		panic(fmt.Errorf("ReadBytes[%d][%s] != except[%d][%s]", rn, rv, wn, value))
	}
}

func AssertConsumeTag(name string, got_n protowire.Number, got_t protowire.Type, exp_n int32, exp_t protowire.Type) {
	if int32(got_n) != exp_n || got_t != exp_t {
		panic(fmt.Errorf("%s ConsumeTag num[%d]type[%d] != except[%d][%d]", name, got_n, got_t, exp_n, exp_t))
	}
}
