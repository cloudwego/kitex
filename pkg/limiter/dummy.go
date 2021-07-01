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

import "time"

// DummyConcurrencyLimiter implements ConcurrencyLimiter but without actual limitation.
type DummyConcurrencyLimiter struct{}

// Acquire .
func (dcl *DummyConcurrencyLimiter) Acquire() bool { return true }

// Release .
func (dcl *DummyConcurrencyLimiter) Release() {}

// Status .
func (dcl *DummyConcurrencyLimiter) Status() (limit, occupied int) { return }

// DummyRateLimiter implements RateLimiter but without actual limitation.
type DummyRateLimiter struct{}

// Acquire .
func (drl *DummyRateLimiter) Acquire() bool { return true }

// Status .
func (drl *DummyRateLimiter) Status() (max, current int, interval time.Duration) { return }
