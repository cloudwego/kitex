package singleflightutil

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestCheckAndDo(t *testing.T) {
	// initial check succeed
	t.Run("CacheHit_ShouldNotCallFn", func(t *testing.T) {
		g := &Group{}
		cache := sync.Map{}
		key, expectedValue := "key", "cached"
		cache.Store(key, expectedValue)

		var fnCalled bool
		fn := func() (any, error) {
			fnCalled = true
			return "new", nil
		}

		checkFunc := func() (any, bool) {
			return cache.Load(key)
		}

		val, err, shared := g.CheckAndDo(key, checkFunc, fn)
		test.Assert(t, err == nil)
		test.Assert(t, shared)
		test.Assert(t, val == expectedValue)
		test.Assert(t, !fnCalled)
	})

	// concurrent
	t.Run("ConcurrentCalls", func(t *testing.T) {
		g := &Group{}
		cache := sync.Map{}
		var fnCalls int32
		key, expectedValue := "key", int32(1)
		fn := func() (any, error) {
			curr := atomic.AddInt32(&fnCalls, 1)
			cache.Store(key, curr)
			return curr, nil
		}
		checkFunc := func() (any, bool) {
			return cache.Load(key)
		}

		var wg sync.WaitGroup
		const numGoroutines = 100
		results := make(chan int32, numGoroutines)

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				val, err, _ := g.CheckAndDo(key, checkFunc, fn)
				if err != nil {
					panic(err)
				}
				results <- val.(int32)
			}()
		}

		wg.Wait()
		close(results)

		test.Assert(t, atomic.LoadInt32(&fnCalls) == 1, fmt.Sprintf("fn should be called only once, actual=%d", atomic.LoadInt32(&fnCalls)))
		for val := range results {
			test.Assert(t, val == expectedValue, fmt.Sprintf("result incorrect, expected=%v, actual=%v", expectedValue, val))
		}
	})
}
