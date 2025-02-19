/*
 * Copyright 2022 CloudWeGo Authors
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
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/cloudwego/kitex/pkg/remote"
)

const (
	alive  uint32 = 0
	closed uint32 = 1
	//
	defaultProbeInterval = 2 * time.Second
)

// NewDialer returns the default go net dialer.
func NewDialer() remote.Dialer {
	return newDialerWithInterval(defaultProbeInterval)
}

func newDialerWithInterval(interval time.Duration) remote.Dialer {
	d := &dialer{probeTime: interval}
	go d.monitor()
	return d
}

type dialer struct {
	net.Dialer
	cm        sync.Map // *connWithState->struct{}{}
	probeTime time.Duration
}

type connWithState struct {
	net.Conn
	state uint32 // 0: alive, 1: closed
}

func (c *connWithState) IsActive() bool {
	return atomic.LoadUint32(&c.state) == alive
}

func (d *dialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := d.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	c := &connWithState{conn, alive}
	d.cm.Store(c, struct{}{})
	return c, nil
}

func (d *dialer) monitor() {
	for range time.Tick(d.probeTime) {
		d.cm.Range(func(k, v interface{}) bool {
			c := k.(*connWithState)
			if d.probe(c) {
				atomic.StoreUint32(&c.state, closed)
				d.cm.Delete(k)
			}
			return true
		})
	}
}

// probe uses syscall to detect if the connection is still alive.
func (d *dialer) probe(c *connWithState) (closed bool) {
	type syscaller interface {
		SyscallConn() (syscall.RawConn, error)
	}
	sysConn, ok := c.Conn.(syscaller)
	if !ok {
		return false
	}
	if rawConn, e := sysConn.SyscallConn(); e == nil {
		err := rawConn.Control(func(fd uintptr) {
			buf := make([]byte, 1)
			n, _, errno := syscall.Recvfrom(int(fd), buf, syscall.MSG_PEEK)
			if !(n > 0 || errors.Is(errno, syscall.EAGAIN)) {
				closed = true
			}
		})
		if err != nil {
			closed = true
			// FIXME: how to handle error?
			//klog.Errorf("KITEX: go net conn probe failed, err=%v", err)
		}
	}
	return closed
}
