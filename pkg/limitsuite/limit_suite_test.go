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

package limitsuite

import (
	"context"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

var _ Limiter = (*nilRequestLimiter)(nil)

type nilRequestLimiter struct{}

func (m nilRequestLimiter) Allow(ctx context.Context, request, response interface{}) bool {
	return request != nil
}

func (m nilRequestLimiter) Close() {}

func TestLimitMWBuilder(t *testing.T) {
	t.Run("allow_for_non_nil_request", func(t *testing.T) {
		limiter := &nilRequestLimiter{}
		mw := limitMWBuilder(limiter)
		ep := mw(func(ctx context.Context, req, resp interface{}) (err error) { return nil })
		test.AssertNil(t, ep(context.Background(), 1, nil), "non-nil-request#1")
	})

	t.Run("reject_for_nil_request", func(t *testing.T) {
		limiter := &nilRequestLimiter{}
		mw := limitMWBuilder(limiter)
		ep := mw(func(ctx context.Context, req, resp interface{}) (err error) { return nil })
		test.AssertNonNil(t, ep(context.Background(), nil, nil), "nil-request#1")
	})
}
