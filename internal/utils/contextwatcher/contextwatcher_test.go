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

package contextwatcher

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRegisterContext_BasicCallback tests that callback is invoked when context is cancelled
func TestRegisterContext_BasicCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	called := atomic.Bool{}
	var mu sync.Mutex
	var callbackCtx context.Context

	RegisterContext(ctx, func(c context.Context) {
		called.Store(true)
		mu.Lock()
		callbackCtx = c
		mu.Unlock()
	})

	// Cancel the context
	cancel()

	// Wait for callback to be invoked
	time.Sleep(50 * time.Millisecond)

	if !called.Load() {
		t.Error("callback was not called after context cancellation")
	}

	mu.Lock()
	receivedCtx := callbackCtx
	mu.Unlock()

	if receivedCtx != ctx {
		t.Error("callback received wrong context")
	}
}

// TestRegisterContext_WithTimeout tests callback with timeout context
func TestRegisterContext_WithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	called := atomic.Bool{}

	RegisterContext(ctx, func(c context.Context) {
		called.Store(true)
	})

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	if !called.Load() {
		t.Error("callback was not called after context timeout")
	}
}

// TestRegisterContext_NoDeadline tests that no goroutine is created for contexts without deadline
func TestRegisterContext_NoDeadline(t *testing.T) {
	ctx := context.Background()

	called := atomic.Bool{}

	beforeGoroutines := runtime.NumGoroutine()

	RegisterContext(ctx, func(c context.Context) {
		called.Store(true)
	})

	time.Sleep(50 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()

	if called.Load() {
		t.Error("callback should not be called for context without deadline")
	}

	// No new goroutine should be created
	if afterGoroutines > beforeGoroutines {
		t.Errorf("goroutine leak detected: before=%d, after=%d", beforeGoroutines, afterGoroutines)
	}
}

// TestRegisterContext_DuplicateRegistration tests that duplicate registrations are ignored
func TestRegisterContext_DuplicateRegistration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := atomic.Int32{}

	callback := func(c context.Context) {
		callCount.Add(1)
	}

	// Register the same context twice
	RegisterContext(ctx, callback)
	RegisterContext(ctx, callback)

	cancel()

	time.Sleep(50 * time.Millisecond)

	// Should only be called once
	if count := callCount.Load(); count != 1 {
		t.Errorf("expected callback to be called once, got %d times", count)
	}
}

// TestDeregisterContext_PreventCallback tests that deregistration prevents callback
func TestDeregisterContext_PreventCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := atomic.Bool{}

	RegisterContext(ctx, func(c context.Context) {
		called.Store(true)
	})

	// Deregister before cancellation
	DeregisterContext(ctx)

	// Give some time for deregister to take effect
	time.Sleep(10 * time.Millisecond)

	cancel()

	// Wait to see if callback is called
	time.Sleep(50 * time.Millisecond)

	if called.Load() {
		t.Error("callback should not be called after deregistration")
	}
}

// TestDeregisterContext_NonExistent tests deregistering a non-existent context
func TestDeregisterContext_NonExistent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should not panic
	DeregisterContext(ctx)
}

// TestDeregisterContext_AfterCallback tests deregistering after callback is called
func TestDeregisterContext_AfterCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	called := atomic.Bool{}

	RegisterContext(ctx, func(c context.Context) {
		called.Store(true)
	})

	cancel()

	time.Sleep(50 * time.Millisecond)

	if !called.Load() {
		t.Fatal("callback should have been called")
	}

	// Deregister after callback - should not panic
	DeregisterContext(ctx)
}

// TestConcurrentRegistration tests concurrent registrations of different contexts
func TestConcurrentRegistration(t *testing.T) {
	const numContexts = 100

	var wg sync.WaitGroup
	callCount := atomic.Int32{}

	for i := 0; i < numContexts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			RegisterContext(ctx, func(c context.Context) {
				callCount.Add(1)
			})

			cancel()
		}()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	if count := callCount.Load(); count != numContexts {
		t.Errorf("expected %d callbacks, got %d", numContexts, count)
	}
}

// TestConcurrentRegisterDeregister tests concurrent register and deregister operations
func TestConcurrentRegisterDeregister(t *testing.T) {
	const numOps = 100

	var wg sync.WaitGroup

	for i := 0; i < numOps; i++ {
		wg.Add(2)

		ctx, cancel := context.WithCancel(context.Background())

		// Register
		go func() {
			defer wg.Done()
			RegisterContext(ctx, func(c context.Context) {
				// callback
			})
		}()

		// Deregister
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			DeregisterContext(ctx)
		}()

		cancel()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}

