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

package gonet

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
)

var _ bufioxReadWriter = &mockBufioxReadWriter{}

type mockBufioxReadWriter struct {
	r *bufiox.DefaultReader
	w *bufiox.DefaultWriter
}

func (m *mockBufioxReadWriter) Reader() *bufiox.DefaultReader {
	return m.r
}

func (m *mockBufioxReadWriter) Writer() *bufiox.DefaultWriter {
	return m.w
}

var (
	msg    = "hello world"
	msgLen = len(msg)
)

// TestBufferReadWrite test bytebuf write and read success.
func TestBufferReadWrite(t *testing.T) {
	var (
		reader   = bufiox.NewDefaultReader(strings.NewReader(strings.Repeat(msg, 5)))
		writer   = bufiox.NewDefaultWriter(bytes.NewBufferString(strings.Repeat(msg, 5)))
		bufioxRW = &mockBufioxReadWriter{r: reader, w: writer}
		bufRW    = newBufferReadWriter(bufioxRW)
	)

	testRead(t, bufRW)
	testWrite(t, bufRW)
}

// TestBufferWrite test write success.
func TestBufferWrite(t *testing.T) {
	wi := bufiox.NewDefaultWriter(bytes.NewBufferString(strings.Repeat(msg, 5)))

	buf := newBufferWriter(wi)
	testWrite(t, buf)
}

// TestBufferRead test read success.
func TestBufferRead(t *testing.T) {
	ri := strings.NewReader(strings.Repeat(msg, 5))

	buf := newBufferReader(bufiox.NewDefaultReader(ri))
	testRead(t, buf)
}

func testRead(t *testing.T, buf remote.ByteBuffer) {
	var (
		p       []byte
		s       string
		err     error
		readLen int
	)

	// Skip
	p, err = buf.Peek(msgLen)
	if err != nil {
		t.Fatalf("Peek failed, err=%s", err.Error())
	}
	test.Assert(t, string(p) == msg)
	if n := buf.ReadLen(); n != readLen {
		t.Fatalf("ReadLen expect %d, but got %d", readLen, n)
	}

	// Skip
	readLen += 1 + msgLen
	err = buf.Skip(1 + msgLen)
	if err != nil {
		t.Fatalf("Skip failed, err=%s", err.Error())
	}
	if n := buf.ReadLen(); n != readLen {
		t.Fatalf("ReadLen expect %d, but got %d", readLen, n)
	}

	// Next
	readLen += msgLen - 1
	p, err = buf.Next(msgLen - 1)
	if err != nil {
		t.Fatalf("Next failed, err=%s", err.Error())
	}
	test.Assert(t, string(p) == msg[1:])
	if n := buf.ReadLen(); n != readLen {
		t.Fatalf("ReadLen expect %d, but got %d", readLen, n)
	}

	// ReadString
	readLen += msgLen
	s, err = buf.ReadString(msgLen)
	if err != nil {
		t.Fatalf("ReadString failed, err=%s", err.Error())
	}
	test.Assert(t, s == msg)
	if n := buf.ReadLen(); n != readLen {
		t.Fatalf("ReadLen expect %d, but got %d", readLen, n)
	}

	// ReadBinary
	p = make([]byte, msgLen)
	nn, err := buf.ReadBinary(p)
	readLen += nn
	if err != nil {
		t.Fatalf("ReadBinary failed, err=%s", err.Error())
	}
	test.Assert(t, nn == len(p))
	test.Assert(t, string(p) == msg)
	if n := buf.ReadLen(); n != readLen {
		t.Fatalf("ReadLen expect %d, but got %d", readLen, n)
	}

	// Read
	p = make([]byte, msgLen)
	nn, err = buf.Read(p)
	readLen += nn
	if err != nil {
		t.Fatalf("ReadBinary failed, err=%s", err.Error())
	}
	test.Assert(t, nn == len(p))
	test.Assert(t, string(p) == msg)
	if n := buf.ReadLen(); n != readLen {
		t.Fatalf("ReadLen expect %d, but got %d", readLen, n)
	}
}

func testWrite(t *testing.T, buf remote.ByteBuffer) {
	p, err := buf.Malloc(msgLen)
	if err != nil {
		t.Logf("Malloc failed, err=%s", err.Error())
		t.FailNow()
	}
	test.Assert(t, len(p) == msgLen)
	copy(p, msg)

	l := buf.WrittenLen()
	test.Assert(t, l == msgLen)

	l, err = buf.WriteString(msg)
	if err != nil {
		t.Logf("WriteString failed, err=%s", err.Error())
		t.FailNow()
	}
	test.Assert(t, l == msgLen)

	l, err = buf.WriteBinary([]byte(msg))
	if err != nil {
		t.Logf("WriteBinary failed, err=%s", err.Error())
		t.FailNow()
	}
	test.Assert(t, l == msgLen)

	err = buf.Flush()
	if err != nil {
		t.Logf("Flush failed, err=%s", err.Error())
		t.FailNow()
	}
}
