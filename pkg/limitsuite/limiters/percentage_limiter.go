/*
 * Copyright 2023 CloudWeGo Authors
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

package limiters

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limitsuite"
)

var _ limitsuite.Limiter = (*PercentageLimiter)(nil)

const (
	defaultName           = "PercentageLimiter"
	defaultInterval       = time.Second
	defaultMinSampleCount = 20

	percentageMultiplier = 10000
)

// CandidateChecker determines (in a middleware) whether a request should be checked by the limiter
type CandidateChecker func(ctx context.Context, request, response interface{}) bool

// PercentageLimiter blocks candidate requests exceeding given percentage limit
type PercentageLimiter struct {
	name string

	interval         time.Duration
	percentage       int64
	minSampleCount   int32
	isLimitCandidate CandidateChecker

	stopSignal chan byte

	totalCount             int64
	allowedCandidatesCount int64
}

// PercentageLimiterOption for specifying optional parameters for initialization
type PercentageLimiterOption func(l *PercentageLimiter)

// WithName specifies the name (for logging purpose)
func WithName(name string) PercentageLimiterOption {
	return func(l *PercentageLimiter) {
		l.name = name
	}
}

// WithInterval specifies the sleep time before resetting counters.
// Resetting is disabled if negative interval is given.
func WithInterval(interval time.Duration) PercentageLimiterOption {
	return func(l *PercentageLimiter) {
		l.interval = interval
	}
}

// WithMinSampleCount specifies the count of minimal requests needed before applying the limit in each cycle
func WithMinSampleCount(count int) PercentageLimiterOption {
	return func(l *PercentageLimiter) {
		l.minSampleCount = int32(count)
	}
}

// NewPercentageLimiter initializes a percentage limiter with given parameters and options
func NewPercentageLimiter(percentage float64, candidateChecker CandidateChecker,
	opts ...PercentageLimiterOption) *PercentageLimiter {
	limiter := &PercentageLimiter{
		percentage:       int64(percentage * percentageMultiplier), // int64 multiplication is faster
		isLimitCandidate: candidateChecker,
		name:             defaultName,
		interval:         defaultInterval,
		minSampleCount:   defaultMinSampleCount,
		stopSignal:       make(chan byte, 1),
	}
	for _, option := range opts {
		option(limiter)
	}
	if limiter.interval > 0 {
		limiter.resetPeriodically()
	}
	return limiter
}

// String returns an interpretation for logging purpose
func (l *PercentageLimiter) String() string {
	return fmt.Sprintf("PercentageLimiter(interval=%v, percentage=%v/%v, minSampleCount=%v, name=%v)",
		l.interval, l.percentage, percentageMultiplier, l.minSampleCount, l.name)
}

// resetPeriodically calls Reset every l.interval and stops when Close is called
func (l *PercentageLimiter) resetPeriodically() {
	go func() {
		if err := recover(); err != nil {
			klog.Errorf("KITEX: PercentageLimiter.resetPeriodically panicked, err=%v, stack=%v", err, debug.Stack())
		}
		ticker := time.NewTicker(l.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.Reset()
			case <-l.stopSignal:
				return
			}
		}
	}()
}

// Allow determines whether a request should be limited(false) or not(true)
func (l *PercentageLimiter) Allow(ctx context.Context, request, response interface{}) bool {
	total := atomic.AddInt64(&l.totalCount, 1)
	if !l.isLimitCandidate(ctx, request, response) {
		return true
	}
	allowedCandidates := atomic.AddInt64(&l.allowedCandidatesCount, 1)
	if total <= int64(l.minSampleCount) {
		return true
	}
	if !l.exceedPercentage(allowedCandidates, total) {
		return true
	}
	// rejected
	atomic.AddInt64(&l.allowedCandidatesCount, -1)
	return false
}

// exceedPercentage calculates whether candidateCount/totalCount > percentage
// the calculation is transformed to integer multiplication for performance
func (l *PercentageLimiter) exceedPercentage(candidateCount, totalCount int64) bool {
	return candidateCount*percentageMultiplier > totalCount*l.percentage
}

// Reset resets the counters
func (l *PercentageLimiter) Reset() {
	// It might be ideal to reset the two counters atomically, but it'll be slower due to a larger lock
	// Here we reset the allowedCandidatesCount first, and relies on minSampleCount to bypass the time gap
	atomic.StoreInt64(&l.allowedCandidatesCount, 0)
	atomic.StoreInt64(&l.totalCount, 0)
}

// Close terminates the goroutine by sending a stopSignal
func (l *PercentageLimiter) Close() {
	select {
	case l.stopSignal <- 0:
	default:
	}
}
