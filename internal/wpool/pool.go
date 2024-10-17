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

/* Difference between wpool and gopool(github.com/bytedance/gopkg/util/gopool):
- wpool is a goroutine pool with high reuse rate. The old goroutine will block to wait for new tasks coming.
- gopool sometimes will have a very low reuse rate. The old goroutine will not block if no new task coming.
*/

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/endpoint"
)

// Pool is a worker pool for task with timeout
type Pool struct {
	size  int32
	tasks chan *task

	// maxIdle is the number of the max idle workers in the pool.
	// if maxIdle too small, the pool works like a native 'go func()'.
	maxIdle int32
	// maxIdleTime is the max idle time that the worker will wait for the new task.
	maxIdleTime time.Duration

	ticker chan struct{}
}

// New creates a new worker pool.
func New(maxIdle int, maxIdleTime time.Duration) *Pool {
	return &Pool{
		tasks:       make(chan *task),
		maxIdle:     int32(maxIdle),
		maxIdleTime: maxIdleTime,
	}
}

// Size returns the number of the running workers.
func (p *Pool) Size() int32 {
	return atomic.LoadInt32(&p.size)
}

func (p *Pool) createTicker() {
	// make sure previous goroutine will be closed before creating a new one
	if p.ticker != nil {
		close(p.ticker)
	}
	ch := make(chan struct{})
	p.ticker = ch

	go func() {
		// if maxIdleTime=60s, maxIdle=100
		// it sends noop task every 60ms
		// but always d >= 10*time.Millisecond
		// this may cause goroutines take more time to exit which is acceptable.
		d := p.maxIdleTime / time.Duration(p.maxIdle) / 10
		if d < 10*time.Millisecond {
			d = 10 * time.Millisecond
		}
		tk := time.NewTicker(d)
		for p.Size() > 0 {
			select {
			case <-tk.C:
			case <-ch:
				return
			}
			select {
			case p.tasks <- nil: // noop task for checking idletime
			case <-tk.C:
			}
		}
	}()
}

func (p *Pool) createWorker(t *task) bool {
	if n := atomic.AddInt32(&p.size, 1); n < p.maxIdle {
		if n == 1 {
			p.createTicker()
		}
		go func(t *task) {
			defer atomic.AddInt32(&p.size, -1)

			t.Run()

			lastactive := time.Now()
			for t := range p.tasks {
				if t == nil { // from `createTicker` func
					if time.Since(lastactive) > p.maxIdleTime {
						break
					}
					continue
				}
				t.Run()
				lastactive = time.Now()
			}
		}(t)
		return true
	} else {
		atomic.AddInt32(&p.size, -1)
		return false
	}
}

// RunTask creates/reuses a worker to run task.
func (p *Pool) RunTask(ctx context.Context, timeout time.Duration,
	req, resp any, ep endpoint.Endpoint,
) (context.Context, error) {
	t := newTask(ctx, timeout, req, resp, ep)
	select {
	case p.tasks <- t:
		return t.Wait()
	default:
	}
	if !p.createWorker(t) {
		// if created worker, t.Run() will be called in worker goroutine
		// if NOT, we should go t.Run() here.
		go t.Run()
	}
	return t.Wait()
}
