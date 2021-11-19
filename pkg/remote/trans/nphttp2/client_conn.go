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

package nphttp2

import (
	"context"
	"io"
	"net"
	"strings"
	"time"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

type clientConn struct {
	tr grpc.ClientTransport
	s  *grpc.Stream
}

var _ net.Conn = (*clientConn)(nil)

func newClientConn(ctx context.Context, tr grpc.ClientTransport, addr string) (*clientConn, error) {
	var str strings.Builder
	ri := rpcinfo.GetRPCInfo(ctx)
	str.WriteString("/")
	str.WriteString(ri.Invocation().PackageName())
	str.WriteString(".")
	str.WriteString(ri.Invocation().ServiceName())
	str.WriteString("/")
	str.WriteString(ri.Invocation().MethodName())
	s, err := tr.NewStream(ctx, &grpc.CallHdr{
		Host: ri.To().ServiceName(),
		// grpc method format /package.Service/Method
		Method: str.String(),
	})
	if err != nil {
		return nil, err
	}
	return &clientConn{
		tr: tr,
		s:  s,
	}, nil
}

// impl net.Conn
func (c *clientConn) Read(b []byte) (n int, err error) {
	n, err = c.s.Read(b)
	if err == io.EOF {
		if statusErr := c.s.Status().Err(); statusErr != nil {
			err = statusErr
		}
	}
	return n, convertErrorFromGrpcToKitex(err)
}

func (c *clientConn) Write(b []byte) (n int, err error) {
	if len(b) < 5 {
		return 0, io.ErrShortWrite
	}
	err = c.tr.Write(c.s, b[:5], b[5:], &grpc.Options{})
	return len(b), convertErrorFromGrpcToKitex(err)
}

func (c *clientConn) LocalAddr() net.Addr                { return c.tr.LocalAddr() }
func (c *clientConn) RemoteAddr() net.Addr               { return c.tr.RemoteAddr() }
func (c *clientConn) SetDeadline(t time.Time) error      { return nil }
func (c *clientConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *clientConn) SetWriteDeadline(t time.Time) error { return nil }
func (c *clientConn) Close() error {
	c.tr.Write(c.s, nil, nil, &grpc.Options{Last: true})
	// Always return nil; io.EOF is the only error that might make sense
	// instead, but there is no need to signal the client to call Read
	// as the only use left for the stream after Close is to call
	// Read. This also matches historical behavior.
	return nil
}

func (c *clientConn) Header() (metadata.MD, error) { return c.s.Header() }
func (c *clientConn) Trailer() metadata.MD         { return c.s.Trailer() }
