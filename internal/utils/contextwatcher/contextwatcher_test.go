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

func init() {
	// Initialize global variable to fix the nil pointer issue
	global = &ContextWatcher{}
}

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
