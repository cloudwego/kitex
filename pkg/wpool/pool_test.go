package wpool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestWPool(t *testing.T) {
	p := New(1, time.Second)
	var (
		sum  int32
		wg   sync.WaitGroup
		size = 10
	)
	test.Assert(t, p.size == 0)
	for i := 0; i < size; i++ {
		wg.Add(1)
		p.GoCtx(context.Background(), func() {
			defer wg.Done()
			atomic.AddInt32(&sum, 1)
		})
	}
	test.Assert(t, p.size != 0)

	wg.Wait()
	test.Assert(t, sum == int32(size))
	test.Assert(t, p.size == 1)
}
