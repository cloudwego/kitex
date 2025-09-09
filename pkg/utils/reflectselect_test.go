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

package utils

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestBasic(t *testing.T) {
	ds := NewCtxDoneSelector()

	var canceled uint32
	var slices []struct {
		ctx    context.Context
		cancel context.CancelFunc
	}
	var hasCanceled [1000]uint32
	for i := 0; i < 3000; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		idx := i
		ds.Add(ctx.Done(), func() {
			atomic.AddUint32(&canceled, 1)
			if idx < 1000 || idx >= 2000 {
				t.Fatal("unexpected index")
			}
			atomic.StoreUint32(&hasCanceled[idx-1000], 1)
		})
		slices = append(slices, struct {
			ctx    context.Context
			cancel context.CancelFunc
		}{ctx, cancel})
	}
	test.Assert(t, atomic.LoadUint32(&canceled) == 0)
	ds.mu.Lock()
	test.Assert(t, ds.count == 3000)
	test.Assert(t, len(ds.selectors) == 3000/singleSelectorMaxCases+1)
	test.Assert(t, ds.stop == 0)
	ds.mu.Unlock()

	// remove
	for i := 0; i < 1000; i++ {
		ds.Delete(slices[i].ctx.Done())
	}
	test.Assert(t, atomic.LoadUint32(&canceled) == 0)
	ds.mu.Lock()
	test.Assert(t, ds.count == 2000)
	test.Assert(t, len(ds.selectors) == 2000/singleSelectorMaxCases+1)
	test.Assert(t, ds.stop == 0)
	ds.mu.Unlock()

	// cancel
	for i := 1000; i < 2000; i++ {
		slices[i].cancel()
	}
	time.Sleep(1 * time.Second)
	test.Assert(t, atomic.LoadUint32(&canceled) == 1000)
	for i := 0; i < 1000; i++ {
		if atomic.LoadUint32(&hasCanceled[i]) != 1 {
			t.Fatal("context cancel not triggered")
		}
	}
	ds.mu.Lock()
	test.Assert(t, ds.count == 1000)
	test.Assert(t, len(ds.selectors) == 1000/singleSelectorMaxCases+1)
	test.Assert(t, ds.stop == 0)
	ds.mu.Unlock()

	// stop
	ds.Close()
	ds.mu.Lock()
	test.Assert(t, ds.stop == 1)
	ds.mu.Unlock()
	for i := 2000; i < 3000; i++ {
		slices[i].cancel()
	}
	time.Sleep(1 * time.Second)
	test.Assert(t, atomic.LoadUint32(&canceled) == 1000)
}

func TestSingleCase(t *testing.T) {
	ds := NewCtxDoneSelector()
	defer ds.Close()

	triggered := make(chan struct{})
	done := make(chan struct{})
	now := time.Now()
	ds.Add(done, func() { close(triggered) })
	t.Logf("add cost: %v", time.Since(now))
	now = time.Now()
	close(done)

	select {
	case <-triggered:
		t.Logf("callback cost: %v", time.Since(now))
	case <-time.After(1 * time.Second):
		t.Fatal("callback not triggered")
	}
}

func TestMultipleCases(t *testing.T) {
	const count = 50
	ds := NewCtxDoneSelector()
	defer ds.Close()

	var triggers [count]atomic.Bool
	dones := make([]chan struct{}, count)

	for i := range dones {
		dones[i] = make(chan struct{})
		idx := i
		ds.Add(dones[i], func() { triggers[idx].Store(true) })
	}

	// reverse close
	for i := count - 1; i >= 0; i-- {
		close(dones[i])
		time.Sleep(10 * time.Millisecond)
	}

	for i := range triggers {
		if !triggers[i].Load() {
			t.Fatalf("callback not triggered: %d", i)
		}
	}
}

