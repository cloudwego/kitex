/*
 * Copyright 2021 CloudWeGo Authors
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

package utils

import "net"

var _ net.Addr = &NetAddr{}

// NetAddr implements the net.Addr interface.
type NetAddr struct {
	network string
	address string
}

// NewNetAddr creates a new NetAddr object with the network and address provided.
func NewNetAddr(network, address string) net.Addr {
	return &NetAddr{network, address}
}

// Network implements the net.Addr interface.
func (na *NetAddr) Network() string {
	return na.network
}

// String implements the net.Addr interface.
func (na *NetAddr) String() string {
	return na.address
}
