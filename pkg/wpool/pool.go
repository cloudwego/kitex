package wpool

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/util/logger"
)

type Task func()

type Pool struct {
	size  int32
	tasks chan Task

	// settings
	maxIdle     int32
	maxIdleTime time.Duration
}

func New(maxIdle int, maxIdleTime time.Duration) *Pool {
	return &Pool{
		tasks:       make(chan Task),
		maxIdle:     int32(maxIdle),
		maxIdleTime: maxIdleTime,
	}
}

func (p *Pool) GoCtx(ctx context.Context, task Task) {
	select {
	case p.tasks <- task:
		// reuse exist goroutine
		return
	default:
	}

	// create new goroutine
	atomic.AddInt32(&p.size, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("panic in wpool: %v: %s", r, debug.Stack())
				logger.CtxErrorf(ctx, msg)
			}
			atomic.AddInt32(&p.size, -1)
		}()

		deadline := time.Now().Add(p.maxIdleTime)
		task()

		if atomic.LoadInt32(&p.size) > p.maxIdle {
			return
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
		for {
			select {
			case task = <-p.tasks:
				task()
			case <-ctx.Done():
				return
			}
		}
	}()
}
