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

package netpoll

import (
	"net"
	"time"
)

// CloseCallback will be called when the connection is closed.
// Return: error is unused which will be ignored directly.
type CloseCallback func(connection Connection) error

// Connection supports reading and writing simultaneously,
// but does not support simultaneous reading or writing by multiple goroutines.
// It maintains its own input/output buffer, and provides nocopy API for reading and writing.
type Connection interface {
	// Connection extends net.Conn, just for interface compatibility.
	// It's not recommended to use net.Conn API except for io.Closer.
	net.Conn

	// The recommended API for nocopy reading and writing.
	// Reader will return nocopy buffer data, or error after timeout which set by SetReadTimeout.
	Reader() Reader
	// Writer will write data to the connection by NIO mode,
	// so it will return an error only when the connection isn't Active.
	Writer() Writer

	// IsActive checks whether the connection is active or not.
	IsActive() bool

	// SetReadTimeout sets the timeout for future Read calls wait.
	// A zero value for timeout means Reader will not timeout.
	SetReadTimeout(timeout time.Duration) error

	// SetIdleTimeout sets the idle timeout of connections.
	// Idle connections that exceed the set timeout are no longer guaranteed to be active,
	// but can be checked by calling IsActive.
	SetIdleTimeout(timeout time.Duration) error

	// SetOnRequest can set or replace the OnRequest method for a connection, but can't be set to nil.
	// Although SetOnRequest avoids data race, it should still be used before transmitting data.
	// Replacing OnRequest while processing data may cause unexpected behavior and results.
	// Generally, the server side should uniformly set the OnRequest method for each connection via NewEventLoop,
	// which is set when the connection is initialized.
	// On the client side, if necessary, make sure that OnRequest is set before sending data.
	SetOnRequest(on OnRequest) error

	// AddCloseCallback can add hangup callback for a connection, which will be called when connection closing.
	// This is very useful for cleaning up idle connections. For instance, you can use callbacks to clean up
	// the local resources, which bound to the idle connection, when hangup by the peer. No need another goroutine
	// to polling check connection status.
	AddCloseCallback(callback CloseCallback) error
}
