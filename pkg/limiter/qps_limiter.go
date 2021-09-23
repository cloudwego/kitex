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
	"sync/atomic"
	"time"
)

var fixedWindowTime = time.Second

// qpsLimiter implements the RateLimiter interface.
type qpsLimiter struct {
	limit      int32
	tokens     int32
	interval   time.Duration
	once       int32
	ticker     *time.Ticker
	tickerDone chan bool
}

// NewQPSLimiter creates qpsLimiter.
func NewQPSLimiter(interval time.Duration, limit int) RateLimiter {
	l := &qpsLimiter{
		limit:    int32(limit),
		tokens:   int32(limit),
		interval: interval,
		once:     calcOnce(interval, limit),
	}
	go l.startTicker(interval)
	return l
}

// UpdateLimit update limitation of QPS. It is **not** concurrent-safe.
func (l *qpsLimiter) UpdateLimit(limit int) {
	once := calcOnce(l.interval, limit)
	atomic.StoreInt32(&l.limit, int32(limit))
	atomic.StoreInt32(&l.once, once)
	l.resetTokens()
}

// UpdateQPSLimit update the interval and limit. It is **not** concurrent-safe.
func (l *qpsLimiter) UpdateQPSLimit(interval time.Duration, limit int) {
	atomic.StoreInt32(&l.limit, int32(limit))
	once := calcOnce(interval, limit)
	atomic.StoreInt32(&l.once, once)
	l.resetTokens()
	if interval != l.interval {
		l.interval = interval
		l.stopTicker()
		go l.startTicker(interval)
	}
}

// Acquire adds 1.
func (l *qpsLimiter) Acquire() bool {
	if atomic.LoadInt32(&l.tokens) <= 0 {
		return false
	}
	return atomic.AddInt32(&l.tokens, -1) >= 0
}

// Status returns the current status.
func (l *qpsLimiter) Status() (max, cur int, interval time.Duration) {
	max = int(atomic.LoadInt32(&l.limit))
	cur = int(atomic.LoadInt32(&l.tokens))
	interval = l.interval
	return
}

func (l *qpsLimiter) startTicker(interval time.Duration) {
	l.ticker = time.NewTicker(interval)
	defer l.ticker.Stop()
	l.tickerDone = make(chan bool, 1)
	tc := l.ticker.C
	td := l.tickerDone
	// ticker and tickerDone can be reset, cannot use l.ticker or l.tickerDone directly
	for {
		select {
		case <-tc:
			l.updateToken()
		case <-td:
			return
		}
	}
}

func (l *qpsLimiter) stopTicker() {
	if l.tickerDone == nil {
		return
	}
	select {
	case l.tickerDone <- true:
	default:
	}
}

// Some deviation is allowed here to gain better performance.
func (l *qpsLimiter) updateToken() {
	if atomic.LoadInt32(&l.limit) < atomic.LoadInt32(&l.tokens) {
		return
	}

	once := atomic.LoadInt32(&l.once)

	delta := atomic.LoadInt32(&l.limit) - atomic.LoadInt32(&l.tokens)

	if delta > once || delta < 0 {
		delta = once
	}

	newTokens := atomic.AddInt32(&l.tokens, delta)
	if newTokens < once {
		atomic.StoreInt32(&l.tokens, once)
	}
}

func calcOnce(interval time.Duration, limit int) int32 {
	if interval > time.Second {
		interval = time.Second
	}
	once := int32(float64(limit) / (fixedWindowTime.Seconds() / interval.Seconds()))
	if once < 0 {
		once = 0
	}
	return once
}

func (l *qpsLimiter) resetTokens() {
	limit := atomic.LoadInt32(&l.limit)
	if atomic.LoadInt32(&l.tokens) > limit {
		atomic.StoreInt32(&l.tokens, l.limit)
	}
}