func TestDynamicExpansion(t *testing.T) {
	ds := NewCtxDoneSelector()
	defer ds.Close()

	const count = 300
	var wg sync.WaitGroup
	dones := make([]chan struct{}, count)

	for i := range dones {
		dones[i] = make(chan struct{})
		wg.Add(1)
		ds.Add(dones[i], wg.Done)
	}

	// randomly delete
	for i := 0; i < 100; i++ {
		ds.Delete(dones[i*3])
		wg.Done()
	}

	// trigger other channel
	for i := range dones {
		if i%3 != 0 {
			close(dones[i])
		}
	}

	if !waitWithTimeout(&wg, 2*time.Second) {
		t.Fatal("not all callbacks triggered")
	}
}

func TestDeleteHandler(t *testing.T) {
	ds := NewCtxDoneSelector()
	defer ds.Close()

	done := make(chan struct{})
	ds.Add(done, func() { t.Error("deleted callback triggered") })
	ds.Delete(done)
	close(done)

	time.Sleep(100 * time.Millisecond)
}

func TestConcurrentOperations(t *testing.T) {
	ds := NewCtxDoneSelector()
	defer ds.Close()

	const workers = 50
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	// concurrent add
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				done := make(chan struct{})
				ds.Add(done, func() {})
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	// concurrent delete
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				done := make(chan struct{})
				ds.Add(done, func() {})
				ds.Delete(done)
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
}

func TestSelectorClose(t *testing.T) {
	ds := NewCtxDoneSelector()

	done := make(chan struct{})
	ds.Add(done, func() { t.Error("closed selector should not trigger callback") })
	ds.Close()

	// closed selector should ignore add
	ds.Add(make(chan struct{}), func() { t.Error("closed selector add success") })
	close(done)

	time.Sleep(100 * time.Millisecond)
}

func TestContextIntegration(t *testing.T) {
	ds := NewCtxDoneSelector()
	defer ds.Close()

	ctx, cancel := context.WithCancel(context.Background())
	var triggered uint32
	ds.Add(ctx.Done(), func() { atomic.StoreUint32(&triggered, 1) })

	cancel()
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadUint32(&triggered) != 1 {
		t.Fatal("context cancel should trigger callback")
	}
}

func BenchmarkSingleCase(b *testing.B) {
	ds := NewCtxDoneSelector()
	defer ds.Close()

	triggered := make(chan struct{})
	done := make(chan struct{})
	ds.Add(done, func() { close(triggered) })

	for i := 0; i < b.N; i++ {
		triggered := make(chan struct{})
		done := make(chan struct{})
		ds.Add(done, func() { close(triggered) })
		close(done)
		<-triggered
	}
}

func BenchmarkParallelCase(b *testing.B) {
	ds := NewCtxDoneSelector()
	defer ds.Close()

	var wg sync.WaitGroup
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < b.N; i++ {
				triggered := make(chan struct{})
				done := make(chan struct{})
				ds.Add(done, func() { close(triggered) })
				close(done)
				<-triggered
			}
		}()
	}
	wg.Wait()
}

func BenchmarkCtxDoneSelector_Add(b *testing.B) {
	b.ReportAllocs()
	ds := NewCtxDoneSelector()
	defer ds.Close()

	var mu sync.Mutex
	var cancels []context.CancelFunc
	var wg sync.WaitGroup

	// Concurrently add channels
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, cancel := context.WithCancel(context.Background())
			mu.Lock()
			cancels = append(cancels, cancel)
			mu.Unlock()
			wg.Add(1)
			ds.Add(ctx.Done(), wg.Done)
		}
	})
	b.StopTimer()

	// Simulate concurrent cancel (e.g., connections closed)
	for _, cancel := range cancels {
		cancel()
	}

	now := time.Now()
	// Wait for all callbacks to finish
	if !waitWithTimeout(&wg, 30*time.Second) {
		b.Fatal("not all callbacks triggered")
	}
	b.Logf("len(cancels): %v, cost: %v", len(cancels), time.Since(now))
}

func waitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return true
	case <-time.After(timeout):
		return false
	}
}
