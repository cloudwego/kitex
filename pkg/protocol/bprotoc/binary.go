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
	"math"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protowire"
)

// Binary protocol for bprotoc.
var Binary binaryProtocol

// When encoding length-prefixed fields, we speculatively set aside some number of bytes
// for the length, encode the data, and then encode the length (shifting the data if necessary
// to make room).
const speculativeLength = 1

var _ BProtocol = binaryProtocol{}

type binaryProtocol struct{}

// WriteMessage implements TLV(tag, length, value) and V(value).
func (b binaryProtocol) WriteMessage(buf []byte, number int32, fastWrite FastWrite) (n int) {
	// V
	if number == SkipTagNumber {
		return fastWrite.FastWrite(buf)
	}
	// TLV
	size := fastWrite.Size()
	n += AppendTag(buf, protowire.Number(number), protowire.BytesType)
	n += AppendVarint(buf[n:], uint64(size))
	n += fastWrite.FastWrite(buf[n:])
	return n
}

// WriteListPacked implements TLV(tag, length, value).
func (b binaryProtocol) WriteListPacked(buf []byte, number int32, length int, single BMarshal) (n int) {
	n += AppendTag(buf, protowire.Number(number), protowire.BytesType)
	buf = buf[n:]

	prefix := speculativeLength
	offset := prefix
	for i := 0; i < length; i++ {
		offset += single(buf[offset:], SkipTagNumber, int32(i))
	}
	mlen := offset - prefix

	msiz := protowire.SizeVarint(uint64(mlen))
	if msiz != speculativeLength {
		copy(buf[msiz:], buf[prefix:prefix+mlen])
	}
	AppendVarint(buf[:msiz], uint64(mlen))

	n += msiz + mlen
	return n
}

// WriteMapEntry implements TLV(tag, length, value).
func (b binaryProtocol) WriteMapEntry(buf []byte, number int32, entry BMarshal) (n int) {
	n += AppendTag(buf, protowire.Number(number), protowire.BytesType)
	buf = buf[n:]

	prefix := speculativeLength
	mlen := entry(buf[prefix:], MapEntry_Key_FieldNumber, MapEntry_Value_FieldNumber)

	msiz := protowire.SizeVarint(uint64(mlen))
	if msiz != speculativeLength {
		copy(buf[msiz:], buf[prefix:prefix+mlen])
	}
	AppendVarint(buf[:msiz], uint64(mlen))

	n += msiz + mlen
	return n
}

// WriteBool implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteBool(buf []byte, number int32, value bool) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.VarintType)
	}
	n += AppendVarint(buf[n:], protowire.EncodeBool(value))
	return n
}

// WriteInt32 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteInt32(buf []byte, number, value int32) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.VarintType)
	}
	n += AppendVarint(buf[n:], uint64(value))
	return n
}

// WriteInt64 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteInt64(buf []byte, number int32, value int64) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.VarintType)
	}
	n += AppendVarint(buf[n:], uint64(value))
	return n
}

// WriteUint32 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteUint32(buf []byte, number int32, value uint32) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.VarintType)
	}
	n += AppendVarint(buf[n:], uint64(value))
	return n
}

// WriteUint64 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteUint64(buf []byte, number int32, value uint64) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.VarintType)
	}
	n += AppendVarint(buf[n:], value)
	return n
}

// WriteSint32 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteSint32(buf []byte, number, value int32) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.VarintType)
	}
	n += AppendVarint(buf[n:], protowire.EncodeZigZag(int64(value)))
	return n
}

// WriteSint64 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteSint64(buf []byte, number int32, value int64) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.VarintType)
	}
	n += AppendVarint(buf[n:], protowire.EncodeZigZag(value))
	return n
}

// WriteFloat implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteFloat(buf []byte, number int32, value float32) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.Fixed32Type)
	}
	n += AppendFixed32(buf[n:], math.Float32bits(value))
	return n
}

