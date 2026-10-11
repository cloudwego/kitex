/*
 * Copyright 2026 CloudWeGo Authors
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

package rpcinfotest

import (
	"context"
	"runtime"
	"testing"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

// MustReadNotNil touches RPCInfo fields commonly reset by framework recycle
// paths. Normal test runs catch nil/reset panics; -race catches framework
// writes racing with an asynchronous user reader.
func MustReadNotNil(ctx context.Context) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil {
		panic("nil RPCInfo")
	}
	if from := ri.From(); from == nil {
		panic("nil From endpoint")
	} else {
		_ = from.ServiceName()
		_ = from.Method()
		_ = from.Address()
	}
	if to := ri.To(); to == nil {
		panic("nil To endpoint")
	} else {
		_ = to.ServiceName()
		_ = to.Method()
		_ = to.Address()
	}
	if inv := ri.Invocation(); inv == nil {
		panic("nil Invocation")
	} else {
		_ = inv.ServiceName()
		_ = inv.MethodName()
		_ = inv.StreamingMode()
	}
	if cfg := ri.Config(); cfg != nil {
		_ = cfg.RPCTimeout()
	}
	if stats := ri.Stats(); stats == nil {
		panic("nil RPCStats")
	} else {
		_ = stats.Level()
		_ = stats.Error()
	}
}

// MustReadAsync reads RPCInfo from a separate goroutine and fails the test if
// the read panics.
func MustReadAsync(t testing.TB, ctx context.Context) {
	t.Helper()

	done := make(chan any, 1)
	go func() {
		defer func() {
			done <- recover()
		}()
		MustReadNotNil(ctx)
	}()
	if panicInfo := <-done; panicInfo != nil {
		t.Fatalf("async RPCInfo read panicked: %v", panicInfo)
	}
}

// ReadLoop continuously reads RPCInfo from a separate goroutine until stopped.
type ReadLoop struct {
	stop chan struct{}
	done chan any
}

// StartReadLoop starts reading RPCInfo asynchronously and waits for the reader
// goroutine to start before returning.
func StartReadLoop(ctx context.Context) *ReadLoop {
	r := &ReadLoop{
		stop: make(chan struct{}),
		done: make(chan any, 1),
	}
	started := make(chan struct{})
	go func() {
		close(started)
		defer func() {
			r.done <- recover()
		}()
		for {
			select {
			case <-r.stop:
				return
			default:
				MustReadNotNil(ctx)
				runtime.Gosched()
			}
		}
	}()
	<-started
	return r
}

// StopAndAssert stops the reader and fails the test if an asynchronous read
// panicked.
func (r *ReadLoop) StopAndAssert(t testing.TB) {
	t.Helper()

	close(r.stop)
	if panicInfo := <-r.done; panicInfo != nil {
		t.Fatalf("async RPCInfo read panicked: %v", panicInfo)
	}
}
