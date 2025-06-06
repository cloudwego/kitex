/*
 * Copyright 2025 CloudWeGo Authors
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
	"net"
	"sync/atomic"
	"syscall"

	"github.com/cloudwego/gopkg/bufiox"
)

var (
	_ bufioxReadWriter = &cliConn{}
	_ bufioxReadWriter = &svrConn{}
)

type bufioxReadWriter interface {
	Reader() bufiox.Reader
	Writer() bufiox.Writer
}

// cliConn implements the net.Conn interface.
// IsActive function which is used to check the connection state when getting from connpool.
// FIXME: add proactive state check of long connection
type cliConn struct {
	net.Conn
	r      bufiox.Reader
	w      bufiox.Writer
	closed atomic.Bool // ensures Close is only executed once
}

func newCliConn(conn net.Conn) *cliConn {
	return &cliConn{
		Conn: conn,
		r:    bufiox.NewDefaultReader(conn),
		w:    bufiox.NewDefaultWriter(conn),
	}
}

func (c *cliConn) Reader() bufiox.Reader {
	return c.r
}

func (c *cliConn) Writer() bufiox.Writer {
	return c.w
}

func (c *cliConn) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		c.r.Release(nil)
		return c.Conn.Close()
	}
	return nil
}

func (c *cliConn) SyscallConn() (syscall.RawConn, error) {
	return c.Conn.(syscall.Conn).SyscallConn()
}

// svrConn implements the net.Conn interface.
type svrConn struct {
	net.Conn
	r      bufiox.Reader
	w      bufiox.Writer
	closed atomic.Bool
}

func newSvrConn(conn net.Conn) *svrConn {
	return &svrConn{
		Conn: conn,
		r:    bufiox.NewDefaultReader(conn),
		w:    bufiox.NewDefaultWriter(conn),
	}
}

func (bc *svrConn) Read(b []byte) (int, error) {
	return bc.r.ReadBinary(b)
}

func (bc *svrConn) Close() error {
	if bc.closed.CompareAndSwap(false, true) {
		bc.r.Release(nil)
		return bc.Conn.Close()
	}
	return nil
}

func (bc *svrConn) Reader() bufiox.Reader {
	return bc.r
}

func (bc *svrConn) Writer() bufiox.Writer {
	return bc.w
}
