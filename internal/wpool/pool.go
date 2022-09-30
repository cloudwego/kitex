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
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
)

// Task is the function that the worker will execute.
type Task func()

// Pool is a worker pool bind with some idle goroutines.
type Pool struct {
	size  int32
	tasks chan Task

	// maxIdle is the number of the max idle workers in the pool.
	// if maxIdle too small, the pool works like a native 'go func()'.
	maxIdle int32
	// maxIdleTime is the max idle time that the worker will wait for the new task.
	maxIdleTime time.Duration
}

// New creates a new worker pool.
func New(maxIdle int, maxIdleTime time.Duration) *Pool {
	return &Pool{
		tasks:       make(chan Task),
		maxIdle:     int32(maxIdle),
		maxIdleTime: maxIdleTime,
	}
}

// Size returns the number of the running workers.
func (p *Pool) Size() int32 {
	return atomic.LoadInt32(&p.size)
}

// Go creates/reuses a worker to run task.
func (p *Pool) Go(task Task) {
	select {
	case p.tasks <- task:
		// reuse exist worker
		return
	default:
	}

	// create new worker
	atomic.AddInt32(&p.size, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				klog.Errorf("panic in wpool: error=%v: stack=%s", r, debug.Stack())
			}
			atomic.AddInt32(&p.size, -1)
		}()
		task()
		if atomic.LoadInt32(&p.size) > p.maxIdle {
			return
		}

		// waiting for new task
		idleTimer := time.NewTimer(p.maxIdleTime)
		for {
			select {
			case task = <-p.tasks:
				task()
			case <-idleTimer.C:
				// worker exits
				return
			}

			if !idleTimer.Stop() {
				<-idleTimer.C
			}
			idleTimer.Reset(p.maxIdleTime)
		}
	}()
}
