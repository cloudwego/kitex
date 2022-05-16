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
	"time"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"

	"github.com/cloudwego/kitex/internal/test"
)

func newClient(conn *mockNetpollConn, opt ConnectOptions) (*http2Client, error) {
	md := metadata.Pairs("key1", "value1")
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	remoteService := "unit_test"
	onGoAway := func(reason GoAwayReason) {}
	onClose := func() {}
	conn.mockSettingFrame()
	client, err := newHTTP2Client(ctx, conn, opt, remoteService, onGoAway, onClose)
	return client, err
}

func TestHttp2ClientRead(t *testing.T) {
	opt := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			// keepalive time must be 0, or the mock stream will be killed
			Time:                0,
			Timeout:             0,
			PermitWithoutStream: true,
		},
		InitialWindowSize:     0,
		InitialConnWindowSize: 0,
		MaxHeaderListSize:     nil,
	}

	conn := newMockNpConn(mockAddr1)

	// test newHTTP2Client()
	client, err := newClient(conn, opt)
	test.Assert(t, err == nil, err)
	defer client.Close()

	// test newServerStream()
	callHdr := &CallHdr{
		Host:             "",
		Method:           "",
		SendCompress:     "grpc-encoding",
		ContentSubtype:   "",
		PreviousAttempts: 0,
	}
	_, err = client.NewStream(client.ctx, callHdr)
	test.Assert(t, err == nil, err)

	// wait 100mills so loopyWriter so stream can be active
	time.Sleep(time.Millisecond * 100)

	// mock different kinds of frames
	conn.mockSettingFrame()
	conn.mockPingFrame()
	conn.mockWindowUpdateFrame()
	conn.mockMetaHeaderFrame()
	conn.mockPutDataFrame()
	conn.mockStreamErrFrame1()
	conn.mockRSTFrame()
	conn.mockGoAwayFrame()

	// wait 100 mills so reader() can handle all those frames
	time.Sleep(time.Millisecond * 100)
}

func TestHttp2ClientWrite(t *testing.T) {
	opt := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			// keepalive time must be 0, or the mock stream will be killed
			Time:                0,
			Timeout:             0,
			PermitWithoutStream: true,
		},
		InitialWindowSize:     0,
		InitialConnWindowSize: 0,
		MaxHeaderListSize:     nil,
	}

	conn := newMockNpConn(mockAddr1)
	// test newHTTP2Client()
	client, err := newClient(conn, opt)
	test.Assert(t, err == nil, err)
	defer client.Close()

	callHdr := &CallHdr{
		Host:             "",
		Method:           "",
		SendCompress:     "grpc-encoding",
		ContentSubtype:   "",
		PreviousAttempts: 0,
	}
	stream, err := client.NewStream(client.ctx, callHdr)
	test.Assert(t, err == nil, err)

	hdr := []byte{0, 0, 0, 0, 1}
	body := []byte{0}
	err = client.Write(stream, hdr, body, &Options{})
	test.Assert(t, err == nil, err)
}

func TestMinTime(t *testing.T) {
	min := minTime(time.Millisecond, time.Second)
	test.Assert(t, min == time.Millisecond)
}
