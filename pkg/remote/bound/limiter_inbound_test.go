package bound

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/pkg/remote"
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/limiter/mocks"
	"github.com/cloudwego/kitex/pkg/remote/trans/invoke"
	"github.com/stretchr/testify/assert"
)

var mockErr = errors.New("mockError")

func TestNewServerLimiterHandler(t *testing.T) {
	concurrencyLimiter := &mocks.ConcurrencyLimiter{}
	rateLimiter := &mocks.RateLimiter{}
	limitReporter := &mocks.LimitReporter{}

	handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
	assert.NotNil(t, handler)
}

func TestLimiterOnActive(t *testing.T) {
	t.Run("Test OnActive with limit acquired", func(t *testing.T) {
		concurrencyLimiter := &mocks.ConcurrencyLimiter{}
		rateLimiter := &mocks.RateLimiter{}
		limitReporter := &mocks.LimitReporter{}
		ctx := context.Background()

		concurrencyLimiter.On("Acquire").Return(true)

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnActive(ctx, invoke.NewMessage(nil, nil))

		assert.NotNil(t, ctx)
		assert.Nil(t, err)
	})

	t.Run("Test OnActive with nil reporter", func(t *testing.T) {
		concurrencyLimiter := &mocks.ConcurrencyLimiter{}
		rateLimiter := &mocks.RateLimiter{}
		ctx := context.Background()

		concurrencyLimiter.On("Acquire").Return(false)

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, nil)
		ctx, err := handler.OnActive(ctx, invoke.NewMessage(nil, nil))

		assert.NotNil(t, ctx)
		assert.Equal(t, kerrors.ErrConnOverLimit, err)
	})

	t.Run("Test OnActive with reporter", func(t *testing.T) {
		concurrencyLimiter := &mocks.ConcurrencyLimiter{}
		rateLimiter := &mocks.RateLimiter{}
		limitReporter := &mocks.LimitReporter{}
		ctx := context.Background()

		concurrencyLimiter.On("Acquire").Return(false)
		limitReporter.On("ConnOverloadReport")

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnActive(ctx, invoke.NewMessage(nil, nil))

		assert.NotNil(t, ctx)
		assert.NotNil(t, err)
		assert.Equal(t, kerrors.ErrConnOverLimit, err)
	})
}

func TestLimiterOnRead(t *testing.T) {
	t.Run("Test OnRead with limit acquired", func(t *testing.T) {
		concurrencyLimiter := &mocks.ConcurrencyLimiter{}
		rateLimiter := &mocks.RateLimiter{}
		limitReporter := &mocks.LimitReporter{}
		ctx := context.Background()

		rateLimiter.On("Acquire").Return(true)

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnRead(ctx, invoke.NewMessage(nil, nil))

		assert.NotNil(t, ctx)
		assert.Nil(t, err)
	})

	t.Run("Test OnRead with nil reporter", func(t *testing.T) {
		concurrencyLimiter := &mocks.ConcurrencyLimiter{}
		rateLimiter := &mocks.RateLimiter{}
		ctx := context.Background()

		rateLimiter.On("Acquire").Return(false)

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, nil)
		ctx, err := handler.OnRead(ctx, invoke.NewMessage(nil, nil))

		assert.NotNil(t, ctx)
		assert.Equal(t, kerrors.ErrQPSOverLimit, err)
	})

	t.Run("Test OnRead with reporter", func(t *testing.T) {
		concurrencyLimiter := &mocks.ConcurrencyLimiter{}
		rateLimiter := &mocks.RateLimiter{}
		limitReporter := &mocks.LimitReporter{}
		ctx := context.Background()

		rateLimiter.On("Acquire").Return(false)
		limitReporter.On("QPSOverloadReport")

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnRead(ctx, invoke.NewMessage(nil, nil))

		assert.NotNil(t, ctx)
		assert.NotNil(t, err)
		assert.Equal(t, kerrors.ErrQPSOverLimit, err)
	})
}

func TestLimiterOnInactive(t *testing.T) {
	t.Run("Test OnInactive", func(t *testing.T) {
		concurrencyLimiter := &mocks.ConcurrencyLimiter{}
		rateLimiter := &mocks.RateLimiter{}
		limitReporter := &mocks.LimitReporter{}
		ctx := context.Background()

		concurrencyLimiter.On("Release")

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx = handler.OnInactive(ctx, invoke.NewMessage(nil, nil))

		assert.NotNil(t, ctx)
	})
}

func TestLimiterOnMessage(t *testing.T) {
	t.Run("Test OnMessage", func(t *testing.T) {
		concurrencyLimiter := &mocks.ConcurrencyLimiter{}
		rateLimiter := &mocks.RateLimiter{}
		limitReporter := &mocks.LimitReporter{}
		ctx := context.Background()

		handler := NewServerLimiterHandler(concurrencyLimiter, rateLimiter, limitReporter)
		ctx, err := handler.OnMessage(ctx, remote.NewMessage(nil, nil, nil, remote.Call, remote.Client),
			remote.NewMessage(nil, nil, nil, remote.Reply, remote.Client))

		assert.NotNil(t, ctx)
		assert.Nil(t, err)
	})
}
