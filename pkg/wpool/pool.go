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

func (p *Pool) Size() int32 {
	return atomic.LoadInt32(&p.size)
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