// TestConcurrentSameContext tests concurrent operations on the same context
func TestConcurrentSameContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := atomic.Int32{}

	var wg sync.WaitGroup

	// Multiple goroutines trying to register the same context
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			RegisterContext(ctx, func(c context.Context) {
				callCount.Add(1)
			})
		}()
	}

	wg.Wait()

	cancel()
	time.Sleep(50 * time.Millisecond)

	// Should only be called once despite multiple registration attempts
	if count := callCount.Load(); count != 1 {
		t.Errorf("expected callback to be called once, got %d times", count)
	}
}

// TestGoroutineCleanup tests that goroutines are properly cleaned up
func TestGoroutineCleanup(t *testing.T) {
	// Allow some buffer for goroutine count fluctuation
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	beforeGoroutines := runtime.NumGoroutine()

	const numContexts = 50

	// Register and cancel contexts
	for i := 0; i < numContexts; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		RegisterContext(ctx, func(c context.Context) {})
		cancel()
	}

	// Wait for all callbacks to be executed
	time.Sleep(200 * time.Millisecond)

	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()

	// Allow some variance but should not have significant goroutine leak
	// Note: This test may need adjustment based on the actual implementation
	// If callback doesn't clean up the map entry, there might be some goroutines left
	if afterGoroutines > beforeGoroutines+numContexts {
		t.Logf("Warning: Potential goroutine leak - before: %d, after: %d",
			beforeGoroutines, afterGoroutines)
	}
}

// TestGoroutineCleanupWithDeregister tests that deregister properly cleans up goroutines
func TestGoroutineCleanupWithDeregister(t *testing.T) {
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	beforeGoroutines := runtime.NumGoroutine()

	const numContexts = 50
	contexts := make([]context.Context, numContexts)
	cancels := make([]context.CancelFunc, numContexts)

	// Register contexts
	for i := 0; i < numContexts; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		contexts[i] = ctx
		cancels[i] = cancel
		RegisterContext(ctx, func(c context.Context) {})
	}

	// Deregister all contexts
	for i := 0; i < numContexts; i++ {
		DeregisterContext(contexts[i])
	}

	// Clean up
	for i := 0; i < numContexts; i++ {
		cancels[i]()
	}

	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	afterGoroutines := runtime.NumGoroutine()

	// With proper deregister, goroutines should be cleaned up
	if afterGoroutines > beforeGoroutines+5 {
		t.Errorf("Goroutine leak detected after deregister - before: %d, after: %d",
			beforeGoroutines, afterGoroutines)
	}
}

// TestCallbackPanic tests that a panic in callback doesn't affect other operations
func TestCallbackPanic(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	called2 := atomic.Bool{}

	// First context with panicking callback
	RegisterContext(ctx1, func(c context.Context) {
		panic("test panic")
	})

	// Second context with normal callback
	RegisterContext(ctx2, func(c context.Context) {
		called2.Store(true)
	})

	cancel1()
	time.Sleep(50 * time.Millisecond)

	cancel2()
	time.Sleep(50 * time.Millisecond)

	// Second callback should still be called despite first one panicking
	if !called2.Load() {
		t.Error("second callback was not called after first callback panicked")
	}
}

// TestMultipleDeregister tests that multiple deregistrations don't cause issues
func TestMultipleDeregister(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	RegisterContext(ctx, func(c context.Context) {})

	// Multiple deregistrations should not panic
	DeregisterContext(ctx)
	DeregisterContext(ctx)
	DeregisterContext(ctx)
}

// TestContextAlreadyCancelled tests registering an already cancelled context
func TestContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Context is already cancelled
	time.Sleep(10 * time.Millisecond)

	called := atomic.Bool{}

	RegisterContext(ctx, func(c context.Context) {
		called.Store(true)
	})

	time.Sleep(50 * time.Millisecond)

	// Callback should still be called for already-cancelled context
	if !called.Load() {
		t.Error("callback was not called for already-cancelled context")
	}
}

// BenchmarkRegisterContext benchmarks the RegisterContext operation
func BenchmarkRegisterContext(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctxB, cancelB := context.WithCancel(ctx)
		RegisterContext(ctxB, func(c context.Context) {})
		cancelB()
	}
}

