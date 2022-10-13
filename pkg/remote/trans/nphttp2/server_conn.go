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
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type serverConn struct {
	tr grpc.ServerTransport
	s  *grpc.Stream
}

var _ GRPCConn = (*serverConn)(nil)

func newServerConn(tr grpc.ServerTransport, s *grpc.Stream) *serverConn {
	return &serverConn{
		tr: tr,
		s:  s,
	}
}

func (c *serverConn) ReadFrame() (hdr, data []byte, err error) {
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

func GetServerConn(st streaming.Stream) (GRPCConn, error) {
	serverStream, ok := st.(*stream)
	if !ok {
		// err!
		return nil, status.Errorf(codes.Internal, "failed to get server conn from stream.")
	}
	grpcServerConn, ok := serverStream.conn.(GRPCConn)
	if !ok {
		// err!
		return nil, status.Errorf(codes.Internal, "failed to trans conn to grpc conn.")
	}
	return grpcServerConn, nil
}

// impl net.Conn
func (c *serverConn) Read(b []byte) (n int, err error) {
	n, err = c.s.Read(b)
	return n, err
}

func (c *serverConn) Write(b []byte) (n int, err error) {
	if len(b) < 5 {
		return 0, io.ErrShortWrite
	}
	return c.WriteFrame(b[:5], b[5:])
}

func (c *serverConn) WriteFrame(hdr, data []byte) (n int, err error) {
	grpcConnOpt := &grpc.Options{}
	// When there's no more data frame, add END_STREAM flag to this empty frame.
	if hdr == nil && data == nil {
		grpcConnOpt.Last = true
	}
	err = c.tr.Write(c.s, hdr, data, grpcConnOpt)
	return len(hdr) + len(data), err
}

func (c *serverConn) LocalAddr() net.Addr                { return c.tr.LocalAddr() }
func (c *serverConn) RemoteAddr() net.Addr               { return c.tr.RemoteAddr() }
func (c *serverConn) SetDeadline(t time.Time) error      { return nil }
func (c *serverConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *serverConn) SetWriteDeadline(t time.Time) error { return nil }
func (c *serverConn) Close() error {
	return c.tr.WriteStatus(c.s, status.New(codes.OK, ""))
}
