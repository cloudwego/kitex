// Copyright 2023 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"net"
	"strconv"
	"sync/atomic"
	"time"
)

var curPort int32 = UnixUserPortStart

const (
	UnixUserPortStart = 1023
	UnixUserPortEnd   = 49151
)

// GetLocalAddress return a local address starting from 1024
// This API ensures no repeated addr returned in one UNIX OS
func GetLocalAddress() string {
start:
	nextPort := curPort + 1
	addr := "127.0.0.1:" + strconv.Itoa(int(nextPort))
	for IsAddressInUse(addr) {
		nextPort += 1
	}
	if nextPort > UnixUserPortEnd {
		panic("too much cocurrent local addresses are being used!")
	}
	if !atomic.CompareAndSwapInt32(&curPort, curPort, nextPort) {
		goto start
	}
	return addr
}

// tells if a net address is already in use.
func IsAddressInUse(address string) bool {
	// Attempt to establish a TCP connection to the address
	conn, err := net.DialTimeout("tcp", address, 1*time.Second)
	if err != nil {
		// If there's an error, the address is likely not in use or not reachable
		return false
	}
	defer conn.Close()

	// If the connection is successful, the address is in use
	return true
}
