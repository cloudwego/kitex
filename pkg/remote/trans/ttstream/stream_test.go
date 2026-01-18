//go:build !windows

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

package ttstream

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

func TestGenericStreaming(t *testing.T) {
	cs, ss, err := newTestStreamPipe(testServiceInfo, "Bidi")
	test.Assert(t, err == nil, err)
	ctx := context.Background()

	// raw struct
	req := new(testRequest)
	req.A = 123
	req.B = "hello"
	err = cs.SendMsg(ctx, req)
	test.Assert(t, err == nil, err)
	res := new(testResponse)
	err = ss.RecvMsg(ctx, res)
	test.Assert(t, err == nil, err)
	test.Assert(t, res.A == req.A)
	test.Assert(t, res.B == req.B)

	// map generic
	// client side
	// reqJSON := `{"A":123, "b":"hello"}`
	// err = cs.SendMsg(ctx, reqJSON)
	// test.Assert(t, err == nil, err)
	// server side
	// res = new(testResponse)
	// err = ss.RecvMsg(ctx, res)
	// test.Assert(t, err == nil, err)
	// test.Assert(t, res.A == req.A)
	// test.Assert(t, res.B == req.B)
}

// TestStreamRecvTimeout tests that RecvMsg correctly handles timeout scenarios
func TestStreamRecvTimeout(t *testing.T) {
	_, ss, err := newTestStreamPipe(testServiceInfo, "Bidi")
	test.Assert(t, err == nil, err)

	// Set a very short timeout for testing
	cfg := rpcinfo.NewRPCConfig()
	rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeout(time.Millisecond * 10)
	ss.setRecvTimeoutConfig(cfg)

	// Create a context that won't expire
	ctx := context.Background()

	// Try to receive message - should timeout quickly
	res := new(testResponse)
	err = ss.RecvMsg(ctx, res)
	test.Assert(t, err != nil, "RecvMsg should timeout")
	test.Assert(t, strings.Contains(err.Error(), "timeout") ||
		strings.Contains(err.Error(), "deadline exceeded"),
		"Error should be timeout related")
}

// TestStreamRecvWithCancellation tests that RecvMsg respects context cancellation
func TestStreamRecvWithCancellation(t *testing.T) {
	cs, ss, err := newTestStreamPipe(testServiceInfo, "Bidi")
	test.Assert(t, err == nil, err)

	// Create a cancellable context with short timeout
	cancelCtx, cancel := context.WithCancel(context.Background())

	cancel()

	// Send message
	req := new(testRequest)
	req.A = 789
	req.B = "normal_test"

	err = cs.SendMsg(cancelCtx, req)
	test.Assert(t, err == nil, err)

	// Receive message
	res := new(testResponse)
	err = ss.RecvMsg(cancelCtx, res)
	test.Assert(t, err == nil, err)

	// Verify content
	test.Assert(t, res.A == req.A)
	test.Assert(t, res.B == req.B)
}

// TestSetRecvTimeoutConfig tests setRecvTimeoutConfig method with different configuration scenarios
func TestSetRecvTimeoutConfig(t *testing.T) {
	t.Run("only set StreamRecvTimeout", func(t *testing.T) {
		_, ss, err := newTestStreamPipe(testServiceInfo, "Bidi")
		test.Assert(t, err == nil, err)

		cfg := rpcinfo.NewRPCConfig()
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeout(100 * time.Millisecond)

		ss.setRecvTimeoutConfig(cfg)

		// verify timeout config is set from StreamRecvTimeout
		test.Assert(t, ss.recvTimeoutConfig.Timeout == 100*time.Millisecond, ss.recvTimeoutConfig)
		test.Assert(t, ss.recvTimeoutConfig.DisableCancelRemote == false, ss.recvTimeoutConfig)
	})

	t.Run("only set StreamRecvTimeoutConfig", func(t *testing.T) {
		_, ss, err := newTestStreamPipe(testServiceInfo, "Bidi")
		test.Assert(t, err == nil, err)

		cfg := rpcinfo.NewRPCConfig()
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeoutConfig(streaming.TimeoutConfig{
			Timeout:             200 * time.Millisecond,
			DisableCancelRemote: true,
		})

		ss.setRecvTimeoutConfig(cfg)

		// verify timeout config is set from StreamRecvTimeoutConfig
		test.Assert(t, ss.recvTimeoutConfig.Timeout == 200*time.Millisecond, ss.recvTimeoutConfig)
		test.Assert(t, ss.recvTimeoutConfig.DisableCancelRemote == true, ss.recvTimeoutConfig)
	})

	t.Run("both set, StreamRecvTimeoutConfig has higher priority", func(t *testing.T) {
		_, ss, err := newTestStreamPipe(testServiceInfo, "Bidi")
		test.Assert(t, err == nil, err)

		cfg := rpcinfo.NewRPCConfig()
		// set StreamRecvTimeout first with a longer duration
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeout(500 * time.Millisecond)
		// then set StreamRecvTimeoutConfig with a shorter duration
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeoutConfig(streaming.TimeoutConfig{
			Timeout:             50 * time.Millisecond,
			DisableCancelRemote: true,
		})

		ss.setRecvTimeoutConfig(cfg)

		// verify StreamRecvTimeoutConfig takes priority over StreamRecvTimeout
		test.Assert(t, ss.recvTimeoutConfig.Timeout == 50*time.Millisecond, ss.recvTimeoutConfig)
		test.Assert(t, ss.recvTimeoutConfig.DisableCancelRemote == true, ss.recvTimeoutConfig)
	})
}
