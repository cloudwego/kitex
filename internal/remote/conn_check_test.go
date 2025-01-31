//go:build !windows

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

package remote

import (
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

type mockConn struct {
	net.Conn
	closed atomic.Bool
}

func (m *mockConn) SetState(c bool) {
	m.closed.Store(c)
}

func (m *mockConn) SyscallConn() (syscall.RawConn, error) {
	if sc, ok := m.Conn.(syscall.Conn); ok {
		return sc.SyscallConn()
	}
	return nil, errors.New("not syscall.Conn")
}

func TestConnectionStateCheck(t *testing.T) {
	// wrong connection type
	err := ConnectionStateCheck(net.Pipe())
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "conn is not a syscall.Conn"))

	ln, err := net.Listen("tcp", "127.0.0.1:0") // 本地端口自动分配
	test.Assert(t, err == nil, err)
	defer ln.Close()

	done := make(chan net.Conn)
	go func() {
		conn, e := ln.Accept()
		test.Assert(t, e == nil)
		done <- conn
	}()

	clientConn, err := net.Dial("tcp", ln.Addr().String())
	test.Assert(t, err == nil, err)

	serverConn := <-done
	serverConnWithState := &mockConn{Conn: serverConn}
	// check, not closed
	err = ConnectionStateCheck(serverConnWithState)
	test.Assert(t, err == nil, err)
	test.Assert(t, !serverConnWithState.closed.Load())

	// close conn
	clientConn.Close()
	time.Sleep(100 * time.Millisecond)

	// check, closed
	err = ConnectionStateCheck(serverConnWithState)
	test.Assert(t, err == nil, err)
	test.Assert(t, serverConnWithState.closed.Load())
}
