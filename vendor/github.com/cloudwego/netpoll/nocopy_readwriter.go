// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package netpoll

import (
	"fmt"
	"io"
)

const maxReadCycle = 16

func newZCReader(r io.Reader) *zcReader {
	return &zcReader{
		r:   r,
		buf: NewLinkBuffer(),
	}
}

var _ Reader = &zcReader{}

// zcReader implements Reader.
type zcReader struct {
	r   io.Reader
	buf *LinkBuffer
}

// Next implements Reader.
func (r *zcReader) Next(n int) (p []byte, err error) {
	if err = r.waitRead(n); err != nil {
		return p, err
	}
	return r.buf.Next(n)
}

// Peek implements Reader.
func (r *zcReader) Peek(n int) (buf []byte, err error) {
	if err = r.waitRead(n); err != nil {
		return buf, err
	}
	return r.buf.Peek(n)
}

// Skip implements Reader.
func (r *zcReader) Skip(n int) (err error) {
	if err = r.waitRead(n); err != nil {
		return err
	}
	return r.buf.Skip(n)
}

// Release implements Reader.
func (r *zcReader) Release() (err error) {
	return r.buf.Release()
}

// Slice implements Reader.
func (r *zcReader) Slice(n int) (reader Reader, err error) {
	if err = r.waitRead(n); err != nil {
		return nil, err
	}
	return r.buf.Slice(n)
}

// Len implements Reader.
func (r *zcReader) Len() (length int) {
	return r.buf.Len()
}

// ReadString implements Reader.
func (r *zcReader) ReadString(n int) (s string, err error) {
	if err = r.waitRead(n); err != nil {
		return s, err
	}
	return r.buf.ReadString(n)
}

// ReadBinary implements Reader.
func (r *zcReader) ReadBinary(n int) (p []byte, err error) {
	if err = r.waitRead(n); err != nil {
		return p, err
	}
	return r.buf.ReadBinary(n)
}

// ReadByte implements Reader.
func (r *zcReader) ReadByte() (b byte, err error) {
	if err = r.waitRead(1); err != nil {
		return b, err
	}
	return r.buf.ReadByte()
}

func (r *zcReader) Until(delim byte) (line []byte, err error) {
	return r.buf.Until(delim)
}

func (r *zcReader) waitRead(n int) (err error) {
	for r.buf.Len() < n {
		err = r.fill(n)
		if err != nil {
			if err == io.EOF {
				err = Exception(ErrEOF, "")
			}
			return err
		}
	}
	return nil
}

// fill buffer to greater than n, range no more than 16 times.
func (r *zcReader) fill(n int) (err error) {
	var buf []byte
	var num int
	for i := 0; i < maxReadCycle && r.buf.Len() < n && err == nil; i++ {
		buf, err = r.buf.Malloc(block4k)
		if err != nil {
			return err
		}
		num, err = r.r.Read(buf)
		if num < 0 {
			if err == nil {
				err = fmt.Errorf("zcReader fill negative count[%d]", num)
			}
			num = 0
		}
		r.buf.MallocAck(num)
		r.buf.Flush()
		if err != nil {
			return err
		}
	}
	return err
}

func newZCWriter(w io.Writer) *zcWriter {
	return &zcWriter{
		w:   w,
		buf: NewLinkBuffer(),
	}
}

var _ Writer = &zcWriter{}

// zcWriter implements Writer.
type zcWriter struct {
	w   io.Writer
	buf *LinkBuffer
}

// Malloc implements Writer.
func (w *zcWriter) Malloc(n int) (buf []byte, err error) {
	return w.buf.Malloc(n)
}

// MallocLen implements Writer.
func (w *zcWriter) MallocLen() (length int) {
	return w.buf.MallocLen()
}

// Flush implements Writer.
func (w *zcWriter) Flush() (err error) {
	w.buf.Flush()
	n, err := w.w.Write(w.buf.Bytes())
	if n > 0 {
		w.buf.Skip(n)
		w.buf.Release()
	}
	return err
}

// MallocAck implements Writer.
func (w *zcWriter) MallocAck(n int) (err error) {
	return w.buf.MallocAck(n)
}

// Append implements Writer.
func (w *zcWriter) Append(w2 Writer) (err error) {
	return w.buf.Append(w2)
}

// WriteString implements Writer.
func (w *zcWriter) WriteString(s string) (n int, err error) {
	return w.buf.WriteString(s)
}

// WriteBinary implements Writer.
func (w *zcWriter) WriteBinary(b []byte) (n int, err error) {
	return w.buf.WriteBinary(b)
}

// WriteDirect implements Writer.
func (w *zcWriter) WriteDirect(p []byte, remainCap int) error {
	return w.buf.WriteDirect(p, remainCap)
}

// WriteByte implements Writer.
func (w *zcWriter) WriteByte(b byte) (err error) {
	return w.buf.WriteByte(b)
}

// zcWriter implements ReadWriter.
type zcReadWriter struct {
	*zcReader
	*zcWriter
}

func newIOReader(r Reader) *ioReader {
	return &ioReader{
		r: r,
	}
}

var _ io.Reader = &ioReader{}

// ioReader implements io.Reader.
type ioReader struct {
	r Reader
}

// Read implements io.Reader.
func (r *ioReader) Read(p []byte) (n int, err error) {
	l := len(p)
	if l == 0 {
		return 0, nil
	}
	// read min(len(p), buffer.Len)
	if has := r.r.Len(); has < l {
		l = has
	}
	if l == 0 {
		return 0, io.EOF
	}
	src, err := r.r.Next(l)
	if err != nil {
		return 0, err
	}
	n = copy(p, src)
	err = r.r.Release()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func newIOWriter(w Writer) *ioWriter {
	return &ioWriter{
		w: w,
	}
}

var _ io.Writer = &ioWriter{}

// ioWriter implements io.Writer.
type ioWriter struct {
	w Writer
}

// Write implements io.Writer.
func (w *ioWriter) Write(p []byte) (n int, err error) {
	dst, err := w.w.Malloc(len(p))
	if err != nil {
		return 0, err
	}
	n = copy(dst, p)
	err = w.w.Flush()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ioReadWriter implements io.ReadWriter.
type ioReadWriter struct {
	*ioReader
	*ioWriter
}
