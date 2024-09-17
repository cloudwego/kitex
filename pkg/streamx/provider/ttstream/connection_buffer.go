/*
 * Copyright 2024 CloudWeGo Authors
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

package ttstream

import (
	"runtime"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/netpoll/mux"
)

var _ bufiox.Reader = (*connBuffer)(nil)
var _ bufiox.Writer = (*connBuffer)(nil)
var gomaxprocs = runtime.GOMAXPROCS(0)

func newConnBuffer(conn netpoll.Connection) *connBuffer {
	queue := mux.NewShardQueue(gomaxprocs, conn)
	return &connBuffer{
		readBuffer:  conn.Reader(),
		writeQueue:  queue,
		writeBuffer: netpoll.NewLinkBuffer(),
	}
}

type connBuffer struct {
	readBuffer netpoll.Reader
	readSize   int

	writeQueue  *mux.ShardQueue
	writeBuffer netpoll.Writer
	writeSize   int
}

// === Read ===

func (c *connBuffer) Next(n int) (p []byte, err error) {
	p, err = c.readBuffer.Next(n)
	c.readSize += len(p)
	return p, err
}

func (c *connBuffer) ReadBinary(bs []byte) (n int, err error) {
	n = len(bs)
	buf, err := c.readBuffer.Next(n)
	if err != nil {
		return 0, err
	}
	copy(bs, buf)
	c.readSize += n
	return n, nil
}

func (c *connBuffer) Peek(n int) (buf []byte, err error) {
	return c.readBuffer.Peek(n)
}

func (c *connBuffer) Skip(n int) (err error) {
	err = c.readBuffer.Skip(n)
	if err != nil {
		return err
	}
	c.readSize += n
	return nil
}

func (c *connBuffer) ReadLen() (n int) {
	return c.readSize
}

func (c *connBuffer) Release(e error) (err error) {
	c.readSize = 0
	return c.readBuffer.Release()
}

// === Write ===

func (c *connBuffer) Malloc(n int) (buf []byte, err error) {
	c.writeSize += n
	return c.writeBuffer.Malloc(n)
}

func (c *connBuffer) WriteBinary(bs []byte) (n int, err error) {
	n, err = c.writeBuffer.WriteBinary(bs)
	c.writeSize += n
	return n, err
}

func (c *connBuffer) WrittenLen() (length int) {
	return c.writeSize
}

func (c *connBuffer) Flush() (err error) {
	c.writeSize = 0
	writeBuffer := c.writeBuffer
	c.writeBuffer = netpoll.NewLinkBuffer()
	c.writeQueue.Add(func() (netpoll.Writer, bool) {
		return writeBuffer, false
	})
	return nil
}
