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
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

type clientConn struct {
	tr grpc.ClientTransport
	s  *grpc.Stream
}

var _ GRPCConn = (*clientConn)(nil)

func (c *clientConn) ReadFrame() (hdr, data []byte, err error) {
	hdr = make([]byte, 5)
	_, err = c.Read(hdr)
	if err != nil {
		return nil, nil, err
	}
	dLen := int(binary.BigEndian.Uint32(hdr[1:]))
	data = make([]byte, dLen)
	_, err = c.Read(data)
	if err != nil {
		return nil, nil, err
	}
	return hdr, data, nil
}

func newClientConn(ctx context.Context, tr grpc.ClientTransport, addr string) (*clientConn, error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	var svcName string
	if ri.Invocation().PackageName() == "" {
		svcName = ri.Invocation().ServiceName()
	} else {
		svcName = fmt.Sprintf("%s.%s", ri.Invocation().PackageName(), ri.Invocation().ServiceName())
	}

	host := ri.To().ServiceName()
	if rawURL, ok := ri.To().Tag(rpcinfo.HTTPURL); ok {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, err
		}
		host = u.Host
	}

	s, err := tr.NewStream(ctx, &grpc.CallHdr{
		Host: host,
		// grpc method format /package.Service/Method
		Method: fmt.Sprintf("/%s/%s", svcName, ri.Invocation().MethodName()),
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
		if status := c.s.Status(); status.Code() != codes.OK {
			if bizStatusErr := c.s.BizStatusErr(); bizStatusErr != nil {
				err = bizStatusErr
			} else {
				err = status.Err()
			}
		}
	}
	return n, err
}

func (c *clientConn) Write(b []byte) (n int, err error) {
	if len(b) < 5 {
		return 0, io.ErrShortWrite
	}
	return c.WriteFrame(b[:5], b[5:])
}

func (c *clientConn) WriteFrame(hdr, data []byte) (n int, err error) {
	grpcConnOpt := &grpc.Options{}
	// When there's no more data frame, add END_STREAM flag to this empty frame.
	if hdr == nil && data == nil {
		grpcConnOpt.Last = true
	}
	err = c.tr.Write(c.s, hdr, data, grpcConnOpt)
	return len(hdr) + len(data), err
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
