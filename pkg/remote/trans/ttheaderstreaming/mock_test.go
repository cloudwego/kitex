/*
 * Copyright 2024 CloudWeGo Authors
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

package ttheaderstreaming

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/ttheader"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type mockFrameMetaHandler struct {
	remote.FrameMetaHandler
	remote.StreamingMetaHandler
	readMeta         func(ctx context.Context, msg remote.Message) error
	clientReadHeader func(ctx context.Context, msg remote.Message) error
	serverReadHeader func(ctx context.Context, msg remote.Message) (context.Context, error)
	writeHeader      func(ctx context.Context, message remote.Message) error
	readTrailer      func(ctx context.Context, msg remote.Message) error
	writeTrailer     func(ctx context.Context, message remote.Message) error
	onConnectStream  func(ctx context.Context) (context.Context, error)
}

func (m *mockFrameMetaHandler) ReadMeta(ctx context.Context, msg remote.Message) error {
	if m.readMeta != nil {
		return m.readMeta(ctx, msg)
	}
	return nil
}

func (m *mockFrameMetaHandler) ClientReadHeader(ctx context.Context, msg remote.Message) error {
	if m.clientReadHeader != nil {
		return m.clientReadHeader(ctx, msg)
	}
	return nil
}

func (m *mockFrameMetaHandler) ServerReadHeader(ctx context.Context, msg remote.Message) (context.Context, error) {
	if m.serverReadHeader != nil {
		return m.serverReadHeader(ctx, msg)
	}
	return ctx, nil
}

func (m *mockFrameMetaHandler) WriteHeader(ctx context.Context, message remote.Message) error {
	if m.writeHeader != nil {
		return m.writeHeader(ctx, message)
	}
	return nil
}

func (m *mockFrameMetaHandler) ReadTrailer(ctx context.Context, message remote.Message) error {
	if m.readTrailer != nil {
		return m.readTrailer(ctx, message)
	}
	return nil
}

func (m *mockFrameMetaHandler) WriteTrailer(ctx context.Context, message remote.Message) error {
	if m.writeTrailer != nil {
		return m.writeTrailer(ctx, message)
	}
	return nil
}

func (m *mockFrameMetaHandler) OnConnectStream(ctx context.Context) (context.Context, error) {
	return m.onConnectStream(ctx)
}

var _ netpollReader = (*mockConn)(nil)

type mockConn struct {
	readBuf    []byte
	readPos    int
	read       func(b []byte) (int, error)
	write      func(b []byte) (int, error)
	remoteAddr net.Addr
	localAddr  net.Addr
	closeErr   error
}

func (m *mockConn) Reader() netpoll.Reader {
	return m
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if m.read != nil {
		return m.read(b)
	}
	available := len(m.readBuf) - m.readPos
	if available <= 0 {
		return 0, io.EOF
	}
	savedPos := m.readPos
	m.readPos += len(b)
	if m.readPos-savedPos >= available {
		m.readPos = len(m.readBuf)
	}
	return copy(b, m.readBuf[savedPos:m.readPos]), nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	if m.write != nil {
		return m.write(b)
	}
	return len(b), nil // fake success
}

func (m *mockConn) Close() error {
	return m.closeErr
}

func (m *mockConn) LocalAddr() net.Addr {
	return m.localAddr
}

func (m *mockConn) RemoteAddr() net.Addr {
	return m.remoteAddr
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// Next implements netpoll.Reader
func (m *mockConn) Next(n int) (p []byte, err error) {
	// TODO implement me
	panic("implement me")
}

// Peek implements netpoll.Reader
func (m *mockConn) Peek(n int) (buf []byte, err error) {
	available := len(m.readBuf) - m.readPos
	if available < n {
		return nil, io.EOF
	}
	return m.readBuf[m.readPos : m.readPos+n], nil
}

// Skip implements netpoll.Reader
func (m *mockConn) Skip(n int) (err error) {
	// TODO implement me
	panic("implement me")
}

// Until implements netpoll.Reader
func (m *mockConn) Until(delim byte) (line []byte, err error) {
	// TODO implement me
	panic("implement me")
}

// ReadString implements netpoll.Reader
func (m *mockConn) ReadString(n int) (s string, err error) {
	// TODO implement me
	panic("implement me")
}

// ReadBinary implements netpoll.Reader
func (m *mockConn) ReadBinary(n int) (p []byte, err error) {
	// TODO implement me
	panic("implement me")
}

// ReadByte implements netpoll.Reader
func (m *mockConn) ReadByte() (b byte, err error) {
	// TODO implement me
	panic("implement me")
}

// Slice implements netpoll.Reader
func (m *mockConn) Slice(n int) (r netpoll.Reader, err error) {
	// TODO implement me
	panic("implement me")
}

// Release implements netpoll.Reader
func (m *mockConn) Release() (err error) {
	// TODO implement me
	panic("implement me")
}

// Len implements netpoll.Reader
func (m *mockConn) Len() (length int) {
	// TODO implement me
	panic("implement me")
}

type mockByteBuffer struct {
	remote.ByteBuffer
	conn    net.Conn
	bufList [][]byte
}

func (m *mockByteBuffer) Malloc(n int) ([]byte, error) {
	buf := make([]byte, n)
	m.bufList = append(m.bufList, buf)
	return buf, nil
}

func (m *mockByteBuffer) MallocLen() (total int) {
	for _, buf := range m.bufList {
		total += len(buf)
	}
	return
}

func (m *mockByteBuffer) Flush() (err error) {
	fullBuffer := make([]byte, 0, m.MallocLen())
	for _, buf := range m.bufList {
		fullBuffer = append(fullBuffer, buf...)
	}
	_, err = m.conn.Write(fullBuffer)
	return
}

func (m *mockByteBuffer) Next(n int) (p []byte, err error) {
	p = make([]byte, n)
	for idx := 0; idx < n; {
		c, err := m.conn.Read(p[idx:])
		if err != nil {
			return nil, err
		}
		idx += c
	}
	return p, nil
}

func (m *mockByteBuffer) Skip(n int) (err error) {
	_, err = m.Next(n)
	return
}

func (m *mockByteBuffer) WriteString(s string) (n int, err error) {
	m.bufList = append(m.bufList, []byte(s))
	return len(s), nil
}

func (m *mockByteBuffer) WriteBinary(p []byte) (n int, err error) {
	m.bufList = append(m.bufList, p)
	return len(p), nil
}

type mockExtension struct{}

func (m *mockExtension) SetReadTimeout(ctx context.Context, conn net.Conn, cfg rpcinfo.RPCConfig, role remote.RPCRole) {
}

func (m *mockExtension) NewWriteByteBuffer(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
	return &mockByteBuffer{conn: conn}
}

func (m *mockExtension) NewReadByteBuffer(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
	return &mockByteBuffer{conn: conn}
}

func (m *mockExtension) ReleaseBuffer(buffer remote.ByteBuffer, err error) error {
	// TODO implement me
	panic("implement me")
}

func (m *mockExtension) IsTimeoutErr(err error) bool {
	// TODO implement me
	panic("implement me")
}

func (m *mockExtension) IsRemoteClosedErr(err error) bool {
	// TODO implement me
	panic("implement me")
}

type mockStream struct {
	streaming.Stream
	doCloseSend func(error) error
	doRecvMsg   func(msg interface{}) error
	doTrailer   func() metadata.MD
}

func (m *mockStream) closeSend(err error) error {
	return m.doCloseSend(err)
}

func (m *mockStream) RecvMsg(msg interface{}) error {
	return m.doRecvMsg(msg)
}

func (m *mockStream) Trailer() metadata.MD {
	if m.doTrailer != nil {
		return m.doTrailer()
	}
	return nil
}

func decodeMessage(ctx context.Context, buf []byte, data interface{}, role remote.RPCRole) (remote.Message, error) {
	return decodeMessageFromBuffer(ctx, remote.NewReaderBuffer(buf), data, role)
}

func decodeMessageFromBuffer(ctx context.Context, in remote.ByteBuffer, data interface{}, role remote.RPCRole) (remote.Message, error) {
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.NewEndpointInfo("", "", nil, nil),
		rpcinfo.NewMutableEndpointInfo("", "", nil, nil).ImmutableView(),
		rpcinfo.NewInvocation("", ""),
		rpcinfo.NewRPCConfig(),
		rpcinfo.NewRPCStats(),
	)
	msg := remote.NewMessage(
		data,
		nil,
		ri,
		remote.Stream,
		role,
	)
	err := ttheader.NewStreamCodec().Decode(ctx, msg, in)
	return msg, err
}

func encodeMessage(ctx context.Context, msg remote.Message) ([]byte, error) {
	out := remote.NewWriterBuffer(256)
	if err := ttheader.NewStreamCodec().Encode(ctx, msg, out); err != nil {
		return nil, err
	}
	return out.Bytes()
}
