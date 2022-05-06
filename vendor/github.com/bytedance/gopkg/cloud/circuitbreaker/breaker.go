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
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/lang/syncx"
)

const (
	// cooling timeout is the time the breaker stay in Open before becoming HalfOpen
	defaultCoolingTimeout = time.Second * 5

	// detect timeout is the time interval between every detect in HalfOpen
	defaultDetectTimeout = time.Millisecond * 200

	// halfopen success is the threshold when the breaker is in HalfOpen;
	// after exceeding consecutively this times, it will change its State from HalfOpen to Closed;
	defaultHalfOpenSuccesses = 2
)

// breaker is the base of a circuit breaker.
type breaker struct {
	rw syncx.RWMutex

	metricer metricer // metrics all success, error and timeout within some time

	state           State     // State now
	openTime        time.Time // the time when the breaker become Open recently
	lastRetryTime   time.Time // last retry time when in HalfOpen State
	halfopenSuccess int32     // consecutive successes when HalfOpen
	isFixed         bool

	options Options

	now func() time.Time // Default value is time.Now, caller may use some high-performance custom time now func here
}

// newBreaker creates a base breaker with a specified options
func newBreaker(options Options) (*breaker, error) {
	if options.Now == nil {
		options.Now = time.Now
	}

	if options.BucketTime <= 0 {
		options.BucketTime = defaultBucketTime
	}

	if options.BucketNums <= 0 {
		options.BucketNums = defaultBucketNums
	}

	if options.CoolingTimeout <= 0 {
		options.CoolingTimeout = defaultCoolingTimeout
	}

	if options.DetectTimeout <= 0 {
		options.DetectTimeout = defaultDetectTimeout
	}

	if options.HalfOpenSuccesses <= 0 {
		options.HalfOpenSuccesses = defaultHalfOpenSuccesses
	}

	var window metricer
	var err error
	if options.EnableShardP {
		window, err = newPerPWindowWithOptions(options.BucketTime, options.BucketNums)
	} else {
		window, err = newWindowWithOptions(options.BucketTime, options.BucketNums)
	}
	if err != nil {
		return nil, err
	}

	breaker := &breaker{
		rw:       syncx.NewRWMutex(),
		metricer: window,
		now:      options.Now,
		state:    Closed,
	}

	breaker.options = Options{
		BucketTime:                options.BucketTime,
		BucketNums:                options.BucketNums,
		CoolingTimeout:            options.CoolingTimeout,
		DetectTimeout:             options.DetectTimeout,
		HalfOpenSuccesses:         options.HalfOpenSuccesses,
		ShouldTrip:                options.ShouldTrip,
		ShouldTripWithKey:         options.ShouldTripWithKey,
		BreakerStateChangeHandler: options.BreakerStateChangeHandler,
		Now:                       options.Now,
	}

	return breaker, nil
}

// Succeed records a success and decreases the concurrency counter by one
func (b *breaker) Succeed() {
	rwx := b.rw.RLocker()
	rwx.Lock()
	switch b.State() {
	case Open: // do nothing
		rwx.Unlock()
	case HalfOpen:
		rwx.Unlock()
		b.rw.Lock()
		// 双重检查 State，防止执行两次 BreakerStateChangeHandler
		if b.State() == HalfOpen {
			atomic.AddInt32(&b.halfopenSuccess, 1)
			if atomic.LoadInt32(&b.halfopenSuccess) >= b.options.HalfOpenSuccesses {
				if b.options.BreakerStateChangeHandler != nil {
					go b.options.BreakerStateChangeHandler(HalfOpen, Closed, b.metricer)
				}
				b.metricer.Reset()
				atomic.StoreInt32((*int32)(&b.state), int32(Closed))
			}
		}
		b.rw.Unlock()
	case Closed:
		b.metricer.Succeed()
		rwx.Unlock()
	}
}

