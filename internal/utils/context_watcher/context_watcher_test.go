/*
 * Copyright 2025 CloudWeGo Authors
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

package context_watcher

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestNewWatcher(t *testing.T) {
	watcher := NewWatcher()
	test.Assert(t, watcher != nil)
	watcher.Close()

	watcher = NewWatcher()
	test.Assert(t, watcher != nil)
}

func TestWatcherRegister(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		calledChan := make(chan context.Context, 1)
		cb := func(ctx context.Context) {
			calledChan <- ctx
		}
		watcher := NewWatcher()
		defer watcher.Close()

		ctx, cancel := context.WithCancel(context.Background())
		err := watcher.Register(ctx, cb)
		test.Assert(t, err == nil, err)
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		timeout := time.After(100 * time.Millisecond)
		select {
		case <-timeout:
			t.Fatal("callback not called within timeout")
		case calledCtx := <-calledChan:
			test.Assert(t, calledCtx == ctx)
		}
	})
	t.Run("register already canceled", func(t *testing.T) {
		calledChan := make(chan context.Context, 1)
		cb := func(ctx context.Context) {
			calledChan <- ctx
		}
		watcher := NewWatcher()
		defer watcher.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := watcher.Register(ctx, cb)
		test.Assert(t, err == nil, err)

		timeout := time.After(100 * time.Millisecond)
		select {
		case <-timeout:
			t.Fatal("callback not called within timeout")
		case calledCtx := <-calledChan:
			test.Assert(t, calledCtx == ctx)
		}
	})
	t.Run("register does not take effect", func(t *testing.T) {
		calledChan := make(chan context.Context, 1)
		cb := func(ctx context.Context) {
			calledChan <- ctx
		}
		watcher := NewWatcher()
		defer watcher.Close()

		// nil ctx
		err := watcher.Register(nil, cb)
		test.Assert(t, err == nil, err)
		timeout := time.After(50 * time.Millisecond)
		select {
		case <-timeout:
		case <-calledChan:
			t.Fatal("callback should not be invoked")
		}

		// ctx without ctx.Done()
		err = watcher.Register(context.Background(), cb)
		test.Assert(t, err == nil, err)
		timeout = time.After(50 * time.Millisecond)
		select {
		case <-timeout:
		case <-calledChan:
			t.Fatal("callback should not be invoked")
		}

		// nil callback
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err = watcher.Register(ctx, nil)
		test.Assert(t, err == nil, err)
	})
}

func TestWatcherDeregister(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		calledChan := make(chan context.Context, 1)
		cb := func(ctx context.Context) {
			calledChan <- ctx
		}
		watcher := NewWatcher()
		defer watcher.Close()

		ctx, cancel := context.WithCancel(context.Background())
		err := watcher.Register(ctx, cb)
		test.Assert(t, err == nil, err)
		watcher.Deregister(ctx)
		cancel()

		timeout := time.After(50 * time.Millisecond)
		select {
		case <-timeout:
		case <-calledChan:
			t.Fatal("callback should not be invoked")
		}
	})
}
