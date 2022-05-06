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

import "time"

// State changes between Closed, Open, HalfOpen
// [Closed] -->- tripped ----> [Open]<-------+
//    ^                          |           ^
//    |                          v           |
//    +                          |      detect fail
//    |                          |           |
//    |                    cooling timeout   |
//    ^                          |           ^
//    |                          v           |
//    +--- detect succeed --<-[HalfOpen]-->--+
//
// The behaviors of each states:
// =================================================================================================
// |           | [Succeed]                  | [Fail or Timeout]       | [IsAllowed]                |
// |================================================================================================
// | [Closed]  | do nothing                 | if tripped, become Open | allow                      |
// |================================================================================================
// | [Open]    | do nothing                 | do nothing              | if cooling timeout, allow; |
// |           |                            |                         | else reject                |
// |================================================================================================
// |           |increase halfopenSuccess,   |                         | if detect timeout, allow;  |
// |[HalfOpen] |if(halfopenSuccess >=       | become Open             | else reject                |
// |           | defaultHalfOpenSuccesses)|                         |                            |
// |           |     become Closed          |                         |                            |
// =================================================================================================
type State int32

func (s State) String() string {
	switch s {
	case Open:
		return "OPEN"
	case HalfOpen:
		return "HALFOPEN"
	case Closed:
		return "CLOSED"
	}
	return "INVALID"
}

// represents the state
const (
	Open     State = iota
	HalfOpen State = iota
	Closed   State = iota
)

// BreakerStateChangeHandler .
type BreakerStateChangeHandler func(oldState, newState State, m Metricer)

// PanelStateChangeHandler .
type PanelStateChangeHandler func(key string, oldState, newState State, m Metricer)

// Options for breaker
type Options struct {
	// parameters for metricser
	BucketTime time.Duration // the time each bucket holds
	BucketNums int32         // the number of buckets the breaker have

	// parameters for breaker
	CoolingTimeout    time.Duration // fixed when create
	DetectTimeout     time.Duration // fixed when create
	HalfOpenSuccesses int32         // halfopen success is the threshold when the breaker is in HalfOpen;
	// after exceeding consecutively this times, it will change its State from HalfOpen to Closed;

	ShouldTrip                TripFunc                  // can be nil
	ShouldTripWithKey         TripFuncWithKey           // can be nil, overwrites ShouldTrip
	BreakerStateChangeHandler BreakerStateChangeHandler // can be nil

	// if to use Per-P Metricer
	// use Per-P Metricer can increase performance in multi-P condition
	// but will consume more memory
	EnableShardP bool

	// Default value is time.Now, caller may use some high-performance custom time now func here
	Now func() time.Time
}

const (
	// bucket time is the time each bucket holds
	defaultBucketTime = time.Millisecond * 100

	// bucket nums is the number of buckets the metricser has;
	// the more buckets you have, the less counters you lose when
	// the oldest bucket expire;
	defaultBucketNums = 100

	// default window size is (defaultBucketTime * defaultBucketNums),
	// which is 10 seconds;
)
