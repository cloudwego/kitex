/*
 * Copyright 2021 CloudWeGo Authors
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

package limiter

import (
	"context"
	"sync/atomic"
)

// concurrencyLimiter implements ConcurrencyLimiter.
type concurrencyLimiter struct {
	lim  int32
	curr int32
}

// NewConcurrencyLimiter returns a new ConcurrencyLimiter with the given limit.
func NewConcurrencyLimiter(lim int) ConcurrencyLimiter {
	return &concurrencyLimiter{int32(lim), 0}
}

// Acquire tries to increase the concurrency by 1.
// The return value indicates whether the operation is allowed under the concurrency limitation.
// Acquired is executed in `OnActive` which is called when a new connection is accepted, so even if the limit is reached
// the count is still need increase, but return false will lead the connection is closed then Release also be executed.
func (ml *concurrencyLimiter) Acquire(ctx context.Context) bool {
	x := atomic.AddInt32(&ml.curr, 1)
	return x <= atomic.LoadInt32(&ml.lim)
}

// Release decrease the concurrency by 1.
func (ml *concurrencyLimiter) Release(ctx context.Context) {
	atomic.AddInt32(&ml.curr, -1)
}

// UpdateLimit updates the limit.
func (ml *concurrencyLimiter) UpdateLimit(lim int) {
	atomic.StoreInt32(&ml.lim, int32(lim))
}

// Status returns the current status.
func (ml *concurrencyLimiter) Status(ctx context.Context) (limit, occupied int) {
	limit = int(atomic.LoadInt32(&ml.lim))
	occupied = int(atomic.LoadInt32(&ml.curr))
	return
}
