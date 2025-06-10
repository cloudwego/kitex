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
	"errors"
	"sync"

	"github.com/bytedance/gopkg/lang/dirtmake"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/unsafex"

	"github.com/cloudwego/kitex/pkg/remote"
)

var rwPool = sync.Pool{New: func() any { return &bufferReadWriter{} }}

var _ remote.ByteBuffer = &bufferReadWriter{}

type bufferReadWriter struct {
	reader *bufiox.DefaultReader
	writer *bufiox.DefaultWriter
	status int
}

// newBufferReader creates a new remote.ByteBuffer using the given bufiox.DefaultReader.
func newBufferReader(reader *bufiox.DefaultReader) remote.ByteBuffer {
	rw := rwPool.Get().(*bufferReadWriter)
	rw.reader = reader
	rw.status = remote.BitReadable
	return rw
}

// newBufferWriter creates a new remote.ByteBuffer using the given bufiox.DefaultWriter.
func newBufferWriter(writer *bufiox.DefaultWriter) remote.ByteBuffer {
	rw := rwPool.Get().(*bufferReadWriter)
	rw.writer = writer
	rw.status = remote.BitWritable
	return rw
}

// newBufferReadWriter creates a new remote.ByteBuffer using the given bufioxReadWriter.
func newBufferReadWriter(irw bufioxReadWriter) remote.ByteBuffer {
	rw := rwPool.Get().(*bufferReadWriter)
	rw.writer = irw.Writer()
	rw.reader = irw.Reader()
	rw.status = remote.BitWritable | remote.BitReadable
	return rw
}

func (rw *bufferReadWriter) readable() bool {
	return rw.status&remote.BitReadable != 0
}

func (rw *bufferReadWriter) writable() bool {
	return rw.status&remote.BitWritable != 0
}

func (rw *bufferReadWriter) Next(n int) (p []byte, err error) {
	if !rw.readable() {
		return nil, errors.New("unreadable buffer, cannot support Next")
	}
	p, err = rw.reader.Next(n)
	return
}

func (rw *bufferReadWriter) Peek(n int) (buf []byte, err error) {
	if !rw.readable() {
		return nil, errors.New("unreadable buffer, cannot support Peek")
	}
	return rw.reader.Peek(n)
}

func (rw *bufferReadWriter) Skip(n int) (err error) {
	if !rw.readable() {
		return errors.New("unreadable buffer, cannot support Skip")
	}
	return rw.reader.Skip(n)
}

// Deprecated: Go net buffer does not implement this.
// All current usage of ReadableLen in Kitex are tied to netpoll connection.
func (rw *bufferReadWriter) ReadableLen() (n int) {
	panic("implement me")
}

func (rw *bufferReadWriter) ReadString(n int) (s string, err error) {
	if !rw.readable() {
		return "", errors.New("unreadable buffer, cannot support ReadString")
	}
	buf := dirtmake.Bytes(n, n)
	_, err = rw.reader.ReadBinary(buf)
	if err != nil {
		return "", err
	}
	return unsafex.BinaryToString(buf), nil
}

func (rw *bufferReadWriter) ReadBinary(p []byte) (n int, err error) {
	if !rw.readable() {
		return 0, errors.New("unreadable buffer, cannot support ReadBinary")
	}
	return rw.reader.ReadBinary(p)
}

func (rw *bufferReadWriter) Read(p []byte) (n int, err error) {
	if !rw.readable() {
		return -1, errors.New("unreadable buffer, cannot support Read")
	}
	return rw.reader.Read(p)
}

func (rw *bufferReadWriter) ReadLen() (n int) {
	return rw.reader.ReadLen()
}

func (rw *bufferReadWriter) Malloc(n int) (buf []byte, err error) {
	if !rw.writable() {
		return nil, errors.New("unwritable buffer, cannot support Malloc")
	}
	return rw.writer.Malloc(n)
}

func (rw *bufferReadWriter) WrittenLen() (length int) {
	if !rw.writable() {
		return -1
	}
	return rw.writer.WrittenLen()
}

func (rw *bufferReadWriter) WriteString(s string) (n int, err error) {
	if !rw.writable() {
		return -1, errors.New("unwritable buffer, cannot support WriteString")
	}
	return rw.writer.WriteBinary(unsafex.StringToBinary(s))
}

func (rw *bufferReadWriter) WriteBinary(b []byte) (n int, err error) {
	if !rw.writable() {
		return -1, errors.New("unwritable buffer, cannot support WriteBinary")
	}
	return rw.writer.WriteBinary(b)
}

func (rw *bufferReadWriter) Flush() (err error) {
	if !rw.writable() {
		return errors.New("unwritable buffer, cannot support Flush")
	}
	return rw.writer.Flush()
}

func (rw *bufferReadWriter) Write(p []byte) (n int, err error) {
	if !rw.writable() {
		return -1, errors.New("unwritable buffer, cannot support Write")
	}
	return rw.writer.WriteBinary(p)
}

func (rw *bufferReadWriter) Release(e error) (err error) {
	if rw.reader != nil {
		rw.reader.Release(e)
	}
	rw.zero()
	rwPool.Put(rw)
	return
}

func (rw *bufferReadWriter) AppendBuffer(buf remote.ByteBuffer) (err error) {
	panic("implement me")
}

// NewBuffer returns a new writable remote.ByteBuffer.
func (rw *bufferReadWriter) NewBuffer() remote.ByteBuffer {
	panic("unimplemented")
}

func (rw *bufferReadWriter) Bytes() (buf []byte, err error) {
	panic("unimplemented")
}

func (rw *bufferReadWriter) zero() {
	rw.reader = nil
	rw.writer = nil
	rw.status = 0
}
