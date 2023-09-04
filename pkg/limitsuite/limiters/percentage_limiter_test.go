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
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

var (
	mockRetryKey = "retry"
	mockFirstCtx = context.WithValue(context.Background(), mockRetryKey, false)
	mockRetryCtx = context.WithValue(context.Background(), mockRetryKey, true)
)

func mockIsRetry(ctx context.Context, request, response interface{}) bool {
	return ctx.Value(mockRetryKey).(bool)
}

// disables reset
func withoutInterval() PercentageLimiterOption {
	return func(l *PercentageLimiter) {
		l.interval = -1
	}
}

func TestPercentageLimiter_Allow(t *testing.T) {
	t.Run("allow:!l.isLimitCandidate(ctx,request,response)", func(t *testing.T) {
		l := NewPercentageLimiter(0.01, mockIsRetry, WithMinSampleCount(0), withoutInterval())
		test.AssertEqual(t, true, l.Allow(mockFirstCtx, nil, nil), "not-limit-candidate")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(0), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")
	})

	t.Run("allow:total<=int64(l.minSampleCount)", func(t *testing.T) {
		l := NewPercentageLimiter(0.01, mockIsRetry, WithMinSampleCount(1), withoutInterval())
		test.AssertEqual(t, true, l.Allow(mockRetryCtx, nil, nil), "<minSampleCount")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")
	})

	t.Run("allow:!l.exceedPercentage(allowed,total)", func(t *testing.T) {
		l := NewPercentageLimiter(0.51, mockIsRetry, WithMinSampleCount(0), withoutInterval())
		test.AssertEqual(t, true, l.Allow(mockFirstCtx, nil, nil), "not-limit-candidate")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(0), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")

		test.AssertEqual(t, true, l.Allow(mockRetryCtx, nil, nil), "<percentage")
		test.AssertEqual(t, int64(2), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")
	})

	t.Run("reject:!l.exceedPercentage(allowed,total)", func(t *testing.T) {
		l := NewPercentageLimiter(0.49, mockIsRetry, WithMinSampleCount(0), withoutInterval())
		test.AssertEqual(t, true, l.Allow(mockFirstCtx, nil, nil), "not-limit-candidate")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(0), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")

		test.AssertEqual(t, false, l.Allow(mockRetryCtx, nil, nil), ">percentage")
		test.AssertEqual(t, int64(2), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(0), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")
	})

	t.Run("allow-second-since-less-than-percentage", func(t *testing.T) {
		l := NewPercentageLimiter(0.51, mockIsRetry, WithMinSampleCount(0), withoutInterval())
		test.AssertEqual(t, false, l.Allow(mockRetryCtx, nil, nil), "retry1") // 1/1, reject
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(0), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")

		test.AssertEqual(t, true, l.Allow(mockRetryCtx, nil, nil), "retry2") // 1/2, allow
		test.AssertEqual(t, int64(2), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")
	})

	t.Run("reject retry, after reset allow retry", func(t *testing.T) {
		l := NewPercentageLimiter(0.51, mockIsRetry, WithMinSampleCount(1), withoutInterval())
		test.AssertEqual(t, true, l.Allow(mockRetryCtx, nil, nil), "retry1") // <=minSample, allow
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")

		test.AssertEqual(t, false, l.Allow(mockRetryCtx, nil, nil), "retry2") // > minSample, reject
		test.AssertEqual(t, int64(2), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")

		l.Reset()

		test.AssertEqual(t, true, l.Allow(mockRetryCtx, nil, nil), "retry3")  // <=minSample, allow
		test.AssertEqual(t, false, l.Allow(mockRetryCtx, nil, nil), "retry4") // > minSample, reject
		test.AssertEqual(t, int64(2), atomic.LoadInt64(&l.totalCount), "totalCount")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount")
	})

	t.Run("close-no-reset", func(t *testing.T) {
		l := NewPercentageLimiter(0.51, mockIsRetry, WithMinSampleCount(1), WithInterval(time.Millisecond*20))
		test.AssertEqual(t, true, l.Allow(mockRetryCtx, nil, nil), "retry1")  // <=minSample, allow
		test.AssertEqual(t, false, l.Allow(mockRetryCtx, nil, nil), "retry2") // > minSample, reject
		test.AssertEqual(t, int64(2), atomic.LoadInt64(&l.totalCount), "totalCount#1")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount#1")

		time.Sleep(time.Millisecond * 35) // make sure l.Reset() is called
		test.AssertEqual(t, int64(0), atomic.LoadInt64(&l.totalCount), "totalCount#2")
		test.AssertEqual(t, int64(0), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount#2")
		l.Close()
		time.Sleep(time.Millisecond * 35) // make sure l stopped

		test.AssertEqual(t, true, l.Allow(mockRetryCtx, nil, nil), "retry1")  // <=minSample, allow
		test.AssertEqual(t, false, l.Allow(mockRetryCtx, nil, nil), "retry2") // > minSample, reject

		time.Sleep(time.Millisecond * 35) // make sure if there's a bug, l.Reset() is called
		test.AssertEqual(t, int64(2), atomic.LoadInt64(&l.totalCount), "totalCount#3")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount#3")

		test.AssertEqual(t, false, l.Allow(mockRetryCtx, nil, nil), "retry3") // >minSample, reject
		test.AssertEqual(t, int64(3), atomic.LoadInt64(&l.totalCount), "totalCount#4")
		test.AssertEqual(t, int64(1), atomic.LoadInt64(&l.allowedCandidatesCount), "allowedCandidatesCount#4") // 2/3? reject -> 1/3
	})
}

func TestPercentageLimiter_exceedPercentage(t *testing.T) {
	tests := []struct {
		name         string
		percentage   int64
		allowedCount int64
		totalCount   int64
		want         bool
	}{
		{
			name:         "0/1>1%: false",
			allowedCount: 0,
			totalCount:   1,
			percentage:   1,
			want:         false,
		},
		{
			name:         "1/99>1%: true",
			allowedCount: 1,
			totalCount:   99,
			percentage:   1,
			want:         true,
		},
		{
			name:         "1/101>1%: false",
			allowedCount: 1,
			totalCount:   101,
			percentage:   1,
			want:         false,
		},
		{
			name:         "2/3>66%: true",
			allowedCount: 2,
			totalCount:   3,
			percentage:   66,
			want:         true,
		},
		{
			name:         "2/3>67%: false",
			allowedCount: 2,
			totalCount:   3,
			percentage:   67,
			want:         false,
		},
		{
			name:         "49/99>50%: false",
			allowedCount: 49,
			totalCount:   99,
			percentage:   50,
			want:         false,
		},
		{
			name:         "50/99>50%: true",
			allowedCount: 50,
			totalCount:   99,
			percentage:   50,
			want:         true,
		},
		{
			name:         "49/100>50%: false",
			allowedCount: 49,
			totalCount:   100,
			percentage:   50,
			want:         false,
		},
		{
			name:         "50/100>50%: false",
			allowedCount: 50,
			totalCount:   100,
			percentage:   50,
			want:         false,
		},
		{
			name:         "51/100>50%: true",
			allowedCount: 51,
			totalCount:   100,
			percentage:   50,
			want:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &PercentageLimiter{
				percentage: tt.percentage * 100,
			}
			if got := l.exceedPercentage(tt.allowedCount, tt.totalCount); got != tt.want {
				t.Errorf("exceedPercentage() = %v, want %v", got, tt.want)
			}
		})
	}
}
