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

package grpc

import (
	"context"
	"testing"

	"github.com/cloudwego/kitex/internal/test"

	"golang.org/x/net/http2"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
)

func newServer(conn *mockNetpollConn) (*http2Server, error) {
	// init
	md := metadata.Pairs("key1", "value1")
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	cfg := &ServerConfig{}

	// newHTTP2Server()
	conn.mockSettingFrame()
	server, err := newHTTP2Server(ctx, conn, cfg)

	return server.(*http2Server), err
}

func newServerStream(conn *mockNetpollConn, server *http2Server) (*Stream, bool) {
	conn.mockMetaHeaderFrame()
	fr, _ := server.framer.ReadFrame()
	frame := fr.(*http2.MetaHeadersFrame)
	server.operateHeaders(frame, func(stream *Stream) {},
		func(ctx context.Context, s string) context.Context {
			return context.Background()
		})
	return server.getStream(frame)
}

func TestHTTP2ServerHandleFrame(t *testing.T) {
	// init
	conn := newMockNpConn(mockAddr0)
	server, err := newServer(conn)
	test.Assert(t, err == nil, err)
	defer server.Close()

	// mock different kinds of frames
	conn.mockMetaHeaderFrame()
	conn.mockPutDataFrame()
	conn.mockSettingFrame()
	conn.mockPingFrame()
	conn.mockWindowUpdateFrame()
	conn.mockStreamErrFrame1()
	conn.mockStreamErrFrame2()
	conn.mockRSTFrame()
	// send err frame to force stop the loop and close server
	conn.mockErrFrame()

	// test HandleStreams()
	server.HandleStreams(func(stream *Stream) {
	}, func(ctx context.Context, s string) context.Context {
		return context.Background()
	})
}

func TestHTTP2ServerWrite(t *testing.T) {
	// init
	conn := newMockNpConn(mockAddr0)
	server, err := newServer(conn)
	test.Assert(t, err == nil, err)
	defer server.Close()

	// mock an active stream
	stream, ok := newServerStream(conn, server)
	test.Assert(t, ok)

	hdr := []byte{0, 0, 0, 0, 1}
	body := []byte{0}
	err = server.Write(stream, hdr, body, &Options{})
	test.Assert(t, err == nil, err)

	// finish Stream
	server.finishStream(stream, false, http2.ErrCodeNo, &headerFrame{}, false)

	// test StreamDone situation Write
	err = server.Write(stream, hdr, body, &Options{})
	test.Assert(t, err != nil)
}

func TestHTTP2ServerWriteStatus(t *testing.T) {
	// init
	conn := newMockNpConn(mockAddr0)
	server, err := newServer(conn)
	test.Assert(t, err == nil, err)
	defer server.Close()

	// mock an active stream
	stream, ok := newServerStream(conn, server)
	test.Assert(t, ok)

	m := map[string]string{}
	md := metadata.New(m)
	err = stream.SetHeader(md)
	test.Assert(t, err == nil, err)

	err = server.WriteStatus(stream, statusGoAway)
	test.Assert(t, err == nil, err)
}
