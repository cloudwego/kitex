package utils

import (
	"context"
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
	for i := 0; i < 30000; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		idx := i
		ds.Add(ctx.Done(), func() {
			atomic.AddUint32(&canceled, 1)
			if idx < 10000 || idx >= 20000 {
				t.Fatal("unexpected index")
			}
		})
		slices = append(slices, struct {
			ctx    context.Context
			cancel context.CancelFunc
		}{ctx, cancel})
	}
	test.Assert(t, atomic.LoadUint32(&canceled) == 0)
	ds.mu.Lock()
	test.Assert(t, ds.count == 30000)
	test.Assert(t, len(ds.selectors) == 30000/singleSelectorMaxCases+1)
	test.Assert(t, ds.stop == 0)
	ds.mu.Unlock()

	// remove
	for i := 0; i < 10000; i++ {
		ds.Delete(slices[i].ctx.Done())
	}
	test.Assert(t, atomic.LoadUint32(&canceled) == 0)
	ds.mu.Lock()
	test.Assert(t, ds.count == 20000)
	test.Assert(t, len(ds.selectors) == 20000/singleSelectorMaxCases+1)
	test.Assert(t, ds.stop == 0)
	ds.mu.Unlock()

	// cancel
	for i := 10000; i < 20000; i++ {
		slices[i].cancel()
	}
	time.Sleep(500 * time.Millisecond)
	test.Assert(t, atomic.LoadUint32(&canceled) == 10000)
	ds.mu.Lock()
	test.Assert(t, ds.count == 10000)
	test.Assert(t, len(ds.selectors) == 10000/singleSelectorMaxCases+1)
	test.Assert(t, ds.stop == 0)
	ds.mu.Unlock()

	// stop
	ds.Close()
	ds.mu.Lock()
	test.Assert(t, ds.stop == 1)
	ds.mu.Unlock()
	for i := 20000; i < 30000; i++ {
		slices[i].cancel()
	}
	time.Sleep(500 * time.Millisecond)
	test.Assert(t, atomic.LoadUint32(&canceled) == 10000)
}

func TestSingleCase(t *testing.T) {
	ds := NewCtxDoneSelector()
	defer ds.Close()

	var triggered uint32
	done := make(chan struct{})
	ds.Add(done, func() { atomic.StoreUint32(&triggered, 1) })
	close(done)

	time.Sleep(100 * time.Millisecond) // wait for callback
	if atomic.LoadUint32(&triggered) != 1 {
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
