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

	internalRemote "github.com/cloudwego/kitex/internal/remote"
)

var (
	_ internalRemote.SetState = &cliConn{}
	_ bufioxReadWriter        = &cliConn{}
	_ bufioxReadWriter        = &svrConn{}
)

type bufioxReadWriter interface {
	Reader() bufiox.Reader
	Writer() bufiox.Writer
}

// cliConn implements the net.Conn interface.
// IsActive function which is used to check the connection state when getting from connpool.
type cliConn struct {
	net.Conn
	r        bufiox.Reader
	w        bufiox.Writer
	inactive atomic.Bool // set inactive in connpool's detection logic.
	closed   atomic.Bool // ensures Close is only executed once
}

func newCliConn(conn net.Conn) *cliConn {
	return &cliConn{
		Conn: conn,
		r:    getReader(conn),
		w:    getWriter(conn),
	}
}

func (c *cliConn) Reader() bufiox.Reader {
	return c.r
}

func (c *cliConn) Writer() bufiox.Writer {
	return c.w
}

func (c *cliConn) IsActive() bool {
	return !c.inactive.Load()
}

func (c *cliConn) SetState(inactive bool) {
	c.inactive.Store(inactive)
}

func (c *cliConn) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		c.r.Release(nil)
		recycleReader(c.r.(*bufiox.DefaultReader))
		recycleWriter(c.w.(*bufiox.DefaultWriter))
		return c.Conn.Close()
	}
	return nil
}

func (c *cliConn) SyscallConn() (syscall.RawConn, error) {
	return c.Conn.(syscall.Conn).SyscallConn()
}

var _ internalRemote.OnceExecutor = &svrConn{}

// svrConn implements the net.Conn interface.
type svrConn struct {
	net.Conn
	r        bufiox.Reader
	w        bufiox.Writer
	closed   atomic.Bool
	onceDone atomic.Bool
}

func newSvrConn(c net.Conn) *svrConn {
	return &svrConn{
		Conn: c,
		r:    getReader(c),
		w:    getWriter(c),
	}
}

func (bc *svrConn) Read(b []byte) (int, error) {
	return bc.r.ReadBinary(b)
}

func (bc *svrConn) Close() error {
	if bc.closed.CompareAndSwap(false, true) {
		bc.r.Release(nil)
		recycleReader(bc.r.(*bufiox.DefaultReader))
		recycleWriter(bc.w.(*bufiox.DefaultWriter))
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

func (bc *svrConn) Done() bool {
	return bc.onceDone.Load()
}

func (bc *svrConn) Do() bool {
	return bc.onceDone.CompareAndSwap(false, true)
}
