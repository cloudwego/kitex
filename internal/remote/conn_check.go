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
	"fmt"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// ConnectionStateCheck uses unix.Poll to detect the connection state
// Since the connections are stored in the pool, we treat any POLLIN event as connection close and set the connection state to closed.
func ConnectionStateCheck(conns ...net.Conn) error {
	pollFds := make([]unix.PollFd, 0, len(conns))

	for _, conn := range conns {
		sysConn, ok := conn.(syscall.Conn)
		if !ok {
			return fmt.Errorf("conn is not a syscall.Conn, got %T", conn)
		}
		rawConn, err := sysConn.SyscallConn()
		if err != nil {
			return err
		}
		var fd int
		err = rawConn.Control(func(fileDescriptor uintptr) {
			fd = int(fileDescriptor)
		})
		if err != nil {
			return err
		}
		pollFds = append(pollFds, unix.PollFd{Fd: int32(fd), Events: unix.POLLIN})
	}

	n, err := unix.Poll(pollFds, 0)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	for i := 0; i < len(pollFds); i++ {
		if pollFds[i].Revents&unix.POLLIN != 0 {
			// the connection should not receive any data, POLLIN means FIN or RST
			// set the state
			if s, ok := conns[i].(SetState); ok {
				s.SetState(true)
			}
		}
	}
	return nil
}
