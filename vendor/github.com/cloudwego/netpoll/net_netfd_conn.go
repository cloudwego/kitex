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

// +build darwin netbsd freebsd openbsd dragonfly linux

package netpoll

import (
	"log"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// Conn extends net.Conn, but supports getting the conn's fd.
type Conn interface {
	net.Conn

	// Fd return conn's fd, used by poll
	Fd() (fd int)
}

var _ Conn = &netFD{}

// Fd implements Conn.
func (c *netFD) Fd() (fd int) {
	return c.fd
}

// Read implements Conn.
func (c *netFD) Read(b []byte) (n int, err error) {
	n, err = syscall.Read(c.fd, b)
	if err != nil {
		if err == syscall.EAGAIN || err == syscall.EINTR {
			return 0, nil
		}
	}
	return n, err
}

// Write implements Conn.
func (c *netFD) Write(b []byte) (n int, err error) {
	n, err = syscall.Write(c.fd, b)
	if err != nil {
		if err == syscall.EAGAIN {
			return 0, nil
		}
	}
	return n, err
}

// Close will be executed only once.
func (c *netFD) Close() (err error) {
	if atomic.AddUint32(&c.closed, 1) != 1 {
		return nil
	}
	if c.fd > 0 {
		err = syscall.Close(c.fd)
		if err != nil {
			log.Printf("netFD[%d] close error: %s", c.fd, err.Error())
		}
	}
	return err
}

// LocalAddr implements Conn.
func (c *netFD) LocalAddr() (addr net.Addr) {
	return c.localAddr
}

// RemoteAddr implements Conn.
func (c *netFD) RemoteAddr() (addr net.Addr) {
	return c.remoteAddr
}

// SetKeepAlive implements Conn.
// TODO: only tcp conn is ok.
func (c *netFD) SetKeepAlive(second int) error {
	if !strings.HasPrefix(c.network, "tcp") {
		return nil
	}
	if second > 0 {
		return SetKeepAlive(c.fd, second)
	}
	return nil
}

// SetDeadline implements Conn.
func (c *netFD) SetDeadline(t time.Time) error {
	return Exception(ErrUnsupported, "SetDeadline")
}

// SetReadDeadline implements Conn.
func (c *netFD) SetReadDeadline(t time.Time) error {
	return Exception(ErrUnsupported, "SetReadDeadline")
}

// SetWriteDeadline implements Conn.
func (c *netFD) SetWriteDeadline(t time.Time) error {
	return Exception(ErrUnsupported, "SetWriteDeadline")
}
