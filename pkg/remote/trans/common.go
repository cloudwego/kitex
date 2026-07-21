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

package trans

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

var (
	readMoreTimeout = 5 * time.Millisecond

	ErrRemoteClosed = errors.New("remote connection closed")
)

// RemoteClosedSource identifies how a remote-closed error was classified.
type RemoteClosedSource int

const (
	// RemoteClosedByExtension indicates the transport extension recognized the error.
	RemoteClosedByExtension RemoteClosedSource = iota
	// RemoteClosedByConnectionState indicates the connection was already inactive.
	RemoteClosedByConnectionState
)

// String implements fmt.Stringer for readable logging.
func (s RemoteClosedSource) String() string {
	switch s {
	case RemoteClosedByExtension:
		return "extension"
	case RemoteClosedByConnectionState:
		return "connection_state"
	default:
		return "unknown"
	}
}

// RemoteClosedError records the original error and how it was classified.
type RemoteClosedError struct {
	Source RemoteClosedSource
	Cause  error
}

// Error implements the error interface.
func (e *RemoteClosedError) Error() string {
	return fmt.Sprintf("%s (%s): %v", ErrRemoteClosed, e.Source, e.Cause)
}

// Unwrap exposes the original error.
func (e *RemoteClosedError) Unwrap() error {
	return e.Cause
}

// Is enables errors.Is(err, ErrRemoteClosed).
func (e *RemoteClosedError) Is(target error) bool {
	return target == ErrRemoteClosed
}

// Extension is the interface that trans extensions need to implement, it will make the extension of trans more easily.
// Normally if we want to extend transport layer we need to implement the trans interfaces which are defined in trans_handler.go.
// In fact most code logic is similar in same mode, so the Extension interface is the the differentiated part that need to
// be implemented separately.
// The default common trans implement is in default_client_handler.go and default_server_handler.go.
type Extension interface {
	SetReadTimeout(ctx context.Context, conn net.Conn, cfg rpcinfo.RPCConfig, role remote.RPCRole)
	NewWriteByteBuffer(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer
	NewReadByteBuffer(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer
	ReleaseBuffer(remote.ByteBuffer, error) error
	IsTimeoutErr(error) bool
	// IsRemoteClosedErr is to check if the error caused by connection closed when output log or report metric
	IsRemoteClosedErr(error) bool
}

// GetReadTimeout is to make the read timeout longer, it is better for proxy case to receive error resp.
func GetReadTimeout(cfg rpcinfo.RPCConfig) time.Duration {
	if cfg.RPCTimeout() <= 0 {
		return 0
	}
	return cfg.RPCTimeout() + readMoreTimeout
}

// IsRemoteClosedErr returns the remote-closed classification for err.
//
// Besides the extension's error-based check, it also treats the error as remote-closed
// when conn is already inactive. This covers the case where an encoder flattens the
// underlying netpoll.ErrConnClosed into a plain string (e.g. gopkg/ttheader uses %s),
// which breaks the errors.Is chain so ext.IsRemoteClosedErr can no longer recognize it.
func IsRemoteClosedErr(ext Extension, err error, conn net.Conn) *RemoteClosedError {
	if err == nil {
		return nil
	}
	if ext.IsRemoteClosedErr(err) {
		return &RemoteClosedError{Source: RemoteClosedByExtension, Cause: err}
	}
	if ac, ok := conn.(remote.IsActive); ok {
		if !ac.IsActive() {
			return &RemoteClosedError{Source: RemoteClosedByConnectionState, Cause: err}
		}
	}
	return nil
}

// MuxEnabledFlag is used to determine whether a serverHandlerFactory is multiplexing.
type MuxEnabledFlag interface {
	MuxEnabled() bool
}