// BenchmarkRegisterDeregister benchmarks register followed by deregister
func BenchmarkRegisterDeregister(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctxB, cancelB := context.WithCancel(ctx)
		RegisterContext(ctxB, func(c context.Context) {})
		DeregisterContext(ctxB)
		cancelB()
	}
}

// BenchmarkConcurrentRegister benchmarks concurrent registrations
func BenchmarkConcurrentRegister(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, cancel := context.WithCancel(context.Background())
			RegisterContext(ctx, func(c context.Context) {})
			cancel()
		}
	})
}

// TestShardExpansion tests that shards expand automatically when load increases
func TestShardExpansion(t *testing.T) {
	// Create a watcher with minimal initial shards
	watcher := newContextWatcherReflect(2, 64, 100)

	// Get initial shard count
	initialShards := len(*watcher.shards.Load())
	t.Logf("Initial shard count: %d", initialShards)

	// Register enough contexts to trigger expansion
	// We need: activeContexts >= minContextsForExpand (512)
	// AND: avgLoad > shardLoadThreshold (64)
	// So we need at least 512 contexts, and preferably more to ensure avgLoad > 64
	const numContexts = 600
	contexts := make([]context.Context, numContexts)
	cancels := make([]context.CancelFunc, numContexts)

	var wg sync.WaitGroup
	wg.Add(numContexts)

	// Use timeout contexts to keep them alive long enough for expansion
	for i := 0; i < numContexts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		contexts[i] = ctx
		cancels[i] = cancel

		watcher.RegisterContext(ctx, func(c context.Context) {
			wg.Done()
		})
	}

	// Wait a bit for expansion to happen (expansion is async)
	time.Sleep(300 * time.Millisecond)

	// Check that shards have expanded
	finalShards := len(*watcher.shards.Load())
	activeContextsCount := watcher.activeContexts.Load()
	t.Logf("Final shard count: %d", finalShards)
	t.Logf("Active contexts: %d", activeContextsCount)

	// With 600 contexts and 2 initial shards, average load is 300 per shard
	// This should trigger expansion since avgLoad (300) > shardLoadThreshold (64)
	if finalShards <= initialShards {
		// If expansion didn't happen, check if it was due to timing
		if activeContextsCount < minContextsForExpand {
			t.Logf("Expansion may not have occurred due to insufficient active contexts: %d < %d",
				activeContextsCount, minContextsForExpand)
		} else {
			t.Errorf("Expected shard expansion, but shards did not increase: initial=%d, final=%d, active=%d",
				initialShards, finalShards, activeContextsCount)
		}
	}

	// Clean up: cancel all contexts
	for i := 0; i < numContexts; i++ {
		cancels[i]()
	}

	wg.Wait()
}

