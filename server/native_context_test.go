package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNativeCancellationRegistration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	called := make(chan struct{})
	stop := nativeAfterCancel(ctx, func() { close(called) })
	if !stop() || stop() {
		t.Fatal("cancellation registration did not stop exactly once")
	}
	cancel()
	select {
	case <-called:
		t.Fatal("stopped cancellation callback ran")
	case <-time.After(10 * time.Millisecond):
	}
	stop = nativeAfterCancel(ctx, func() { close(called) })
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("already canceled request did not cancel its handler")
	}
	if stop() {
		t.Fatal("completed cancellation callback was stopped")
	}
}

func TestNativeCompletionContext(t *testing.T) {
	type key struct{}
	parent, expire := context.WithTimeout(context.WithValue(context.Background(), key{}, "metadata"), time.Minute)
	defer expire()
	parent, cancel := context.WithCancelCause(parent)
	cancel(errors.New("request canceled"))
	ctx, release := nativeCompleteContext(parent)
	if release != nil {
		defer release()
	}
	if ctx.Err() != nil || context.Cause(ctx) != nil || ctx.Value(key{}) != "metadata" {
		t.Fatalf("completion lost values or inherited cancellation: error=%v cause=%v value=%v", ctx.Err(), context.Cause(ctx), ctx.Value(key{}))
	}
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("completion inherited the request deadline")
	}
	select {
	case <-ctx.Done():
		t.Fatal("completion was canceled with its request")
	default:
	}
}
