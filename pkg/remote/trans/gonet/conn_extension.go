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
	"io"
	"net"
	"syscall"
	"time"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// NewGonetExtension to build new gonetConnExtension which implements trans.Extension
func NewGonetExtension() trans.Extension {
	return &gonetConnExtension{}
}

type gonetConnExtension struct{}

// SetReadTimeout implements the trans.Extension interface.
func (e *gonetConnExtension) SetReadTimeout(ctx context.Context, conn net.Conn, cfg rpcinfo.RPCConfig, role remote.RPCRole) {
	// only client sets the ReadDeadline here.
	// server sets the ReadDeadline in transServer before invoking OnRead.
	if role == remote.Client {
		if err := setReadTimeout(conn, trans.GetReadTimeout(cfg)); err != nil {
			// The extension cannot return an error. Close before Decode so a custom
			// connection with unsupported timeout methods cannot strand its reader.
			conn.Close()
		}
	}
}

// setReadTimeout uses the duration-based API of netpoll connections when
// available. Other connections retain the standard absolute deadline behavior.
func setReadTimeout(conn net.Conn, timeout time.Duration) error {
	if c, ok := conn.(interface{ SetReadTimeout(time.Duration) error }); ok {
		return c.SetReadTimeout(timeout)
	}
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	return conn.SetReadDeadline(deadline)
}

// NewWriteByteBuffer implements the trans.Extension interface.
func (e *gonetConnExtension) NewWriteByteBuffer(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
	return newBufferWriter(conn.(bufioxReadWriter).Writer())
}

// NewReadByteBuffer implements the trans.Extension interface.
func (e *gonetConnExtension) NewReadByteBuffer(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
	return newBufferReader(conn.(bufioxReadWriter).Reader())
}

// ReleaseBuffer implements the trans.Extension interface.
func (e *gonetConnExtension) ReleaseBuffer(buffer remote.ByteBuffer, err error) error {
	if buffer != nil {
		return buffer.Release(err)
	}
	return nil
}

// IsTimeoutErr implements the trans.Extension interface.
func (e *gonetConnExtension) IsTimeoutErr(err error) bool {
	if err != nil {
		if nerr, ok := err.(net.Error); ok {
			return nerr.Timeout()
		}
	}
	return false
}

// IsRemoteClosedErr implements the trans.Extension interface.
func (e *gonetConnExtension) IsRemoteClosedErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, io.EOF) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, net.ErrClosed)
}
