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

// Package streaming interface
package streaming

import (
	"context"
	"io"
)

// Stream both client and server stream
type Stream interface {
	// Context the stream context.Context
	Context() context.Context
	// RecvMsg recvive message from peer
	// will block until an error or a message received
	// not concurrent-safety
	RecvMsg(m interface{}) error
	// SendMsg send message to peer
	// will block until an error or enough buffer to send
	// not concurrent-safety
	SendMsg(m interface{}) error
	// not concurrent-safety with SendMsg
	io.Closer
}

// Args endpoint request
type Args struct {
	Stream Stream
}

// Result endpoint response
type Result struct {
	Stream Stream
}
