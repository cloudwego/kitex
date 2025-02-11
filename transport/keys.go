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

// Package transport provides predefined transport protocol.
package transport

// Protocol indicates the transport protocol.
type Protocol int

// Predefined transport protocols.
const (
	PurePayload Protocol = 0

	TTHeader          Protocol = 1 << iota // unary methods only
	Framed                                 // unary methods only
	HTTP                                   // unary methods only
	GRPC                                   // both support unary and streaming methods
	HESSIAN2                               // unary methods only
	TTHeaderStreaming                      // streaming methods only
	GRPCStreaming                          // streaming methods only

	TTHeaderFramed = TTHeader | Framed // unary methods only
)

// Unknown indicates the protocol is unknown.
const Unknown = "Unknown"

// String prints human readable information.
func (tp Protocol) String() string {
	var s string
	if tp&TTHeader == TTHeader {
		s += "TTHeader|"
	}
	if tp&Framed == Framed {
		s += "Framed|"
	}
	if tp&HTTP == HTTP {
		s += "HTTP|"
	}
	if tp&GRPC == GRPC {
		s += "GRPC|"
	}
	if tp&HESSIAN2 == HESSIAN2 {
		s += "Hessian2|"
	}
	if tp&TTHeaderStreaming == TTHeaderStreaming {
		s += "TTHeaderStreaming|"
	}
	if tp&GRPCStreaming == GRPCStreaming {
		s += "GRPCStreaming|"
	}
	if s != "" {
		return s[:len(s)-1]
	}
	return "PurePayload"
}
