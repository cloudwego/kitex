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
	ss.setRecvTimeout(time.Millisecond * 10)

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
