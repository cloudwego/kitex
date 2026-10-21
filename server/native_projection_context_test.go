package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNativeProjectionCancellationPreservesValuesCauseAndDeadline(t *testing.T) {
	type key struct{}
	projected := context.WithValue(context.Background(), key{}, "final")
	decoded, cancel := context.WithDeadline(context.WithValue(context.Background(), key{}, "old"), time.Now().Add(20*time.Millisecond))
	defer cancel()
	ctx, stop, release := nativeProjectCancellation(projected, decoded)
	defer release()
	defer stop()
	if ctx.Value(key{}) != "final" {
		t.Fatal("projection retained stale values")
	}
	if actual, ok := ctx.Deadline(); !ok {
		t.Fatal("projection lost external deadline")
	} else if want, _ := decoded.Deadline(); !actual.Equal(want) {
		t.Fatal("projection extended external deadline")
	}
	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded || context.Cause(ctx) != context.DeadlineExceeded {
			t.Fatalf("deadline reason changed: err=%v cause=%v", ctx.Err(), context.Cause(ctx))
		}
	case <-time.After(time.Second):
		t.Fatal("projected context did not expire")
	}
}

func TestNativeProjectionCancellationAlreadyCanceledAndStopped(t *testing.T) {
	want := errors.New("external reason")
	decoded, cancel := context.WithCancelCause(context.Background())
	cancel(want)
	ctx, stop, release := nativeProjectCancellation(context.Background(), decoded)
	if ctx.Err() != context.Canceled || context.Cause(ctx) != want {
		t.Fatal("already canceled external context was discarded")
	}
	stop()
	release()
	decoded, cancel = context.WithCancelCause(context.Background())
	ctx, stop, release = nativeProjectCancellation(context.Background(), decoded)
	stop()
	cancel(want)
	if ctx.Err() != nil {
		t.Fatal("stopped projection callback ran after detaching")
	}
	release()
	if ctx.Err() != context.Canceled || context.Cause(ctx) != context.Canceled {
		t.Fatal("stopped projection callback ran after cleanup")
	}
}

func TestNativeProjectionCancellationFirstReasonRemainsStableForChildren(t *testing.T) {
	for _, projectedFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "projected-first", false: "decoded-first"}[projectedFirst], func(t *testing.T) {
			projected, cancelProjected := context.WithCancelCause(context.Background())
			defer cancelProjected(context.Canceled)
			decoded, cancelDecoded := context.WithDeadline(context.Background(), time.Now().Add(30*time.Millisecond))
			defer cancelDecoded()
			ctx, stop, release := nativeProjectCancellation(projected, decoded)
			defer release()
			defer stop()
			child, cancelChild := context.WithCancel(ctx)
			defer cancelChild()
			wantErr, wantCause := error(context.DeadlineExceeded), error(context.DeadlineExceeded)
			projectedCause := errors.New("projected canceled first")
			if projectedFirst {
				wantErr, wantCause = context.Canceled, projectedCause
				cancelProjected(projectedCause)
			}
			for _, target := range []context.Context{ctx, child} {
				select {
				case <-target.Done():
					if target.Err() != wantErr || context.Cause(target) != wantCause {
						t.Errorf("first source reason changed: error=%v cause=%v wantError=%v wantCause=%v", target.Err(), context.Cause(target), wantErr, wantCause)
					}
				case <-time.After(time.Second):
					t.Fatal("source cancellation did not propagate")
				}
			}
			if projectedFirst {
				<-decoded.Done()
			} else {
				cancelProjected(projectedCause)
			}
			for i := 0; i < 100; i++ {
				for _, target := range []context.Context{ctx, child} {
					if target.Err() != wantErr || context.Cause(target) != wantCause {
						t.Fatalf("second source changed the first cancellation: error=%v cause=%v wantError=%v wantCause=%v", target.Err(), context.Cause(target), wantErr, wantCause)
					}
				}
			}
		})
	}
}

func TestNativeProjectionCancellationConcurrentSourcesAndAfterFunc(t *testing.T) {
	for i := 0; i < 50; i++ {
		projected, cancelProjected := context.WithCancelCause(context.Background())
		decoded, cancelDecoded := context.WithCancelCause(context.Background())
		ctx, stop, release := nativeProjectCancellation(projected, decoded)
		child, cancelChild := context.WithCancel(ctx)
		after := ctx.(interface{ AfterFunc(func()) func() bool })
		stopped := make(chan struct{})
		stopCallback := after.AfterFunc(func() { close(stopped) })
		if !stopCallback() || stopCallback() {
			t.Fatal("projection AfterFunc stop did not win exactly once")
		}
		callback := make(chan error, 1)
		stopRunning := after.AfterFunc(func() { callback <- context.Cause(ctx) })
		first, second := errors.New("projected cancellation"), errors.New("decoded cancellation")
		var joined sync.WaitGroup
		joined.Add(2)
		go func() { defer joined.Done(); cancelProjected(first) }()
		go func() { defer joined.Done(); cancelDecoded(second) }()
		joined.Wait()
		select {
		case <-child.Done():
		case <-time.After(time.Second):
			t.Fatal("simultaneous cancellation did not reach standard child")
		}
		cause := context.Cause(ctx)
		if (cause != first && cause != second) || ctx.Err() != context.Canceled || child.Err() != context.Canceled || context.Cause(child) != cause {
			t.Fatal("simultaneous cancellation mixed source error and cause")
		}
		select {
		case observed := <-callback:
			if observed != cause || stopRunning() {
				t.Fatal("projection callback observed an unstable cause or ran after being stopped")
			}
		case <-time.After(time.Second):
			t.Fatal("projection callback deadlocked while reading context")
		}
		select {
		case <-stopped:
			t.Fatal("stopped projection callback executed")
		default:
		}
		stop()
		release()
		cancelChild()
	}
}
