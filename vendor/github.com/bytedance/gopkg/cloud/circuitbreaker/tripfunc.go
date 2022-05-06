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

// TripFunc is a function called by a breaker when error appear and
// determines whether the breaker should trip.
type TripFunc func(Metricer) bool

// TripFuncWithKey returns a TripFunc according to the key.
type TripFuncWithKey func(string) TripFunc

// ThresholdTripFunc .
func ThresholdTripFunc(threshold int64) TripFunc {
	return func(m Metricer) bool {
		return m.Failures()+m.Timeouts() >= threshold
	}
}

// ConsecutiveTripFunc .
func ConsecutiveTripFunc(threshold int64) TripFunc {
	return func(m Metricer) bool {
		return m.ConseErrors() >= threshold
	}
}

// RateTripFunc .
func RateTripFunc(rate float64, minSamples int64) TripFunc {
	return func(m Metricer) bool {
		samples := m.Samples()
		return samples >= minSamples && m.ErrorRate() >= rate
	}
}

// ConsecutiveTripFuncV2 uses the following three strategies based on the parameters passed in.
// 1. when the number of samples >= minSamples and the error rate >= rate
// 2. when the number of samples >= durationSamples and the length of consecutive errors >= duration
// 3. When the number of consecutive errors >= conseErrors
// The fuse is opened when any of the above three strategies holds.
func ConsecutiveTripFuncV2(rate float64, minSamples int64, duration time.Duration, durationSamples, conseErrors int64) TripFunc {
	return func(m Metricer) bool {
		samples := m.Samples()
		// based on stat
		if samples >= minSamples && m.ErrorRate() >= rate {
			return true
		}
		// based on continuous time
		if duration > 0 && m.ConseErrors() >= durationSamples && m.ConseTime() >= duration {
			return true
		}
		// base on consecutive errors
		if conseErrors > 0 && m.ConseErrors() >= conseErrors {
			return true
		}
		return false
	}
}
