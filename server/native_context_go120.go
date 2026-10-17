//go:build !go1.21

package server

import (
	"context"
	"sync/atomic"
	"time"
)

func nativeAfterCancel(ctx context.Context, cancel func()) func() bool {
	var state atomic.Uint32
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if state.CompareAndSwap(0, 1) {
				cancel()
			}
		case <-stopped:
		}
	}()
	return func() bool {
		if !state.CompareAndSwap(0, 2) {
			return false
		}
		close(stopped)
		return true
	}
}

type nativeDetachedContext struct{ context.Context }

func (nativeDetachedContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (nativeDetachedContext) Done() <-chan struct{}       { return nil }
func (nativeDetachedContext) Err() error                  { return nil }

func nativeCompleteContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(nativeDetachedContext{ctx})
}