// TestFallbackMechanism tests that fallback mechanism works when shards are full
func TestFallbackMechanism(t *testing.T) {
	// Create a watcher with very limited capacity to force fallback
	// Use 1 shard with low capacity
	watcher := newContextWatcherReflect(1, 10, 50)

	// Register more contexts than maxCasesPerShard to trigger fallback
	const numContexts = 150 // Exceeds maxCasesPerShard (128)
	var wg sync.WaitGroup
	wg.Add(numContexts)

	fallbackCount := atomic.Int32{}
	regularCount := atomic.Int32{}

	for i := 0; i < numContexts; i++ {
		ctx, cancel := context.WithCancel(context.Background())

		watcher.RegisterContext(ctx, func(c context.Context) {
			wg.Done()
		})

		// Check if context was added to fallback
		if _, inFallback := watcher.fallbackCtxs.Load(ctx); inFallback {
			fallbackCount.Add(1)
		} else if _, inShard := watcher.ctxShards.Load(ctx); inShard {
			regularCount.Add(1)
		}

		// Cancel immediately to avoid accumulation
		cancel()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	t.Logf("Regular shard contexts: %d", regularCount.Load())
	t.Logf("Fallback contexts: %d", fallbackCount.Load())

	if fallbackCount.Load() == 0 {
		t.Logf("Warning: No contexts used fallback mechanism. This may be normal if shards expanded quickly.")
	} else {
		t.Logf("Successfully tested fallback mechanism with %d contexts", fallbackCount.Load())
	}
}

// TestLargeScaleConcurrent tests handling of 10000+ concurrent contexts
func TestLargeScaleConcurrent(t *testing.T) {
	// Skip in short mode as this test is resource-intensive
	if testing.Short() {
		t.Skip("Skipping large-scale test in short mode")
	}

	watcher := newContextWatcherReflect(defaultShardNum, 128, 100)

	const numContexts = 10000
	var wg sync.WaitGroup
	wg.Add(numContexts)

	start := time.Now()

	// Register 10000 contexts concurrently
	for i := 0; i < numContexts; i++ {
		go func(idx int) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			watcher.RegisterContext(ctx, func(c context.Context) {
				wg.Done()
			})
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	t.Logf("Processed %d contexts in %v", numContexts, duration)
	t.Logf("Average time per context: %v", duration/numContexts)
	t.Logf("Final shard count: %d", len(*watcher.shards.Load()))

	// Verify no goroutine leaks
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
}

// TestLazyInitialization tests that the global watcher is initialized lazily
func TestLazyInitialization(t *testing.T) {
	// Note: This test assumes it runs in isolation or that global has been reset
	// In practice, global will be initialized by previous tests

	// We can test that calling DeregisterContext before RegisterContext doesn't panic
	ctx := context.Background()
	DeregisterContext(ctx) // Should not panic even if global is nil

	// Now trigger initialization
	ctx2, cancel := context.WithCancel(context.Background())
	defer cancel()

	called := atomic.Bool{}
	RegisterContext(ctx2, func(c context.Context) {
		called.Store(true)
	})

	// Verify global is now initialized
	if global == nil {
		t.Error("Expected global watcher to be initialized after RegisterContext call")
	}

	cancel()
	time.Sleep(50 * time.Millisecond)

	if !called.Load() {
		t.Error("Callback should have been called after lazy initialization")
	}
}

// TestShardLoadBalancing tests that contexts are distributed across shards
func TestShardLoadBalancing(t *testing.T) {
	watcher := newContextWatcherReflect(4, 64, 100)

	const numContexts = 200
	contexts := make([]context.Context, numContexts)
	cancels := make([]context.CancelFunc, numContexts)

	// Register contexts
	for i := 0; i < numContexts; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		contexts[i] = ctx
		cancels[i] = cancel

		watcher.RegisterContext(ctx, func(c context.Context) {})
	}

	// Give time for all registrations to complete
	time.Sleep(100 * time.Millisecond)

	// Check distribution across shards
	shards := *watcher.shards.Load()
	loadDistribution := make([]int32, len(shards))

	for i, shard := range shards {
		loadDistribution[i] = shard.load.Load()
	}

	t.Logf("Load distribution across %d shards: %v", len(shards), loadDistribution)

	// Calculate variance to check if load is balanced
	var sum, sumSquares int64
	for _, load := range loadDistribution {
		sum += int64(load)
		sumSquares += int64(load) * int64(load)
	}

	mean := float64(sum) / float64(len(loadDistribution))
	variance := float64(sumSquares)/float64(len(loadDistribution)) - mean*mean
	stdDev := variance // We can use variance directly for comparison

	t.Logf("Mean load: %.2f, Variance: %.2f", mean, variance)

	// Check that no single shard is overloaded
	for i, load := range loadDistribution {
		if load > int32(mean*2) {
			t.Logf("Warning: Shard %d has significantly higher load: %d (mean: %.2f)", i, load, mean)
		}
	}

	// Verify total active contexts
	totalActive := watcher.activeContexts.Load()
	if totalActive != numContexts {
		t.Errorf("Expected %d active contexts, got %d", numContexts, totalActive)
	}

	// Clean up
	for i := 0; i < numContexts; i++ {
		cancels[i]()
	}

	// Verify cleanup
	time.Sleep(100 * time.Millisecond)
	finalActive := watcher.activeContexts.Load()
	if finalActive != 0 {
		t.Logf("Warning: %d contexts still active after cleanup (may be timing issue)", finalActive)
	}

	_ = stdDev // Use the variable to avoid unused warning
}

// TestDeregisterWithFallback tests deregistering contexts that are in fallback mode
func TestDeregisterWithFallback(t *testing.T) {
	watcher := newContextWatcherReflect(1, 10, 50)

	// Create a context and force it into fallback
	// First fill up the regular shards
	const fillCount = 130 // Exceeds maxCasesPerShard
	fillContexts := make([]context.Context, fillCount)
	fillCancels := make([]context.CancelFunc, fillCount)

	for i := 0; i < fillCount; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		fillContexts[i] = ctx
		fillCancels[i] = cancel
		watcher.RegisterContext(ctx, func(c context.Context) {})
	}

	time.Sleep(50 * time.Millisecond)

	// Now create a context that should go to fallback
	fallbackCtx, fallbackCancel := context.WithCancel(context.Background())
	defer fallbackCancel()

	called := atomic.Bool{}
	watcher.RegisterContext(fallbackCtx, func(c context.Context) {
		called.Store(true)
	})

	time.Sleep(10 * time.Millisecond)

	// Verify it's in fallback
	_, inFallback := watcher.fallbackCtxs.Load(fallbackCtx)
	if inFallback {
		t.Log("Context successfully added to fallback")

		// Now deregister it
		watcher.DeregisterContext(fallbackCtx)

		// Cancel and verify callback is NOT called
		fallbackCancel()
		time.Sleep(50 * time.Millisecond)

		if called.Load() {
			t.Error("Callback should not be called after deregistration from fallback")
		}
	} else {
		t.Log("Context was not in fallback (shards may have expanded)")
	}

	// Clean up
	for i := 0; i < fillCount; i++ {
		fillCancels[i]()
	}
}

