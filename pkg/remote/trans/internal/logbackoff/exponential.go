/*
 * Copyright 2026 CloudWeGo Authors
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

package logbackoff

import (
	"sync"
	"time"
)

const (
	initialInterval = time.Second
	maxInterval     = time.Minute
	resetAfter      = 5 * time.Minute
)

// Exponential limits repeated logs with exponential backoff.
type Exponential struct {
	mu       sync.Mutex
	lastLog  time.Time
	interval time.Duration
	count    uint64
}

// Observe records an event and reports its count and duration since the
// previous emitted log when the current event should be logged.
func (b *Exponential) Observe(now time.Time) (count uint64, elapsed time.Duration, allow bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.count++
	if b.lastLog.IsZero() {
		b.lastLog = now
		b.interval = initialInterval
		count = b.count
		b.count = 0
		return count, 0, true
	}
	elapsed = now.Sub(b.lastLog)
	if elapsed < resetAfter && now.Before(b.lastLog.Add(b.interval)) {
		return 0, 0, false
	}
	if elapsed >= resetAfter {
		b.interval = initialInterval
	} else if b.interval < maxInterval/2 {
		b.interval *= 2
	} else {
		b.interval = maxInterval
	}
	b.lastLog = now
	count = b.count
	b.count = 0
	return count, elapsed, true
}
