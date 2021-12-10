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

package nphttp2

import (
	"errors"
	"net"
	"sync"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/remote"
)

type buffer struct {
	buf      *netpoll.LinkBuffer
	conn     net.Conn
	readSize int
}

var (
	_ remote.ByteBuffer = (*buffer)(nil)

	bufPool sync.Pool
)

func init() {
	bufPool.New = newBuffer
}

func newBuffer() interface{} {
	return &buffer{
		buf: netpoll.NewLinkBuffer(),
	}
}

func newByteBuffer(conn net.Conn) remote.ByteBuffer {
	bytebuf := bufPool.Get().(*buffer)
	bytebuf.conn = conn
	return bytebuf
}

func (b *buffer) readN(n int) error {
	readLen := b.buf.Len()
	if readLen >= n {
		return nil
	}

	d, err := b.buf.Malloc(n - readLen)
	if err != nil {
		return err
	}
	if _, err = b.conn.Read(d); err != nil {
		return err
	}
	if err = b.buf.Flush(); err != nil {
		return err
	}
	b.readSize += n
	return nil
}

func (b *buffer) Read(d []byte) (n int, err error) {
	return
}

func (b *buffer) Write(p []byte) (n int, err error) {
	return
}

// impl ByteBuffer
func (b *buffer) Next(n int) (p []byte, err error) {
	if err = b.readN(n); err != nil {
		return nil, err
	}
	return b.buf.Next(n)
}

func (b *buffer) Peek(n int) (buf []byte, err error) {
	if err = b.readN(n); err != nil {
		return nil, err
	}
	return b.buf.Peek(n)
}

func (b *buffer) Skip(n int) (err error) {
	if err = b.readN(n); err != nil {
		return err
	}
	return b.buf.Skip(n)
}

func (b *buffer) Release(e error) (err error) {
	_ = b.buf.Close()
	b.buf = netpoll.NewLinkBuffer()
	return nil
}

func (b *buffer) ReadableLen() (n int) {
	return b.buf.Len()
}

func (b *buffer) ReadLen() (n int) {
	return b.readSize
}

func (b *buffer) ReadString(n int) (s string, err error) {
	if err = b.readN(n); err != nil {
		return
	}
	return b.buf.ReadString(n)
}

func (b *buffer) ReadBinary(n int) (p []byte, err error) {
	if err = b.readN(n); err != nil {
		return
	}
	return b.buf.ReadBinary(n)
}

func (b *buffer) Malloc(n int) (buf []byte, err error) {
	return b.buf.Malloc(n)
}

func (b *buffer) MallocLen() (length int) {
	return b.buf.MallocLen()
}

func (b *buffer) WriteString(s string) (n int, err error) {
	return b.buf.WriteString(s)
}

func (b *buffer) WriteBinary(d []byte) (n int, err error) {
	return b.buf.WriteBinary(d)
}

func (b *buffer) WriteDirect(d []byte, remainCap int) error {
	return b.buf.WriteDirect(d, remainCap)
}

func (b *buffer) Flush() (err error) {
	if err = b.buf.Flush(); err != nil {
		return
	}

	wb, _ := b.buf.ReadBinary(b.buf.Len())
	_, err = b.conn.Write(wb)
	return
}

func (b *buffer) NewBuffer() remote.ByteBuffer {
	return newByteBuffer(b.conn)
}

// ErrUnimplemented return unimplemented error
var ErrUnimplemented = errors.New("unimplemented")

func (b *buffer) AppendBuffer(buf remote.ByteBuffer) (err error) {
	return ErrUnimplemented
}

func (b *buffer) Bytes() (buf []byte, err error) {
	return nil, ErrUnimplemented
}
