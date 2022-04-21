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

package codec

import (
	"fmt"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"testing"
)

func TestReadUint32(t *testing.T) {
	bytes := []byte{0x18, 0x2d, 0x44, 0x54}
	bytes2, _ := Bytes2Uint32(bytes)
	buffer := remote.NewReaderByteBuffer(bytes)
	read, err := ReadUint32(buffer)
	test.Assert(t, read == bytes2, err)
}

func TestPeekUint32(t *testing.T) {
	bytes := []byte{0x18, 0x19, 0x44, 0x54}
	buffer := remote.NewReaderByteBuffer(bytes)
	read, err := PeekUint32(buffer)
	bytes2, _ := Bytes2Uint32(bytes)
	test.Assert(t, read == bytes2, err)
}

func TestReadUint16(t *testing.T) {
	bytes := []byte{0x44, 0x54}
	buffer := remote.NewReaderByteBuffer(bytes)
	read, err := ReadUint16(buffer)
	bytes2 := Bytes2Uint16NoCheck(bytes)
	test.Assert(t, read == bytes2, err)
}

func TestWriteUint32(t *testing.T) {
	bytes := []byte{0x12, 0x34, 0x56, 0x78}
	bytes2Uint32, _ := Bytes2Uint32(bytes)
	buffer := remote.NewWriterBuffer(4)
	err := WriteUint32(bytes2Uint32, buffer)
	next, _ := buffer.Bytes()
	bytes2, _ := Bytes2Uint32(next)
	test.Assert(t, bytes2Uint32 == bytes2, err)
}

func TestWriteUint16(t *testing.T) {
	bytes := []byte{0x44, 0x23}
	bytes2Uint16 := Bytes2Uint16NoCheck(bytes)
	buffer := remote.NewWriterBuffer(2)
	err := WriteUint16(bytes2Uint16, buffer)
	next, _ := buffer.Bytes()
	bytes2 := Bytes2Uint16NoCheck(next)
	test.Assert(t, bytes2Uint16 == bytes2, err)
}

func TestWriteByte(t *testing.T) {
	var bytes byte = 'a'
	out := remote.NewWriterBuffer(1)
	err := WriteByte(bytes, out)
	next, _ := out.Bytes()
	test.Assert(t, bytes == next[0], err)
}

func TestBytes2Uint32NoCheck(t *testing.T) {

	bytes := []byte{0x12, 0x34, 0x56, 0x78}
	u := uint32(0x12345678)

	bytes2 := Bytes2Uint32NoCheck(bytes)
	test.Assert(t, bytes2 == u)
}

func TestBytes2Uint32(t *testing.T) {
	bytes := []byte{0x12, 0x34, 0x56, 0x78}
	u := uint32(0x12345678)
	b, err := Bytes2Uint32(bytes)
	test.Assert(t, b == u, err)
	bytes2 := []byte{0x12, 0x34}
	bytes2Uint32, err := Bytes2Uint32(bytes2)
	test.Assert(t, bytes2Uint32 == 0, err)
}

func TestBytes2Uint16NoCheck(t *testing.T) {
	bytes := []byte{0x56, 0x78}
	u := uint16(0x5678)

	bytes2 := Bytes2Uint16NoCheck(bytes)
	test.Assert(t, bytes2 == u)
}

func TestBytes2Uint16(t *testing.T) {

	bytes := []byte{0x12, 0x34, 0x56, 0x78}
	u := uint16(0x5678)
	b, err := Bytes2Uint16(bytes, 2)
	test.Assert(t, b == u, err)
	b2, err1 := Bytes2Uint16(bytes, 4)
	test.Assert(t, b2 == 0, err1)
}

func TestBytes2Uint8(t *testing.T) {
	bytes := []byte{0x12, 0x34, 0x56, 0x78}
	u := uint8(0x78)
	b, err := Bytes2Uint8(bytes, 3)
	test.Assert(t, b == u, err)
	b2, err1 := Bytes2Uint8(bytes, 4)
	test.Assert(t, b2 == 0, err1)
}

//todo This test encountered problems and found that the returned value was not the string length
func TestReadString(t *testing.T) {
	bytes := []byte("abcd")
	inBuffer := remote.NewReaderByteBuffer(bytes)
	readString, i, err := ReadString(inBuffer)
	fmt.Println(i)
	test.Assert(t, readString == "", err)
}

func TestWriteString(t *testing.T) {

	val := "avvavv"
	buffer := remote.NewWriterBuffer(6)
	writeString, err := WriteString(val, buffer)
	test.Assert(t, writeString == 10, err)
}

func TestWriteString2BLen(t *testing.T) {
	val := "bbbbbb"
	buffer := remote.NewWriterBuffer(6)
	writeString, err := WriteString2BLen(val, buffer)
	test.Assert(t, writeString == 8, err)
}

//todo This test encountered problems and found that the returned value was not the string length
func TestReadString2BLen(t *testing.T) {

}
