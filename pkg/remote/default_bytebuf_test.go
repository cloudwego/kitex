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

package remote

import (
	"testing"

	"github.com/jackedelic/kitex/internal/test"
)

func TestDefaultByteBuffer(t *testing.T) {
	buf1 := NewReaderWriterBuffer(-1)
	checkWritable(t, buf1)
	checkReadable(t, buf1)

	buf2 := NewReaderWriterBuffer(1024)
	checkWritable(t, buf2)
	checkReadable(t, buf2)

	buf3 := buf2.NewBuffer()
	checkWritable(t, buf3)
	checkUnreadable(t, buf3)

	buf4 := NewReaderWriterBuffer(-1)
	b, err := buf3.Bytes()
	test.Assert(t, err == nil, err)
	var n int
	n, err = buf4.AppendBuffer(buf3)
	test.Assert(t, err == nil)
	test.Assert(t, n == len(b))
	checkReadable(t, buf4)
}

func TestDefaultWriterBuffer(t *testing.T) {
	buf := NewWriterBuffer(-1)
	checkWritable(t, buf)
	checkUnreadable(t, buf)
}

func TestDefaultReaderBuffer(t *testing.T) {
	msg := "hello world"
	b := []byte(msg + msg + msg + msg + msg)
	buf := NewReaderBuffer(b)
	checkUnwritable(t, buf)
	checkReadable(t, buf)
}

func checkWritable(t *testing.T, buf ByteBuffer) {
	msg := "hello world"

	p, err := buf.Malloc(len(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, len(p) == len(msg))
	copy(p, msg)
	l := buf.MallocLen()
	test.Assert(t, l == len(msg))
	l, err = buf.WriteString(msg)
	test.Assert(t, err == nil, err)
	test.Assert(t, l == len(msg))
	l, err = buf.WriteBinary([]byte(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, l == len(msg))
	l, err = buf.Write([]byte(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, l == len(msg))
	err = buf.Flush()
	test.Assert(t, err == nil, err)
	var n int
	n, err = buf.Write([]byte(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(msg))
	b, err := buf.Bytes()
	test.Assert(t, err == nil, err)
	test.Assert(t, string(b) == msg+msg+msg+msg+msg, string(b))
}

func checkReadable(t *testing.T, buf ByteBuffer) {
	msg := "hello world"

	p, err := buf.Peek(len(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, string(p) == msg)
	err = buf.Skip(1 + len(msg))
	test.Assert(t, err == nil, err)
	p, err = buf.Next(len(msg) - 1)
	test.Assert(t, err == nil, err)
	test.Assert(t, string(p) == msg[1:])
	n := buf.ReadableLen()
	test.Assert(t, n == 3*len(msg), n)
	n = buf.ReadLen()
	test.Assert(t, n == 2*len(msg), n)

	var s string
	s, err = buf.ReadString(len(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, s == msg)
	p, err = buf.ReadBinary(len(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, string(p) == msg)
	p = make([]byte, len(msg))
	n, err = buf.Read(p)
	test.Assert(t, err == nil, err)
	test.Assert(t, string(p) == msg)
	test.Assert(t, n == 11, n)
}

func checkUnwritable(t *testing.T, buf ByteBuffer) {
	msg := "hello world"
	_, err := buf.Malloc(len(msg))
	test.Assert(t, err != nil)
	l := buf.MallocLen()
	test.Assert(t, l == -1, l)
	_, err = buf.WriteString(msg)
	test.Assert(t, err != nil)
	_, err = buf.WriteBinary([]byte(msg))
	test.Assert(t, err != nil)
	err = buf.Flush()
	test.Assert(t, err != nil)
	var n int
	n, err = buf.Write([]byte(msg))
	test.Assert(t, err != nil)
	test.Assert(t, n == -1, n)
}

func checkUnreadable(t *testing.T, buf ByteBuffer) {
	msg := "hello world"
	_, err := buf.Peek(len(msg))
	test.Assert(t, err != nil)
	err = buf.Skip(1)
	test.Assert(t, err != nil)
	_, err = buf.Next(len(msg) - 1)
	test.Assert(t, err != nil)
	n := buf.ReadableLen()
	test.Assert(t, n == -1)
	n = buf.ReadLen()
	test.Assert(t, n == 0)
	_, err = buf.ReadString(len(msg))
	test.Assert(t, err != nil)
	_, err = buf.ReadBinary(len(msg))
	test.Assert(t, err != nil)
	p := make([]byte, len(msg))
	n, err = buf.Read(p)
	test.Assert(t, err != nil)
	test.Assert(t, n == -1, n)
}
