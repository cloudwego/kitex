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

// FastRead is designed for generating code.
type FastRead interface {
	FastRead(buf []byte, _type int8, number int32) (n int, err error)
}

// FastWrite is designed for generating code.
type FastWrite interface {
	FastSize
	FastWrite(buf []byte) (n int)
}

// FastSize is designed for generating code.
type FastSize interface {
	Size() (n int)
}

// BProtocol .
type BProtocol interface {
	WriteMessage(buf []byte, number int32, fastWrite FastWrite) (n int)
	WriteListPacked(buf []byte, number int32, length int, single BMarshal) (n int)
	WriteMapEntry(buf []byte, number int32, entry BMarshal) (n int)
	WriteBool(buf []byte, number int32, value bool) (n int)
	WriteInt32(buf []byte, number, value int32) (n int)
	WriteInt64(buf []byte, number int32, value int64) (n int)
	WriteUint32(buf []byte, number int32, value uint32) (n int)
	WriteUint64(buf []byte, number int32, value uint64) (n int)
	WriteSint32(buf []byte, number, value int32) (n int)
	WriteSint64(buf []byte, number int32, value int64) (n int)
	WriteFloat(buf []byte, number int32, value float32) (n int)
	WriteDouble(buf []byte, number int32, value float64) (n int)
	WriteFixed32(buf []byte, number int32, value uint32) (n int)
	WriteFixed64(buf []byte, number int32, value uint64) (n int)
	WriteSfixed32(buf []byte, number, value int32) (n int)
	WriteSfixed64(buf []byte, number int32, value int64) (n int)
	WriteString(buf []byte, number int32, value string) (n int)
	WriteBytes(buf []byte, number int32, value []byte) (n int)
	ReadMessage(buf []byte, _type int8, fastRead FastRead) (offset int, err error)
	ReadList(buf []byte, _type int8, single BUnmarshal) (n int, err error)
	ReadMapEntry(buf []byte, _type int8, umk, umv BUnmarshal) (int, error)
	ReadBool(buf []byte, _type int8) (value bool, n int, err error)
	ReadInt32(buf []byte, _type int8) (value int32, n int, err error)
	ReadInt64(buf []byte, _type int8) (value int64, n int, err error)
	ReadUint32(buf []byte, _type int8) (value uint32, n int, err error)
	ReadUint64(buf []byte, _type int8) (value uint64, n int, err error)
	ReadSint32(buf []byte, _type int8) (value int32, n int, err error)
	ReadSint64(buf []byte, _type int8) (value int64, n int, err error)
	ReadFloat(buf []byte, _type int8) (value float32, n int, err error)
	ReadDouble(buf []byte, _type int8) (value float64, n int, err error)
	ReadFixed32(buf []byte, _type int8) (value uint32, n int, err error)
	ReadFixed64(buf []byte, _type int8) (value uint64, n int, err error)
	ReadSfixed32(buf []byte, _type int8) (value int32, n int, err error)
	ReadSfixed64(buf []byte, _type int8) (value int64, n int, err error)
	ReadString(buf []byte, _type int8) (value string, n int, err error)
	ReadBytes(buf []byte, _type int8) (value []byte, n int, err error)
	SizeBool(number int32, value bool) (n int)
	SizeInt32(number, value int32) (n int)
	SizeInt64(number int32, value int64) (n int)
	SizeUint32(number int32, value uint32) (n int)
	SizeUint64(number int32, value uint64) (n int)
	SizeSint32(number, value int32) (n int)
	SizeSint64(number int32, value int64) (n int)
	SizeFloat(number int32, value float32) (n int)
	SizeDouble(number int32, value float64) (n int)
	SizeFixed32(number int32, value uint32) (n int)
	SizeFixed64(number int32, value uint64) (n int)
	SizeSfixed32(number, value int32) (n int)
	SizeSfixed64(number int32, value int64) (n int)
	SizeString(number int32, value string) (n int)
	SizeBytes(number int32, value []byte) (n int)
	SizeMessage(number int32, size FastSize) (n int)
	SizeMapEntry(number int32, entry BSize) (n int)
	SizeListPacked(number int32, length int, single BSize) (n int)
	Skip(buf []byte, _type int8, number int32) (n int, err error)
}

// BMarshal .
type BMarshal func(buf []byte, numTagOrKey, numIdxOrVal int32) int

// BSize .
type BSize func(numTagOrKey, numIdxOrVal int32) int

// BUnmarshal .
type BUnmarshal func(buf []byte, _type int8) (n int, err error)
