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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gopkg/lang/fastrand"

	"github.com/cloudwego/kitex/internal/test"
)

func TestWatcher_Register(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		w := NewWatcher()
		defer w.Close()

		calledChan := make(chan context.Context, 1)
		callback := func(ctx context.Context) {
			calledChan <- ctx
		}

		ctx, cancel := context.WithCancel(context.Background())
		err := w.Register(ctx, callback)
		test.Assert(t, err == nil, err)
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		timeout := time.After(50 * time.Millisecond)
		select {
		case <-timeout:
			t.Fatal("callback not called within timeout")
		case calledCtx := <-calledChan:
			test.Assert(t, calledCtx == ctx)
		}
	})
	t.Run("invalid ctx or callback", func(t *testing.T) {
		w := NewWatcher()
		defer w.Close()

		// nil context
		//nolint:staticcheck // SA1012: unit test
		err := w.Register(nil, func(context.Context) {})
		test.Assert(t, err == errRegisterNilCtx, err)

		// nil callback
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err = w.Register(ctx, nil)
		test.Assert(t, err == errRegisterNilCallback, err)

		// ctx with nil ctx.Done()
		err = w.Register(context.Background(), func(context.Context) {})
		test.Assert(t, err == nil, err)
	})
	t.Run("duplicate ctx", func(t *testing.T) {
		w := NewWatcher()
		defer w.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		calledChan := make(chan context.Context, 1)
		callback := func(ctx context.Context) {
			calledChan <- ctx
		}

		// Register twice with same context
		err := w.Register(ctx, callback)
		test.Assert(t, err == nil, err)
		err = w.Register(ctx, callback)
		test.Assert(t, err == nil, err)

		cancel()
		timeout := time.After(50 * time.Millisecond)
		select {
		case <-timeout:
			t.Fatal("callback not called within timeout")
		case calledCtx := <-calledChan:
			test.Assert(t, calledCtx == ctx)
		}

		timeout = time.After(50 * time.Millisecond)
		select {
		case <-timeout:
		case <-calledChan:
			t.Fatal("callback should only be called once")
		}
	})
	t.Run("register ctx with timeout triggered", func(t *testing.T) {
		w := NewWatcher()
		defer w.Close()

		calledChan := make(chan context.Context, 1)
		callback := func(ctx context.Context) {
			calledChan <- ctx
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		err := w.Register(ctx, callback)
		test.Assert(t, err == nil, err)

		timeout := time.After(50 * time.Millisecond)
		select {
		case <-timeout:
			t.Fatal("callback not called within timeout")
		case calledCtx := <-calledChan:
			test.Assert(t, calledCtx == ctx)
			test.Assert(t, calledCtx.Err() == context.DeadlineExceeded, calledCtx.Err())
		}
	})
	t.Run("register panic callback", func(t *testing.T) {
		w := NewWatcher()
		defer w.Close()

		ctx, cancel := context.WithCancel(context.Background())
		callback := func(ctx context.Context) {
			panic("callback panic")
		}

		err := w.Register(ctx, callback)
		test.Assert(t, err == nil, err)

		cancel()
		defer func() {
			r := recover()
			test.Assert(t, r == nil, r)
		}()

		timeout := time.After(50 * time.Millisecond)
		// wait for callback triggered
		<-timeout
	})
	t.Run("register after close", func(t *testing.T) {
		w := NewWatcher()
		err := w.Close()
		test.Assert(t, err == nil, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err = w.Register(ctx, func(context.Context) {})
		test.Assert(t, err == errWatcherClosed, err)
	})
	t.Run("register concurrently", func(t *testing.T) {
		w := NewWatcher()
		defer w.Close()
		var calledNum int32
		callback := func(ctx context.Context) {
			atomic.AddInt32(&calledNum, 1)
		}
		var wg sync.WaitGroup
		ctxNum := 1000
		wg.Add(ctxNum)
		for i := 0; i < ctxNum; i++ {
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				err := w.Register(ctx, callback)
				test.Assert(t, err == nil, err)
				go func() {
					time.Sleep(time.Duration(fastrand.Intn(50)) * time.Millisecond)
					cancel()
				}()
			}()
		}
		wg.Wait()

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ticker.C:
				if atomic.LoadInt32(&calledNum) == int32(ctxNum) {
					return
				}
			case <-timer.C:
				t.Fatalf("wait for calledNum %d timeout, got %d actually", ctxNum, atomic.LoadInt32(&calledNum))
			}
		}
	})
	t.Run("register concurrently with new shard created", func(t *testing.T) {
		initialShards := 1
		maxCtxPerShard := 1000
		expectShards := initialShards + 1
		ctxNum := expectShards * maxCtxPerShard
		w := newWatcher(initialShards, maxCtxPerShard)
		defer w.Close()
		var calledNum int32
		callback := func(ctx context.Context) {
			atomic.AddInt32(&calledNum, 1)
		}
		var wg sync.WaitGroup

		cancelCh := make(chan struct{})
		wg.Add(ctxNum)
		for i := 0; i < ctxNum; i++ {
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				err := w.Register(ctx, callback)
				test.Assert(t, err == nil, err)
				go func() {
					<-cancelCh
					cancel()
				}()
			}()
		}
		wg.Wait()
		w.shardsMu.RLock()
		test.Assert(t, len(w.shards) == expectShards, len(w.shards))
		w.shardsMu.RUnlock()
		close(cancelCh)

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ticker.C:
				if atomic.LoadInt32(&calledNum) == int32(ctxNum) {
					return
				}
			case <-timer.C:
				t.Fatalf("wait for calledNum %d timeout, got %d actually", ctxNum, atomic.LoadInt32(&calledNum))
			}
		}
	})
}