// WriteDouble implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteDouble(buf []byte, number int32, value float64) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.Fixed64Type)
	}
	n += AppendFixed64(buf[n:], math.Float64bits(value))
	return n
}

// WriteFixed32 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteFixed32(buf []byte, number int32, value uint32) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.Fixed32Type)
	}
	n += AppendFixed32(buf[n:], value)
	return n
}

// WriteFixed64 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteFixed64(buf []byte, number int32, value uint64) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.Fixed64Type)
	}
	n += AppendFixed64(buf[n:], value)
	return n
}

// WriteSfixed32 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteSfixed32(buf []byte, number, value int32) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.Fixed32Type)
	}
	n += AppendFixed32(buf[n:], uint32(value))
	return n
}

// WriteSfixed64 implements TV(tag, value) and V(value).
func (b binaryProtocol) WriteSfixed64(buf []byte, number int32, value int64) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.Fixed64Type)
	}
	n += AppendFixed64(buf[n:], uint64(value))
	return n
}

// WriteString implements TLV(tag, length, value) and LV(length, value).
func (b binaryProtocol) WriteString(buf []byte, number int32, value string) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.BytesType)
	}
	// only support proto3
	if EnforceUTF8() && !utf8.ValidString(value) {
		panic(errInvalidUTF8)
	}
	n += AppendString(buf[n:], value)
	return n
}

// WriteBytes implements TLV(tag, length, value) and LV(length, value).
func (b binaryProtocol) WriteBytes(buf []byte, number int32, value []byte) (n int) {
	if number != SkipTagNumber {
		n += AppendTag(buf[n:], protowire.Number(number), protowire.BytesType)
	}
	n += AppendBytes(buf[n:], value)
	return n
}

// ReadBool implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadBool(buf []byte, _type int8) (value bool, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.VarintType {
		return value, 0, errUnknown
	}

	v, n := protowire.ConsumeVarint(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return protowire.DecodeBool(v), n, nil
}

// ReadInt32 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadInt32(buf []byte, _type int8) (value int32, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.VarintType {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeVarint(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return int32(v), n, nil
}

// ReadInt64 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadInt64(buf []byte, _type int8) (value int64, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.VarintType {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeVarint(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return int64(v), n, nil
}

// ReadUint32 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadUint32(buf []byte, _type int8) (value uint32, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.VarintType {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeVarint(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return uint32(v), n, nil
}

// ReadUint64 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadUint64(buf []byte, _type int8) (value uint64, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.VarintType {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeVarint(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return v, n, nil
}

// ReadSint32 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadSint32(buf []byte, _type int8) (value int32, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.VarintType {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeVarint(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return int32(protowire.DecodeZigZag(v & math.MaxUint32)), n, nil
}

// ReadSint64 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadSint64(buf []byte, _type int8) (value int64, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.VarintType {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeVarint(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return protowire.DecodeZigZag(v), n, nil
}

// ReadFloat implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadFloat(buf []byte, _type int8) (value float32, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.Fixed32Type {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeFixed32(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return math.Float32frombits(uint32(v)), n, nil
}

// ReadDouble implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadDouble(buf []byte, _type int8) (value float64, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.Fixed64Type {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeFixed64(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return math.Float64frombits(v), n, nil
}

// ReadFixed32 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadFixed32(buf []byte, _type int8) (value uint32, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.Fixed32Type {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeFixed32(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return uint32(v), n, nil
}

// ReadFixed64 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadFixed64(buf []byte, _type int8) (value uint64, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.Fixed64Type {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeFixed64(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return v, n, nil
}

// ReadSfixed32 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadSfixed32(buf []byte, _type int8) (value int32, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.Fixed32Type {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeFixed32(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return int32(v), n, nil
}

