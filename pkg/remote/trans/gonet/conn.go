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
	"errors"
	"net"
	"sync/atomic"

	"github.com/cloudwego/gopkg/bufiox"
)

var (
	_             bufioxReadWriter = &cliConn{}
	_             bufioxReadWriter = &svrConn{}
	errConnClosed error            = errors.New("connection has been closed")
)

type bufioxReadWriter interface {
	Reader() *bufiox.DefaultReader
	Writer() *bufiox.DefaultWriter
}

// cliConn implements the net.Conn interface.
// FIXME: add proactive state check of long connection
type cliConn struct {
	net.Conn
	r      *bufiox.DefaultReader
	w      *bufiox.DefaultWriter
	closed atomic.Bool // ensures Close is only executed once
}

func newCliConn(conn net.Conn) *cliConn {
	return &cliConn{
		Conn: conn,
		r:    bufiox.NewDefaultReader(conn),
		w:    bufiox.NewDefaultWriter(conn),
	}
}

func (c *cliConn) Reader() *bufiox.DefaultReader {
	return c.r
}

func (c *cliConn) Writer() *bufiox.DefaultWriter {
	return c.w
}

func (c *cliConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

func (c *cliConn) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		c.r.Release(nil)
		return c.Conn.Close()
	}
	return errConnClosed
}

// svrConn implements the net.Conn interface.
type svrConn struct {
	net.Conn
	r      *bufiox.DefaultReader
	w      *bufiox.DefaultWriter
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
	return bc.r.Read(b)
}

func (bc *svrConn) Close() error {
	if bc.closed.CompareAndSwap(false, true) {
		bc.r.Release(nil)
		return bc.Conn.Close()
	}
	return errConnClosed
}

func (bc *svrConn) Reader() *bufiox.DefaultReader {
	return bc.r
}

func (bc *svrConn) Writer() *bufiox.DefaultWriter {
	return bc.w
}
