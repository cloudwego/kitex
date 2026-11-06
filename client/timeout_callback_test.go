//go:build go1.21

package client

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func TestTimeoutCallbacksNoWatcherGoroutines(t *testing.T) {
	p := newTimeoutContext(context.Background(), 0)
	before := runtime.NumGoroutine()
	stops := make([]func() bool, 1000)
	var calls atomic.Int32
	for i := range stops {
		stops[i] = context.AfterFunc(p, func() { calls.Add(1) })
	}
	if got := runtime.NumGoroutine(); got > before+32 {
		t.Fatalf("callback registration spawned watcher goroutines: before=%d after=%d", before, got)
	}
	for _, stop := range stops {
		if !stop() || stop() {
			t.Fatal("stop must win exactly once")
		}
	}
	p.Cancel(context.Canceled)
	if calls.Load() != 0 {
		t.Fatal("stopped callback executed")
	}
	if p.Err() != context.Canceled {
		t.Fatal("cancellation lost")
	}
}

func TestTimeoutCallbackStopCancelRace(t *testing.T) {
	for i := 0; i < 1000; i++ {
		p := newTimeoutContext(context.Background(), 0)
		var calls atomic.Int32
		done := make(chan struct{})
		stop := context.AfterFunc(p, func() { calls.Add(1); close(done) })
		var workers sync.WaitGroup
		workers.Add(1)
		go func() { defer workers.Done(); p.Cancel(context.DeadlineExceeded) }()
		stopped := stop()
		workers.Wait()
		if !stopped {
			<-done
		}
		want := int32(1)
		if stopped {
			want = 0
		}
		if calls.Load() != want {
			t.Fatal("callback dispatched twice or lost", i, calls.Load(), want)
		}
		p.Cancel(errors.New("another concrete error type"))
		if p.Err() != context.DeadlineExceeded {
			t.Fatal("first cancellation error changed")
		}
	}
}

func TestTimeoutCallbackAfterCancel(t *testing.T) {
	p := newTimeoutContext(context.Background(), 0)
	p.Cancel(nil)
	if p.Err() != nil {
		t.Fatal("nil cancellation changed state")
	}
	p.Cancel(context.Canceled)
	done := make(chan struct{})
	stop := context.AfterFunc(p, func() { close(done) })
	if stop() {
		t.Fatal("already canceled context accepted stop")
	}
	<-done
}
