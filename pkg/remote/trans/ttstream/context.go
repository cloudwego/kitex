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

package ttstream

import (
	"context"
	"sync/atomic"
)

// when Kitex supported minimal Go version becomes v1.20, replace with context.WithCancelCause standard func
func cause(ctx context.Context) error {
	if c, ok := ctx.(ctxWithCancelCause); ok {
		return c.cause()
	}
	return ctx.Err()
}

type cancelWithCause func(error)

type ctxWithCancelCause struct {
	context.Context

	causeVal   atomic.Value
	cancelFunc context.CancelFunc
}

func (c *ctxWithCancelCause) cancelWithCause(cause error) {
	if cause != nil {
		c.causeVal.CompareAndSwap(nil, cause)
	}
	c.cancelFunc()
}

func (c *ctxWithCancelCause) cause() error {
	res := c.causeVal.Load()
	if res != nil {
		return res.(error)
	}
	return c.Err()
}

func newCtxWithCancelCause(ctx context.Context, cancelFunc context.CancelFunc) (context.Context, cancelWithCause) {
	newCtx := &ctxWithCancelCause{
		Context:    ctx,
		cancelFunc: cancelFunc,
	}
	return newCtx, newCtx.cancelWithCause
}
