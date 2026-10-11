/*
 * Copyright 2024 CloudWeGo Authors
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

package grpc

import (
	"context"
	"sync"
	"sync/atomic"
)

// contextWithCancelReason implements context.Context with a cancel func for
// passing a reason while preserving the legacy Err behavior of the context
// passed directly to the handler.
type contextWithCancelReason struct {
	context.Context

	cancel      context.CancelFunc
	cancelCause context.CancelCauseFunc
	once        sync.Once
	reason      atomic.Value
}

func (c *contextWithCancelReason) Err() error {
	err := c.reason.Load()
	if err != nil {
		return err.(error)
	}
	return c.Context.Err()
}

func (c *contextWithCancelReason) CancelWithReason(reason error) {
	// The reason, cascade cause, and optional parent cancel must be published by
	// the same first caller. Without once, a concurrent loser could cancel the
	// parent before the winner installs streamCancelCause, permanently replacing
	// the cascade cause with context.Canceled. It also ensures reason is stored
	// only once, avoiding atomic.Value panics for different concrete error types.
	c.once.Do(func() {
		if reason != nil {
			c.reason.Store(reason)
			c.cancelCause(&streamCancelCause{err: reason})
		} else {
			c.cancelCause(nil)
		}
		if c.cancel != nil {
			c.cancel()
		}
	})
}

// streamCancelCause identifies cancellation originating from a Kitex stream.
// It is intentionally private so a user-provided context.WithCancelCause error,
// including a *status.Error, cannot be mistaken for a cascading cancellation.
// Unwrap keeps the original cancellation reason available to standard error APIs.
type streamCancelCause struct {
	err error
}

func (c *streamCancelCause) Error() string {
	return c.err.Error()
}

func (c *streamCancelCause) Unwrap() error {
	return c.err
}

type cancelWithReason func(reason error)

func newContextWithCancelReason(ctx context.Context, cancel context.CancelFunc) (context.Context, cancelWithReason) {
	causeCtx, cancelCause := context.WithCancelCause(ctx)
	ret := &contextWithCancelReason{Context: causeCtx, cancel: cancel, cancelCause: cancelCause}
	return ret, ret.CancelWithReason
}
