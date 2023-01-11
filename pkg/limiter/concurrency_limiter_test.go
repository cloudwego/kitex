package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestNonBlockingConcurrencyLimiter(t *testing.T) {
	var wg sync.WaitGroup
	limit := 100
	limiter := NewConcurrencyLimiter(limit, false)
	ctx := context.Background()
	for i := 0; i < limit; i++ {
		wg.Add(1)
		acquired := limiter.Acquire(ctx)
		test.Assert(t, acquired)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond * 100)
			limiter.Release(ctx)
		}()
	}
	for i := 0; i < limit; i++ {
		acquired := limiter.Acquire(ctx)
		test.Assert(t, !acquired)
	}
	wg.Wait()
	for i := 0; i < limit; i++ {
		acquired := limiter.Acquire(ctx)
		test.Assert(t, acquired)
		lmt, ift := limiter.Status(ctx)
		test.Assert(t, lmt == limit, ift == i+1)
	}
	acquired := limiter.Acquire(ctx)
	test.Assert(t, !acquired)
}

func TestBlockingConcurrencyLimiter(t *testing.T) {
	limit := 100
	limiter := NewConcurrencyLimiter(limit, true)
	ctx := context.Background()
	var total int32
	for i := 0; i < limit; i++ {
		acquired := limiter.Acquire(ctx)
		test.Assert(t, acquired)
		go func() {
			time.Sleep(time.Millisecond * 100)
			atomic.AddInt32(&total, 1)
			limiter.Release(ctx)
		}()
	}
	for i := 0; i < limit; i++ {
		acquired := limiter.Acquire(ctx)
		test.Assert(t, acquired)
		lmt, ift := limiter.Status(ctx)
		test.Assert(t, lmt == limit, ift == i+1)
	}
	test.Assert(t, atomic.LoadInt32(&total) == int32(limit))
}
