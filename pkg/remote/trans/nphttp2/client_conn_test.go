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

package nphttp2

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func TestClientConn(t *testing.T) {
	// init
	connPool := newMockConnPool()
	ctx := newMockCtxWithRPCInfo()
	conn, err := connPool.Get(ctx, "tcp", mockAddr0, newMockConnOption())
	test.Assert(t, err == nil, err)
	defer conn.Close()
	clientConn := conn.(*clientConn)

	// test LocalAddr()
	la := clientConn.LocalAddr()
	test.Assert(t, la == nil)

	// test RemoteAddr()
	ra := clientConn.RemoteAddr()
	test.Assert(t, ra.String() == mockAddr0)

	// test Read()
	testStr := "1234567890"
	mockStreamRecv(clientConn.s, testStr)
	testByte := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	n, err := clientConn.Read(testByte)
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(testStr))
	test.Assert(t, string(testByte) == testStr)

	// test write()
	testByte = []byte(testStr)
	n, err = clientConn.Write(testByte)
	test.Assert(t, err == nil, err)
	test.Assert(t, n == len(testByte))

	// test write() short write error
	n, err = clientConn.Write([]byte(""))
	test.Assert(t, err != nil, err)
	test.Assert(t, n == 0)
}

func TestClientConnStreamDesc(t *testing.T) {
	connPool := newMockConnPool()

	// streaming
	ctx := newMockCtxWithRPCInfo()
	ri := rpcinfo.GetRPCInfo(ctx)
	test.Assert(t, ri != nil)
	rpcinfo.AsMutableRPCConfig(ri.Config()).SetInteractionMode(rpcinfo.Streaming)
	conn, err := connPool.Get(ctx, "tcp", mockAddr0, newMockConnOption())
	test.Assert(t, err == nil, err)
	defer conn.Close()
	cn := conn.(*clientConn)
	test.Assert(t, cn != nil)
	test.Assert(t, cn.desc.isStreaming == true)

	// pingpong
	rpcinfo.AsMutableRPCConfig(ri.Config()).SetInteractionMode(rpcinfo.PingPong)
	conn, err = connPool.Get(ctx, "tcp", mockAddr0, newMockConnOption())
	test.Assert(t, err == nil, err)
	defer conn.Close()
	cn = conn.(*clientConn)
	test.Assert(t, cn != nil)
	test.Assert(t, cn.desc.isStreaming == false)
}

func Test_fullMethodName(t *testing.T) {
	t.Run("empty pkg", func(t *testing.T) {
		got := fullMethodName("", "svc", "method")
		test.Assert(t, got == "/svc/method")
	})
	t.Run("with pkg", func(t *testing.T) {
		got := fullMethodName("pkg", "svc", "method")
		test.Assert(t, got == "/pkg.svc/method")
	})
}

func Test_streamingCancel(t *testing.T) {
	t.Run("unary method", func(t *testing.T) {
		// unary method
		pool := newMockConnPool()
		ctx, cancel := context.WithCancel(context.Background())
		ctx = rpcinfo.NewCtxWithRPCInfo(ctx, newMockRPCInfo(false))
		conn, err := pool.Get(ctx, "tcp", mockAddr0, newMockConnOption())
		test.Assert(t, err == nil, err)
		cliConn, ok := conn.(*clientConn)
		test.Assert(t, ok)
		cancel()
		st := cliConn.s
		test.Assert(t, errors.Is(st.Context().Err(), context.Canceled))
		// Wait a while in case the monitor goroutine is actually created.
		time.Sleep(10 * time.Millisecond)
		select {
		case <-st.Done():
			t.Fatal("stream.Done() should not be closed")
		default:
		}
	})

	t.Run("non-unary method", func(t *testing.T) {
		// non-unary method
		pool := newMockConnPool()
		ctx, cancel := context.WithCancel(context.Background())
		ctx = rpcinfo.NewCtxWithRPCInfo(ctx, newMockRPCInfo(true))
		conn, err := pool.Get(ctx, "tcp", mockAddr1, newMockConnOption())
		test.Assert(t, err == nil, err)
		cliConn, ok := conn.(*clientConn)
		test.Assert(t, ok)
		cancel()
		st := cliConn.s
		test.Assert(t, errors.Is(st.Context().Err(), context.Canceled))
		timer := time.NewTimer(10 * time.Second)
		select {
		case <-st.Done():
		case <-timer.C:
			t.Fatal("stream.Done() should be closed quickly")
		}
	})
}
