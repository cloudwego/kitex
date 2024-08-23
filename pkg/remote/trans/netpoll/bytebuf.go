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
	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/netpoll"
)

func NewBufioxWriter(w netpoll.Writer) bufiox.Writer {
	return &netpollBufferWriter{writer: w}
}

type netpollBufferWriter struct {
	writer netpoll.Writer
}

// Malloc n bytes sequentially in the writer buffer.
func (b *netpollBufferWriter) Malloc(n int) (buf []byte, err error) {
	return b.writer.Malloc(n)
}

// MallocAck n bytes in the writer buffer.
func (b *netpollBufferWriter) MallocAck(n int) (err error) {
	return b.writer.MallocAck(n)
}

// WrittenLen returns the total length of the buffer written.
func (b *netpollBufferWriter) WrittenLen() (length int) {
	return b.writer.MallocLen()
}

// Write implement io.Writer
func (b *netpollBufferWriter) Write(p []byte) (n int, err error) {
	wb, err := b.writer.Malloc(len(p))
	return copy(wb, p), err
}

// WriteBinary writes the []byte directly. Callers must guarantee that the []byte doesn't change.
func (b *netpollBufferWriter) WriteBinary(p []byte) (n int, err error) {
	return b.writer.WriteBinary(p)
}

// WriteDirect is a way to write []byte without copying, and splits the original buffer.
func (b *netpollBufferWriter) WriteDirect(p []byte, remainCap int) error {
	return b.writer.WriteDirect(p, remainCap)
}

// Flush writes any malloc data to the underlying io.Writer.
// The malloced buffer must be set correctly.
func (b *netpollBufferWriter) Flush() (err error) {
	return b.writer.Flush()
}

func NewBufioxReader(r netpoll.Reader) bufiox.Reader {
	return &netpollBufferReader{reader: r}
}

type netpollBufferReader struct {
	reader   netpoll.Reader
	readSize int
}

// Next reads n bytes sequentially, returns the original address.
func (b *netpollBufferReader) Next(n int) (p []byte, err error) {
	if p, err = b.reader.Next(n); err == nil {
		b.readSize += n
	}
	return
}

// Peek returns the next n bytes without advancing the reader.
func (b *netpollBufferReader) Peek(n int) (buf []byte, err error) {
	return b.reader.Peek(n)
}

// Skip is used to skip n bytes, it's much faster than Next.
// Skip will not cause release.
func (b *netpollBufferReader) Skip(n int) (err error) {
	return b.reader.Skip(n)
}

// Read implement io.Reader
func (b *netpollBufferReader) Read(p []byte) (n int, err error) {
	return b.ReadBinary(p)
}

// ReadBinary like ReadString.
// Returns a copy of original buffer.
func (b *netpollBufferReader) ReadBinary(p []byte) (n int, err error) {
	var buf []byte
	n = len(p)
	if buf, err = b.reader.Next(n); err != nil {
		return 0, err
	}
	copy(p, buf)
	b.readSize += n
	return
}

// ReadLen returns the size already read.
func (b *netpollBufferReader) ReadLen() (n int) {
	return b.readSize
}

func (b *netpollBufferReader) Release(e error) error {
	b.readSize = 0
	return b.reader.Release()
}
