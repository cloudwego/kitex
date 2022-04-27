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

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
)

// todo: cover 80%

func NewClient(conn *mockNetpollConn, opt ConnectOptions) (*http2Client, error) {
	md := metadata.Pairs("key1", "value1")
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	remoteService := "test"
	onGoAway := func(reason GoAwayReason) {}
	onClose := func() {}
	conn.mockSettingFrame()
	return newHTTP2Client(ctx, conn, opt, remoteService, onGoAway, onClose)
}

func TestHttp2ClientRead(t *testing.T) {
	opt := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			// must be 0, or your test stream will be killed
			Time:                0,
			Timeout:             0,
			PermitWithoutStream: true,
		},
		InitialWindowSize:     0,
		InitialConnWindowSize: 0,
		MaxHeaderListSize:     nil,
	}

	conn := newMockNpConn(mockAddr0)

	// test newHTTP2Client()
	client, err := NewClient(conn, opt)
	test.Assert(t, err == nil, err)

	// test NewStream()
	callHdr := &CallHdr{
		Host:             "",
		Method:           "",
		SendCompress:     "grpc-encoding",
		ContentSubtype:   "",
		PreviousAttempts: 0,
	}
	stream, err := client.NewStream(client.ctx, callHdr)
	test.Assert(t, err == nil, err)

	client.activeStreams[stream.id] = stream

	conn.mockSettingFrame()
	conn.mockPingFrame()
	conn.mockWindowUpdateFrame()
	conn.mockMetaHeaderFrame()
	conn.mockPutDataFrame()
	// mock different kinds of frame
	conn.mockStreamErrFrame1()
	conn.mockRSTFrame()
	// still err, force stop reader()
	conn.mockGoAwayFrame()

	// todo handleGoAway

	// wait 500mills so reader() can handle those frames
	time.Sleep(time.Millisecond * 500)

	err = client.Close()
	test.Assert(t, err == nil, err)
}

func TestHttp2ClientWrite(t *testing.T) {
	opt := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			// must be 0, or your test stream will be killed
			Time:                0,
			Timeout:             0,
			PermitWithoutStream: true,
		},
		InitialWindowSize:     0,
		InitialConnWindowSize: 0,
		MaxHeaderListSize:     nil,
	}

	conn := newMockNpConn(mockAddr0)
	// test newHTTP2Client()
	client, err := NewClient(conn, opt)
	test.Assert(t, err == nil, err)

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
	client.Write(stream, hdr, body, &Options{})
}

func TestHttp2ClientKeepalive(t *testing.T) {
	conn := newMockNpConn(mockAddr0)

	opt := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			// must be 0, or your test stream will be killed
			Time:                1,
			Timeout:             1,
			PermitWithoutStream: true,
		},
		InitialWindowSize:     0,
		InitialConnWindowSize: 0,
		MaxHeaderListSize:     nil,
	}

	// test newHTTP2Client()
	client, err := NewClient(conn, opt)
	test.Assert(t, err == nil, err)

	// test a stream
	callHdr := &CallHdr{
		Host:             "",
		Method:           "",
		SendCompress:     "grpc-encoding",
		ContentSubtype:   "",
		PreviousAttempts: 0,
	}
	stream, err := client.NewStream(client.ctx, callHdr)
	test.Assert(t, err == nil, err)

	client.CloseStream(stream, context.Canceled)

	client.GracefulClose()
}

func TestHttp2ClientAction(t *testing.T) {
	opt := ConnectOptions{
		KeepaliveParams: ClientKeepalive{
			// must be 0, or your test stream will be killed
			Time:                0,
			Timeout:             0,
			PermitWithoutStream: true,
		},
		InitialWindowSize:     0,
		InitialConnWindowSize: 0,
		MaxHeaderListSize:     nil,
	}

	conn := newMockNpConn(mockAddr0)
	// test newHTTP2Client()
	client, err := NewClient(conn, opt)
	test.Assert(t, err == nil, err)

	client.Error()
	client.GoAway()
	client.RemoteAddr()
	client.LocalAddr()

	callHdr := &CallHdr{
		Host:             "",
		Method:           "",
		SendCompress:     "grpc-encoding",
		ContentSubtype:   "",
		PreviousAttempts: 0,
	}
	stream, err := client.NewStream(client.ctx, callHdr)
	test.Assert(t, err == nil, err)

	stream.fc.pendingData = 600

	// test updateWindow()
	client.updateWindow(stream, 65535)

	stream.fc.pendingUpdate = 1

	// test adjustWindow()
	client.adjustWindow(stream, 65535)

	// test updateFlowControl
	client.updateFlowControl(1)
}

func TestMinTime(t *testing.T) {
	min := minTime(time.Millisecond, time.Second)
	test.Assert(t, min == time.Millisecond)
}
