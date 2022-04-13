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

package bound

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/kitex/pkg/remote"

	limiterMocks "github.com/cloudwego/kitex/internal/mocks/limiter"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/invoke"
)

var errFoo = errors.New("mockError")

// TestNewServerLimiterHandler test NewServerLimiterHandler function and assert result to be not nil.
func TestNewServerLimiterHandler(t *testing.T) {
	concurrencyLimiter := &limiterMocks.ConcurrencyLimiter{}
	rateLimiter := &limiterMocks.RateLimiter{}
	limitReporter := &limiterMocks.LimitReporter{}

	handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
	test.Assert(t, handler != nil)
}

// TestLimiterOnActive test OnActive function of serverLimiterHandler, to assert the concurrencyLimiter 'Acquire' works.
// If not acquired, the message would be rejected with ErrConnOverLimit error.
func TestLimiterOnActive(t *testing.T) {
	t.Run("Test OnActive with limit acquired", func(t *testing.T) {
		concurrencyLimiter := &limiterMocks.ConcurrencyLimiter{}
		rateLimiter := &limiterMocks.RateLimiter{}
		limitReporter := &limiterMocks.LimitReporter{}
		ctx := context.Background()

		concurrencyLimiter.On("Acquire").Return(true)

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnActive(ctx, invoke.NewMessage(nil, nil))

		test.Assert(t, ctx != nil)
		test.Assert(t, err == nil)
	})

	t.Run("Test OnActive with limit acquire false and nil reporter", func(t *testing.T) {
		concurrencyLimiter := &limiterMocks.ConcurrencyLimiter{}
		rateLimiter := &limiterMocks.RateLimiter{}
		ctx := context.Background()

		concurrencyLimiter.On("Acquire").Return(false)

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, nil)
		ctx, err := handler.OnActive(ctx, invoke.NewMessage(nil, nil))

		test.Assert(t, ctx != nil)
		test.Assert(t, errors.Is(kerrors.ErrConnOverLimit, err))
	})

	t.Run("Test OnActive with limit acquire false and non-nil reporter", func(t *testing.T) {
		concurrencyLimiter := &limiterMocks.ConcurrencyLimiter{}
		rateLimiter := &limiterMocks.RateLimiter{}
		limitReporter := &limiterMocks.LimitReporter{}
		ctx := context.Background()

		concurrencyLimiter.On("Acquire").Return(false)
		limitReporter.On("ConnOverloadReport")

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnActive(ctx, invoke.NewMessage(nil, nil))

		test.Assert(t, ctx != nil)
		test.Assert(t, err != nil)
		test.Assert(t, errors.Is(kerrors.ErrConnOverLimit, err))
	})
}

// TestLimiterOnRead test OnRead function of serverLimiterHandler, to assert the rateLimiter 'Acquire' works.
// If not acquired, the message would be rejected with ErrQPSOverLimit error.
func TestLimiterOnRead(t *testing.T) {
	t.Run("Test OnRead with limit acquired", func(t *testing.T) {
		concurrencyLimiter := &limiterMocks.ConcurrencyLimiter{}
		rateLimiter := &limiterMocks.RateLimiter{}
		limitReporter := &limiterMocks.LimitReporter{}
		ctx := context.Background()

		rateLimiter.On("Acquire").Return(true)

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnRead(ctx, invoke.NewMessage(nil, nil))

		test.Assert(t, ctx != nil)
		test.Assert(t, err == nil)
	})

	t.Run("Test OnActive with limit acquire false and nil reporter", func(t *testing.T) {
		concurrencyLimiter := &limiterMocks.ConcurrencyLimiter{}
		rateLimiter := &limiterMocks.RateLimiter{}
		ctx := context.Background()

		rateLimiter.On("Acquire").Return(false)

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, nil)
		ctx, err := handler.OnRead(ctx, invoke.NewMessage(nil, nil))

		test.Assert(t, ctx != nil)
		test.Assert(t, errors.Is(kerrors.ErrQPSOverLimit, err))
	})

	t.Run("Test OnActive with limit acquire false and non-nil reporter", func(t *testing.T) {
		concurrencyLimiter := &limiterMocks.ConcurrencyLimiter{}
		rateLimiter := &limiterMocks.RateLimiter{}
		limitReporter := &limiterMocks.LimitReporter{}
		ctx := context.Background()

		rateLimiter.On("Acquire").Return(false)
		limitReporter.On("QPSOverloadReport")

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnRead(ctx, invoke.NewMessage(nil, nil))

		test.Assert(t, ctx != nil)
		test.Assert(t, err != nil)
		test.Assert(t, errors.Is(kerrors.ErrQPSOverLimit, err))
	})
}

// TestLimiterOnInactive test OnInactive function of serverLimiterHandler, to assert the release is called.
func TestLimiterOnInactive(t *testing.T) {
	t.Run("Test OnInactive", func(t *testing.T) {
		concurrencyLimiter := &limiterMocks.ConcurrencyLimiter{}
		rateLimiter := &limiterMocks.RateLimiter{}
		limitReporter := &limiterMocks.LimitReporter{}
		ctx := context.Background()

		concurrencyLimiter.On("Release")

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx = handler.OnInactive(ctx, invoke.NewMessage(nil, nil))

		test.Assert(t, ctx != nil)
	})
}

// TestLimiterOnMessage test OnMessage function of serverLimiterHandler
func TestLimiterOnMessage(t *testing.T) {
	t.Run("Test OnMessage", func(t *testing.T) {
		concurrencyLimiter := &limiterMocks.ConcurrencyLimiter{}
		rateLimiter := &limiterMocks.RateLimiter{}
		limitReporter := &limiterMocks.LimitReporter{}
		ctx := context.Background()

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnMessage(ctx, remote.NewMessage(nil, nil, nil, remote.Call, remote.Client),
			remote.NewMessage(nil, nil, nil, remote.Reply, remote.Client))

		test.Assert(t, ctx != nil)
		test.Assert(t, err == nil)
	})
}