func (b *breaker) error(isTimeout bool, trip TripFunc) {
	rwx := b.rw.RLocker()
	rwx.Lock()
	if isTimeout {
		b.metricer.Timeout()
	} else {
		b.metricer.Fail()
	}

	switch b.State() {
	case Open: // do nothing
		rwx.Unlock()
	case HalfOpen: // become Open
		rwx.Unlock()
		b.rw.Lock()
		// 双重检查 State，防止执行两次 BreakerStateChangeHandler
		if b.State() == HalfOpen {
			if b.options.BreakerStateChangeHandler != nil {
				go b.options.BreakerStateChangeHandler(HalfOpen, Open, b.metricer)
			}
			b.openTime = b.now()
			atomic.StoreInt32((*int32)(&b.state), int32(Open))
		}
		b.rw.Unlock()
	case Closed: // call ShouldTrip
		if trip != nil && trip(b.metricer) {
			rwx.Unlock()
			b.rw.Lock()
			if b.State() == Closed {
				// become Open and set the Open time
				if b.options.BreakerStateChangeHandler != nil {
					go b.options.BreakerStateChangeHandler(Closed, Open, b.metricer)
				}
				b.openTime = b.now()
				atomic.StoreInt32((*int32)(&b.state), int32(Open))
			}
			b.rw.Unlock()
		} else {
			rwx.Unlock()
		}
	}
}

// Fail records a failure and decreases the concurrency counter by one
func (b *breaker) Fail() {
	b.error(false, b.options.ShouldTrip)
}

// FailWithTrip .
func (b *breaker) FailWithTrip(trip TripFunc) {
	b.error(false, trip)
}

// Timeout records a timeout and decreases the concurrency counter by one
func (b *breaker) Timeout() {
	b.error(true, b.options.ShouldTrip)
}

// TimeoutWithTrip .
func (b *breaker) TimeoutWithTrip(trip TripFunc) {
	b.error(true, trip)
}

// IsAllowed .
func (b *breaker) IsAllowed() bool {
	return b.isAllowed()
}

// IsAllowed .
func (b *breaker) isAllowed() bool {
	rwx := b.rw.RLocker()
	rwx.Lock()
	switch b.State() {
	case Open:
		now := b.now()
		if b.openTime.Add(b.options.CoolingTimeout).After(now) {
			rwx.Unlock()
			return false
		}
		rwx.Unlock()
		b.rw.Lock()
		if b.State() == Open {
			// cooling timeout, then become HalfOpen
			if b.options.BreakerStateChangeHandler != nil {
				go b.options.BreakerStateChangeHandler(Open, HalfOpen, b.metricer)
			}
			atomic.StoreInt32((*int32)(&b.state), int32(HalfOpen))
			atomic.StoreInt32(&b.halfopenSuccess, 0)
			b.lastRetryTime = now
			b.rw.Unlock()
		} else {
			// other request has changed the state, so we reject current request
			b.rw.Unlock()
			return false
		}
	case HalfOpen:
		now := b.now()
		if b.lastRetryTime.Add(b.options.DetectTimeout).After(now) {
			rwx.Unlock()
			return false
		}
		rwx.Unlock()
		b.rw.Lock()
		if b.State() == HalfOpen {
			b.lastRetryTime = now
		} else if b.State() == Open { // callback may change the state to open
			b.rw.Unlock()
			return false
		}
		b.rw.Unlock()
	case Closed:
		rwx.Unlock()
	}

	return true
}

// State returns the breaker's State now
func (b *breaker) State() State {
	return State(atomic.LoadInt32((*int32)(&b.state)))
}

// Metricer returns the breaker's Metricer
func (b *breaker) Metricer() Metricer {
	return b.metricer
}

// Reset resets this breaker
func (b *breaker) Reset() {
	b.rw.Lock()
	b.metricer.Reset()
	atomic.StoreInt32((*int32)(&b.state), int32(Closed))
	// don't change concurrency counter anyway
	b.rw.Unlock()
}
