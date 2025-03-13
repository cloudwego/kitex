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

	// TTHeader unary methods only
	TTHeader Protocol = 1 << iota

	// Framed unary methods only
	Framed

	// HTTP unary methods only
	HTTP

	// GRPC indicates all methods (including unary and streaming) using grpc protocol.
	GRPC

	// HESSIAN2 unary methods only
	HESSIAN2

	// TTHeaderStreaming indicates all streaming methods using ttheader streaming protocol,
	// and it doesn't affect the protocol of all unary methods.
	TTHeaderStreaming

	// GRPCStreaming indicates all streaming methods using grpc protocol,
	// and it doesn't affect the protocol of all unary methods.
	// NOTE: only used in global config of the client side.
	GRPCStreaming

	// TTHeaderFramed unary methods only
	TTHeaderFramed = TTHeader | Framed
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