// ReadSfixed64 implements TV(tag, value) and V(value).
func (b binaryProtocol) ReadSfixed64(buf []byte, _type int8) (value int64, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.Fixed64Type {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeFixed64(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	return int64(v), n, nil
}

// ReadString implements TLV(tag, length, value) and V(length, value).
func (b binaryProtocol) ReadString(buf []byte, _type int8) (value string, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != protowire.BytesType {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeBytes(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	// only support proto3
	if EnforceUTF8() && !utf8.Valid(v) {
		return value, 0, errInvalidUTF8
	}
	return string(v), n, nil
}

// ReadBytes .
func (b binaryProtocol) ReadBytes(buf []byte, _type int8) (value []byte, n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != protowire.BytesType {
		return value, 0, errUnknown
	}
	v, n := protowire.ConsumeBytes(buf)
	if n < 0 {
		return value, 0, errDecode
	}
	value = make([]byte, len(v))
	copy(value, v)
	return value, n, nil
}

// ReadList .
func (b binaryProtocol) ReadList(buf []byte, _type int8, single BUnmarshal) (n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp == protowire.BytesType {
		var framed []byte
		framed, n = protowire.ConsumeBytes(buf)
		if n < 0 {
			return 0, errDecode
		}
		for len(framed) > 0 {
			off, err := single(buf, SkipTypeCheck)
			if err != nil {
				return 0, err
			}
			framed = framed[off:]
		}
		return n, nil
	}
	n, err = single(buf, _type)
	return n, err
}

// ReadMapEntry .
func (b binaryProtocol) ReadMapEntry(buf []byte, _type int8, umk, umv BUnmarshal) (int, error) {
	offset := 0
	wtyp := protowire.Type(_type)
	if wtyp != protowire.BytesType {
		return 0, errUnknown
	}
	bs, n := protowire.ConsumeBytes(buf)
	if n < 0 {
		return 0, errDecode
	}
	offset = n

	// Map entries are represented as a two-element message with fields
	// containing the key and value.
	for len(bs) > 0 {
		num, wtyp, n := protowire.ConsumeTag(bs)
		if n < 0 {
			return 0, errDecode
		}
		if num > protowire.MaxValidNumber {
			return 0, errDecode
		}
		bs = bs[n:]
		err := errUnknown
		switch num {
		case MapEntry_Key_FieldNumber:
			n, err = umk(bs, int8(wtyp))
		case MapEntry_Value_FieldNumber:
			n, err = umv(bs, int8(wtyp))
		}
		if err == errUnknown {
			n, err = b.Skip(buf, int8(wtyp), int32(num))
		}
		if err != nil && err != errUnknown {
			return 0, err
		}
		bs = bs[n:]
	}
	return offset, nil
}

// ReadMessage .
func (b binaryProtocol) ReadMessage(buf []byte, _type int8, fastRead FastRead) (n int, err error) {
	wtyp := protowire.Type(_type)
	if wtyp != SkipTypeCheck && wtyp != protowire.BytesType {
		return 0, errUnknown
	}
	// framed
	if wtyp == protowire.BytesType {
		buf, n = protowire.ConsumeBytes(buf)
		if n < 0 {
			return 0, errDecode
		}
	}
	offset := 0
	for offset < len(buf) {
		// Parse the tag (field number and wire type).
		num, wtyp, l := protowire.ConsumeTag(buf[offset:])
		offset += l
		if l < 0 {
			return offset, errDecode
		}
		if num > protowire.MaxValidNumber {
			return offset, errDecode
		}
		l, err = fastRead.FastRead(buf[offset:], int8(wtyp), int32(num))
		if err != nil {
			return offset, err
		}
		offset += l
	}
	// check if framed
	if n == 0 {
		n = offset
	}
	return n, nil
}

// Skip .
func (b binaryProtocol) Skip(buf []byte, _type int8, number int32) (n int, err error) {
	n = protowire.ConsumeFieldValue(protowire.Number(number), protowire.Type(_type), buf)
	if n < 0 {
		return 0, errDecode
	}
	return n, nil
}

// SizeBool .
func (b binaryProtocol) SizeBool(number int32, value bool) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.VarintType))
	}
	n += protowire.SizeVarint(protowire.EncodeBool(value))
	return n
}