func TestWatcher_Deregister(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		w := NewWatcher()
		defer w.Close()

		calledChan := make(chan context.Context, 1)
		callback := func(ctx context.Context) {
			calledChan <- ctx
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := w.Register(ctx, callback)
		test.Assert(t, err == nil, err)

		w.Deregister(ctx)

		timeout := time.After(50 * time.Millisecond)
		select {
		case <-timeout:
		case <-calledChan:
			t.Fatal("callback should not be called")
		}
	})
	t.Run("invalid ctx", func(t *testing.T) {
		w := NewWatcher()
		defer w.Close()

		// nil context
		//nolint:staticcheck // SA1012: unit test
		err := w.Register(nil, func(context.Context) {})
		test.Assert(t, err == errRegisterNilCtx, err)

		// nil callback
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err = w.Register(ctx, nil)
		test.Assert(t, err == errRegisterNilCallback, err)

		// ctx with nil ctx.Done()
		err = w.Register(context.Background(), func(context.Context) {})
		test.Assert(t, err == nil, err)
	})
	t.Run("deregister after close", func(t *testing.T) {
		w := NewWatcher()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := w.Register(ctx, func(context.Context) {})
		test.Assert(t, err == nil, err)
		err = w.Close()
		test.Assert(t, err == nil, err)
		w.Deregister(ctx)
	})
	t.Run("deregister concurrently", func(t *testing.T) {
		w := NewWatcher()
		cancelCh := make(chan struct{})
		defer func() {
			close(cancelCh)
			w.Close()
		}()
		var calledNum int32
		callback := func(ctx context.Context) {
			atomic.AddInt32(&calledNum, 1)
		}
		var wg sync.WaitGroup
		ctxNum := 1000
		wg.Add(ctxNum)
		for i := 0; i < ctxNum; i++ {
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				err := w.Register(ctx, callback)
				test.Assert(t, err == nil, err)
				go func() {
					time.Sleep(time.Duration(fastrand.Intn(50)) * time.Millisecond)
					w.Deregister(ctx)
					<-cancelCh
					cancel()
				}()
			}()
		}
		wg.Wait()

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
	RegistryCheckLoop:
		for {
			select {
			case <-ticker.C:
				w.registryMu.Lock()
				if len(w.registry) == 0 {
					w.registryMu.Unlock()
					break RegistryCheckLoop
				}
				w.registryMu.Unlock()
			case <-timer.C:
				t.Fatalf("wait for calledNum %d timeout, got %d actually", ctxNum, atomic.LoadInt32(&calledNum))
			}
		}
		timer = time.NewTimer(5 * time.Second)
	CtxCountCheckLoop:
		for {
			select {
			case <-ticker.C:
				w.shardsMu.RLock()
				for _, s := range w.shards {
					if count := s.getCtxCount(); count != 0 {
						w.shardsMu.RUnlock()
						continue CtxCountCheckLoop
					}
				}
				w.shardsMu.RUnlock()
				break CtxCountCheckLoop
			case <-timer.C:
				t.Fatalf("wait for ctxCount becoming 0 timeout")
			}
		}
	})
	t.Run("deregister concurrently with new shard created", func(t *testing.T) {
		w := newWatcher(1, 1000)
		cancelCh := make(chan struct{})
		defer func() {
			close(cancelCh)
			w.Close()
		}()
		var calledNum int32
		callback := func(ctx context.Context) {
			atomic.AddInt32(&calledNum, 1)
		}
		var wg sync.WaitGroup
		ctxNum := 2000
		wg.Add(ctxNum)
		for i := 0; i < ctxNum; i++ {
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				err := w.Register(ctx, callback)
				test.Assert(t, err == nil, err)
				go func() {
					time.Sleep(time.Duration(fastrand.Intn(50)) * time.Millisecond)
					w.Deregister(ctx)
					<-cancelCh
					cancel()
				}()
			}()
		}
		wg.Wait()

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
	RegistryCheckLoop:
		for {
			select {
			case <-ticker.C:
				w.registryMu.Lock()
				if len(w.registry) == 0 {
					w.registryMu.Unlock()
					break RegistryCheckLoop
				}
				w.registryMu.Unlock()
			case <-timer.C:
				t.Fatalf("wait for calledNum %d timeout, got %d actually", ctxNum, atomic.LoadInt32(&calledNum))
			}
		}
		timer = time.NewTimer(5 * time.Second)
	CtxCountCheckLoop:
		for {
			select {
			case <-ticker.C:
				w.shardsMu.RLock()
				for _, s := range w.shards {
					if count := s.getCtxCount(); count != 0 {
						w.shardsMu.RUnlock()
						continue CtxCountCheckLoop
					}
				}
				w.shardsMu.RUnlock()
				break CtxCountCheckLoop
			case <-timer.C:
				t.Fatalf("wait for ctxCount becoming 0 timeout")
			}
		}
	})
}
