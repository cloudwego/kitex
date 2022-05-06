// Copyright 2021 ByteDance Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package circuitbreaker

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/lang/syncx"
)

// bucket holds counts of failures and successes
type bucket struct {
	failure int64
	success int64
	timeout int64
}

// Reset resets the counts to 0 and refreshes the time stamp
func (b *bucket) Reset() {
	atomic.StoreInt64(&b.failure, 0)
	atomic.StoreInt64(&b.success, 0)
	atomic.StoreInt64(&b.timeout, 0)
}

func (b *bucket) Fail() {
	atomic.AddInt64(&b.failure, 1)
}

func (b *bucket) Succeed() {
	atomic.AddInt64(&b.success, 1)
}

func (b *bucket) Timeout() {
	atomic.AddInt64(&b.timeout, 1)
}

func (b *bucket) Failures() int64 {
	return atomic.LoadInt64(&b.failure)
}

func (b *bucket) Successes() int64 {
	return atomic.LoadInt64(&b.success)
}

func (b *bucket) Timeouts() int64 {
	return atomic.LoadInt64(&b.timeout)
}

// window maintains a ring of buckets and increments the failure and success
// counts of the current bucket.
type window struct {
	rw      syncx.RWMutex
	oldest  int32    // oldest perPBucket index
	latest  int32    // latest perPBucket index
	buckets []bucket // buckets this perPWindow holds

	bucketTime time.Duration // time each perPBucket holds
	bucketNums int32         // the numbe of buckets
	inWindow   int32         // the number of buckets in the perPWindow

	allSuccess int64
	allFailure int64
	allTimeout int64

	errStart int64
	conseErr int64
}

// newWindow .
func newWindow() metricer {
	m, _ := newWindowWithOptions(defaultBucketTime, defaultBucketNums)
	return m
}

// newWindowWithOptions creates a new perPWindow.
func newWindowWithOptions(bucketTime time.Duration, bucketNums int32) (metricer, error) {
	if bucketNums < 100 {
		return nil, fmt.Errorf("BucketNums can't be less than 100")
	}

	w := new(window)
	w.rw = syncx.NewRWMutex()
	w.bucketNums = bucketNums
	w.bucketTime = bucketTime
	w.buckets = make([]bucket, w.bucketNums)

	w.Reset()
	return w, nil
}

// Success records a success in the current perPBucket.
func (w *window) Succeed() {
	rwx := w.rw.RLocker()
	rwx.Lock()
	b := w.getBucket()
	atomic.StoreInt64(&w.errStart, 0)
	atomic.StoreInt64(&w.conseErr, 0)
	atomic.AddInt64(&w.allSuccess, 1)
	rwx.Unlock()
	b.Succeed()
}

// Fail records a failure in the current perPBucket.
func (w *window) Fail() {
	w.rw.Lock()
	b := w.getBucket()
	atomic.AddInt64(&w.conseErr, 1)
	atomic.AddInt64(&w.allFailure, 1)
	if atomic.LoadInt64(&w.errStart) == 0 {
		atomic.StoreInt64(&w.errStart, time.Now().UnixNano())
	}
	w.rw.Unlock()
	b.Fail()
}

// Timeout records a timeout in the current perPBucket
func (w *window) Timeout() {
	w.rw.Lock()
	b := w.getBucket()
	atomic.AddInt64(&w.conseErr, 1)
	atomic.AddInt64(&w.allTimeout, 1)
	if atomic.LoadInt64(&w.errStart) == 0 {
		atomic.StoreInt64(&w.errStart, time.Now().UnixNano())
	}
	w.rw.Unlock()
	b.Timeout()
}

func (w *window) Counts() (successes, failures, timeouts int64) {
	return w.Successes(), w.Failures(), w.Timeouts()
}

// Successes returns the total number of successes recorded in all buckets.
func (w *window) Successes() int64 {
	return atomic.LoadInt64(&w.allSuccess)
}

// Failures returns the total number of failures recorded in all buckets.
func (w *window) Failures() int64 {
	return atomic.LoadInt64(&w.allFailure)
}

// Timeouts returns the total number of Timeout recorded in all buckets.
func (w *window) Timeouts() int64 {
	return atomic.LoadInt64(&w.allTimeout)
}

func (w *window) ConseErrors() int64 {
	return atomic.LoadInt64(&w.conseErr)
}

func (w *window) ConseTime() time.Duration {
	return time.Duration(time.Now().UnixNano() - atomic.LoadInt64(&w.errStart))
}

// ErrorRate returns the error rate calculated over all buckets, expressed as
// a floating point number (e.g. 0.9 for 90%)
func (w *window) ErrorRate() float64 {
	successes, failures, timeouts := w.Counts()

	if (successes + failures + timeouts) == 0 {
		return 0.0
	}

	return float64(failures+timeouts) / float64(successes+failures+timeouts)
}

func (w *window) Samples() int64 {
	successes, failures, timeouts := w.Counts()

	return successes + failures + timeouts
}

// Reset resets this perPWindow
func (w *window) Reset() {
	w.rw.Lock()
	atomic.StoreInt32(&w.oldest, 0)
	atomic.StoreInt32(&w.latest, 0)
	atomic.StoreInt32(&w.inWindow, 1)
	atomic.StoreInt64(&w.conseErr, 0)
	atomic.StoreInt64(&w.allSuccess, 0)
	atomic.StoreInt64(&w.allFailure, 0)
	atomic.StoreInt64(&w.allTimeout, 0)
	w.getBucket().Reset()
	w.rw.Unlock() // don't use defer
}

func (w *window) tick() {
	w.rw.Lock()
	// 这一段必须在前面，因为latest可能会覆盖oldest
	if w.inWindow == w.bucketNums {
		// the lastest covered the oldest(latest == oldest)
		oldBucket := &w.buckets[w.oldest]
		atomic.AddInt64(&w.allSuccess, -oldBucket.Successes())
		atomic.AddInt64(&w.allFailure, -oldBucket.Failures())
		atomic.AddInt64(&w.allTimeout, -oldBucket.Timeouts())
		w.oldest++
		if w.oldest >= w.bucketNums {
			w.oldest = 0
		}
	} else {
		w.inWindow++
	}

	w.latest++
	if w.latest >= w.bucketNums {
		w.latest = 0
	}
	w.getBucket().Reset()
	w.rw.Unlock()
}

func (w *window) getBucket() *bucket {
	return &w.buckets[atomic.LoadInt32(&w.latest)]
}
