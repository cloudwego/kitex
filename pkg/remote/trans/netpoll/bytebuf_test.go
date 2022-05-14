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

package netpoll

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/netpoll"
)

func TestByteBuffer(t *testing.T) {
	// test NewReadWriter()
	msg := "hello world"
	reader := bufio.NewReader(strings.NewReader(msg + msg + msg + msg + msg))
	writer := bufio.NewWriter(bytes.NewBufferString(msg + msg + msg + msg + msg))
	bufRW := bufio.NewReadWriter(reader, writer)
	rw := netpoll.NewReadWriter(bufRW)

	// test NewReaderWriterByteBuffer()
	// p.s. NewReaderWriterByteBuffer unsupported io.ReadWriter
	buf1 := NewReaderWriterByteBuffer(rw)
	checkWritable(t, buf1)
	checkReadable(t, buf1)

	// test NewBuffer()
	buf2 := buf1.NewBuffer()
	checkWritable(t, buf2)
	checkUnreadable(t, buf2)

	// test AppendBuffer()
	err := buf1.AppendBuffer(buf2)
	test.Assert(t, err == nil)
}

func TestWriterBuffer(t *testing.T) {
	// test NewWriter()
	msg := "hello world"
	wi := bytes.NewBufferString(msg + msg + msg + msg + msg)
	w := netpoll.NewWriter(wi)
	nbp := &netpollByteBuffer{}

	// test NewWriterByteBuffer()
	buf := NewWriterByteBuffer(w)
	checkWritable(t, buf)
	checkUnreadable(t, buf)

	// test WriteDirect()
	err := nbp.WriteDirect([]byte(msg), 11)
	test.Assert(t, err != nil, err)
}

func TestReaderBuffer(t *testing.T) {
	// test NewReader()
	msg := "hello world"
	ri := strings.NewReader(msg + msg + msg + msg + msg)
	r := netpoll.NewReader(ri)

	// test NewReaderByteBuffer()
	buf := NewReaderByteBuffer(r)
	checkUnwritable(t, buf)
	checkReadable(t, buf)
}

func checkWritable(t *testing.T, buf remote.ByteBuffer) {
	// init
	msg := "hello world"

	// test Malloc()
	p, err := buf.Malloc(len(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, len(p) == len(msg))
	copy(p, msg)

	// test MallocLen()
	l := buf.MallocLen()
	test.Assert(t, l == len(msg))

	// test WriteString()
	l, err = buf.WriteString(msg)
	test.Assert(t, err == nil, err)
	test.Assert(t, l == len(msg))

	// test WriteBinary()
	l, err = buf.WriteBinary([]byte(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, l == len(msg))

	// test Flush()
	err = buf.Flush()
	test.Assert(t, err == nil, err)
}

func checkReadable(t *testing.T, buf remote.ByteBuffer) {
	// init
	msg := "hello world"
	var s string

	// test Peek()
	p, err := buf.Peek(len(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, string(p) == msg)

	// test Skip()
	err = buf.Skip(1 + len(msg))
	test.Assert(t, err == nil, err)

	// test Next()
	p, err = buf.Next(len(msg) - 1)
	test.Assert(t, err == nil, err)
	test.Assert(t, string(p) == msg[1:])

	// test ReadableLen()
	n := buf.ReadableLen()
	test.Assert(t, n == 3*len(msg), n)

	// test ReadLen()
	n = buf.ReadLen()
	test.Assert(t, n == len(msg)-1, n)

	// test ReadString()
	s, err = buf.ReadString(len(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, s == msg)

	// test ReadBinary
	p, err = buf.ReadBinary(len(msg))
	test.Assert(t, err == nil, err)
	test.Assert(t, string(p) == msg)
}

func checkUnwritable(t *testing.T, buf remote.ByteBuffer) {
	// init
	msg := "hello world"
	var n int

	// test Malloc()
	_, err := buf.Malloc(len(msg))
	test.Assert(t, err != nil)

	// test MallocLen()
	l := buf.MallocLen()
	test.Assert(t, l == -1, l)

	// test WriteString()
	_, err = buf.WriteString(msg)
	test.Assert(t, err != nil)

	// test WriteBinary()
	_, err = buf.WriteBinary([]byte(msg))
	test.Assert(t, err != nil)

	// test Flush()
	err = buf.Flush()
	test.Assert(t, err != nil)

	// test Write()
	n, err = buf.Write([]byte(msg))
	test.Assert(t, err != nil)
	test.Assert(t, n == -1, n)

	_, err = buf.Write(nil)
	test.Assert(t, err != nil)
}

func checkUnreadable(t *testing.T, buf remote.ByteBuffer) {
	// init
	msg := "hello world"
	p := make([]byte, len(msg))
	var n int

	// test Peek()
	_, err := buf.Peek(len(msg))
	test.Assert(t, err != nil)

	// test Skip()
	err = buf.Skip(1)
	test.Assert(t, err != nil)

	// test Next()
	_, err = buf.Next(len(msg) - 1)
	test.Assert(t, err != nil)

	// test ReadableLen()
	n = buf.ReadableLen()
	test.Assert(t, n == -1)

	// test ReadLen()
	n = buf.ReadLen()
	test.Assert(t, n == 0)

	// test ReadString()
	_, err = buf.ReadString(len(msg))
	test.Assert(t, err != nil)

	// test ReadBinary()
	_, err = buf.ReadBinary(len(msg))
	test.Assert(t, err != nil)

	// test Read()
	n, err = buf.Read(p)
	test.Assert(t, err != nil)
	test.Assert(t, n == -1, n)

	_, err = buf.Read(nil)
	test.Assert(t, err != nil)
}
