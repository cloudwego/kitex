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
	"sync"
)

// concurrencyLimit implements ConcurrencyLimiter.
type concurrencyLimit struct {
	limit    int32
	inflight int32
	blocking bool // Acquire will block until could return true
	lock     *sync.RWMutex
	cond     *sync.Cond
}

func NewConcurrencyLimiter(limit int, blocking bool) ConcurrencyLimiter {
	var lock = &sync.RWMutex{}
	var cond = sync.NewCond(lock)
	return &concurrencyLimit{
		limit: int32(limit), inflight: 0, blocking: blocking,
		lock: lock, cond: cond,
	}
}

func (l *concurrencyLimit) Acquire(ctx context.Context) bool {
	l.lock.Lock()
	for l.inflight >= l.limit {
		if !l.blocking {
			l.lock.Unlock()
			return false
		}
		l.cond.Wait()
	}
	l.inflight++
	l.lock.Unlock()
	return true
}

// Release decrease the connection counter.
func (l *concurrencyLimit) Release(ctx context.Context) {
	l.lock.Lock()
	limited := l.inflight >= l.limit
	l.inflight--
	l.lock.Unlock()
	if limited {
		l.cond.Signal()
	}
}

// UpdateLimit updates the limit.
func (l *concurrencyLimit) UpdateLimit(limit int) {
	l.lock.Lock()
	l.limit = int32(limit)
	l.lock.Unlock()
}

// Status returns the current status.
func (l *concurrencyLimit) Status(ctx context.Context) (limit, inflight int) {
	l.lock.RLock()
	limit = int(l.limit)
	inflight = int(l.inflight)
	l.lock.RUnlock()
	return
}
