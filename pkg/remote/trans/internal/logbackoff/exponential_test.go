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
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestExponentialObserve(t *testing.T) {
	var backoff Exponential
	now := time.Unix(0, 0)

	count, elapsed, allow := backoff.Observe(now)
	test.Assert(t, allow)
	test.Assert(t, count == 1, count)
	test.Assert(t, elapsed == 0, elapsed)

	count, elapsed, allow = backoff.Observe(now.Add(initialInterval - time.Nanosecond))
	test.Assert(t, !allow)
	test.Assert(t, count == 0, count)
	test.Assert(t, elapsed == 0, elapsed)

	now = now.Add(initialInterval)
	count, elapsed, allow = backoff.Observe(now)
	test.Assert(t, allow)
	test.Assert(t, count == 2, count)
	test.Assert(t, elapsed == initialInterval, elapsed)

	count, elapsed, allow = backoff.Observe(now.Add(2*time.Second - time.Nanosecond))
	test.Assert(t, !allow)
	test.Assert(t, count == 0, count)
	test.Assert(t, elapsed == 0, elapsed)

	now = now.Add(2 * time.Second)
	count, elapsed, allow = backoff.Observe(now)
	test.Assert(t, allow)
	test.Assert(t, count == 2, count)
	test.Assert(t, elapsed == 2*time.Second, elapsed)
	for interval := 4 * time.Second; interval <= maxInterval; interval *= 2 {
		now = now.Add(interval)
		count, elapsed, allow = backoff.Observe(now)
		test.Assert(t, allow)
		test.Assert(t, count == 1, count)
		test.Assert(t, elapsed == interval, elapsed)
	}

	count, elapsed, allow = backoff.Observe(now.Add(maxInterval - time.Nanosecond))
	test.Assert(t, !allow)
	test.Assert(t, count == 0, count)
	test.Assert(t, elapsed == 0, elapsed)
	now = now.Add(maxInterval)
	count, elapsed, allow = backoff.Observe(now)
	test.Assert(t, allow)
	test.Assert(t, count == 2, count)
	test.Assert(t, elapsed == maxInterval, elapsed)

	now = now.Add(resetAfter)
	count, elapsed, allow = backoff.Observe(now)
	test.Assert(t, allow)
	test.Assert(t, count == 1, count)
	test.Assert(t, elapsed == resetAfter, elapsed)

	count, elapsed, allow = backoff.Observe(now.Add(initialInterval - time.Nanosecond))
	test.Assert(t, !allow)
	test.Assert(t, count == 0, count)
	test.Assert(t, elapsed == 0, elapsed)
}