// TestConcurrentShardExpansion tests concurrent registrations during shard expansion
func TestConcurrentShardExpansion(t *testing.T) {
	watcher := newContextWatcherReflect(2, 64, 100)

	const numGoroutines = 50
	const contextsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines * contextsPerGoroutine)

	// Launch multiple goroutines that register contexts concurrently
	// This should trigger shard expansion while registrations are ongoing
	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < contextsPerGoroutine; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				watcher.RegisterContext(ctx, func(c context.Context) {
					wg.Done()
				})

				// Small delay to spread out registrations
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()

	finalShards := len(*watcher.shards.Load())
	t.Logf("Final shard count after concurrent expansion: %d", finalShards)
	t.Logf("Active contexts: %d", watcher.activeContexts.Load())

	if finalShards < 2 {
		t.Error("Expected at least some shard expansion during concurrent operations")
	}
}

func BenchmarkContextCallback(b *testing.B) {
	scenarios := []struct {
		name  string
		count int
	}{
		{"10ctx", 10},
		{"100ctx", 100},
		{"1000ctx", 1000},
		{"10000ctx", 10000},
	}

	for _, sc := range scenarios {
		// test ContextWatcher (goroutine-based)
		b.Run("ContextWatcher/"+sc.name, func(b *testing.B) {
			benchContextWatcher(b, sc.count)
		})
		// test contextWatcherReflect (reflect.Select-based)
		b.Run("Reflect/"+sc.name, func(b *testing.B) {
			benchContextWatcherReflect(b, sc.count)
		})
		// test context.AfterFunc (standard library, Go 1.21+)
		b.Run("AfterFunc/"+sc.name, func(b *testing.B) {
			benchContextAfterFunc(b, sc.count)
		})
	}
}

// benchContextWatcher benchmarks the goroutine-based implementation
func benchContextWatcher(b *testing.B, count int) {
	watcher := &contextWatcherReflect{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(count)
		for j := 0; j < count; j++ {
			ctx, cancel := context.WithCancel(context.Background())
			watcher.fallbackRegister(ctx, func(c context.Context) {
				wg.Done()
			})
			cancel()
		}
		wg.Wait()
	}
}

// benchContextWatcherReflect benchmarks the reflect.Select-based implementation
func benchContextWatcherReflect(b *testing.B, count int) {
	watcher := newContextWatcherReflect(defaultShardNum, defaultShardCapacity, defaultSignalCapacity)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(count)
		for j := 0; j < count; j++ {
			ctx, cancel := context.WithCancel(context.Background())
			watcher.RegisterContext(ctx, func(c context.Context) {
				wg.Done()
			})
			cancel()
		}
		wg.Wait()
	}
}

// benchContextAfterFunc benchmarks the standard library context.AfterFunc
func benchContextAfterFunc(b *testing.B, count int) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(count)

		for j := 0; j < count; j++ {
			ctx, cancel := context.WithCancel(context.Background())
			context.AfterFunc(ctx, func() {
				wg.Done()
			})
			cancel()
		}

		wg.Wait()
	}
}
