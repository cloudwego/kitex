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
	"fmt"
	"net"
	"sync/atomic"
	"syscall"

	"github.com/cloudwego/gopkg/bufiox"
)

var (
	_             bufioxReadWriter = &cliConn{}
	_             syscall.Conn     = &cliConn{}
	_             bufioxReadWriter = &svrConn{}
	errConnClosed                  = errors.New("connection has been closed")
)

type bufioxReadWriter interface {
	Reader() *bufiox.DefaultReader
	Writer() *bufiox.DefaultWriter
}

// cliConn implements the net.Conn interface.
type cliConn struct {
	net.Conn
	r        *bufiox.DefaultReader
	w        *bufiox.DefaultWriter
	closed   uint32 // 1: closed
	inactive uint32 // 1: inactive, set by proactive check logic in connection pool
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

// IsActive is used to check if the connection is active.
func (c *cliConn) IsActive() bool {
	return atomic.LoadUint32(&c.inactive) == 0
}

func (c *cliConn) SetConnState(inactive bool) {
	if inactive {
		atomic.StoreUint32(&c.inactive, 1)
	} else {
		atomic.StoreUint32(&c.inactive, 0)
	}
}

func (c *cliConn) SyscallConn() (syscall.RawConn, error) {
	sc, ok := c.Conn.(syscall.Conn)
	if !ok {
		return nil, fmt.Errorf("conn is not a syscall.Conn, got %T", c.Conn)
	}
	return sc.SyscallConn()
}

func (c *cliConn) Close() error {
	if atomic.CompareAndSwapUint32(&c.closed, 0, 1) {
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
	closed uint32 // 1: closed
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
	if atomic.CompareAndSwapUint32(&bc.closed, 0, 1) {
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