// SizeInt32 .
func (b binaryProtocol) SizeInt32(number, value int32) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.VarintType))
	}
	n += protowire.SizeVarint(uint64(value))
	return n
}

// SizeInt64 .
func (b binaryProtocol) SizeInt64(number int32, value int64) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.VarintType))
	}
	n += protowire.SizeVarint(uint64(value))
	return n
}

// SizeUint32 .
func (b binaryProtocol) SizeUint32(number int32, value uint32) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.VarintType))
	}
	n += protowire.SizeVarint(uint64(value))
	return n
}

// SizeUint64 .
func (b binaryProtocol) SizeUint64(number int32, value uint64) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.VarintType))
	}
	n += protowire.SizeVarint(value)
	return n
}

// SizeSint32 .
func (b binaryProtocol) SizeSint32(number, value int32) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.VarintType))
	}
	n += protowire.SizeVarint(protowire.EncodeZigZag(int64(value)))
	return n
}

// SizeSint64 .
func (b binaryProtocol) SizeSint64(number int32, value int64) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.VarintType))
	}
	n += protowire.SizeVarint(protowire.EncodeZigZag(value))
	return n
}

// SizeFloat .
func (b binaryProtocol) SizeFloat(number int32, value float32) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.Fixed32Type))
	}
	n += protowire.SizeFixed32()
	return n
}

// SizeDouble .
func (b binaryProtocol) SizeDouble(number int32, value float64) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.Fixed64Type))
	}
	n += protowire.SizeFixed64()
	return n
}

// SizeFixed32 .
func (b binaryProtocol) SizeFixed32(number int32, value uint32) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.Fixed32Type))
	}
	n += protowire.SizeFixed32()
	return n
}

// SizeFixed64 .
func (b binaryProtocol) SizeFixed64(number int32, value uint64) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.Fixed64Type))
	}
	n += protowire.SizeFixed64()
	return n
}

// SizeSfixed32 .
func (b binaryProtocol) SizeSfixed32(number, value int32) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.Fixed32Type))
	}
	n += protowire.SizeFixed32()
	return n
}

// SizeSfixed64 .
func (b binaryProtocol) SizeSfixed64(number int32, value int64) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.Fixed64Type))
	}
	n += protowire.SizeFixed64()
	return n
}

// SizeString .
func (b binaryProtocol) SizeString(number int32, value string) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.BytesType))
	}
	n += protowire.SizeBytes(len(value))
	return n
}

// SizeBytes .
func (b binaryProtocol) SizeBytes(number int32, value []byte) (n int) {
	if number != SkipTagNumber {
		n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.BytesType))
	}
	n += protowire.SizeBytes(len(value))
	return n
}

// SizeMessage .
func (b binaryProtocol) SizeMessage(number int32, size FastSize) (n int) {
	if number == SkipTagNumber {
		return size.Size()
	}
	n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.BytesType))
	n += protowire.SizeBytes(size.Size())
	return n
}

// SizeListPacked .
func (b binaryProtocol) SizeListPacked(number int32, length int, single BSize) (n int) {
	n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.BytesType))

	mlen := 0
	for i := 0; i < length; i++ {
		// append mlen
		mlen += single(SkipTagNumber, int32(i))
	}
	n += protowire.SizeBytes(mlen)
	return n
}

// SizeMapEntry .
func (b binaryProtocol) SizeMapEntry(number int32, entry BSize) (n int) {
	n += protowire.SizeVarint(protowire.EncodeTag(protowire.Number(number), protowire.BytesType))
	mlen := entry(MapEntry_Key_FieldNumber, MapEntry_Value_FieldNumber)
	n += protowire.SizeBytes(mlen)
	return n
}
