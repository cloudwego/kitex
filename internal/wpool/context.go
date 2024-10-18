/*
 * Copyright 2024 CloudWeGo Authors
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
	"errors"
	"sync"
	"time"
)

type taskContext struct {
	context.Context

	dl time.Time
	ch chan struct{}

	mu  sync.Mutex
	err error
}

func newTaskContext(ctx context.Context, timeout time.Duration) *taskContext {
	ret := &taskContext{Context: ctx, ch: make(chan struct{})}
	deadline, ok := ctx.Deadline()
	if ok {
		ret.dl = deadline
	}
	if timeout != 0 {
		dl := time.Now().Add(timeout)
		if ret.dl.IsZero() || dl.Before(ret.dl) {
			// The new deadline is sooner than the ctx one
			ret.dl = dl
		}
	}
	return ret
}

func (p *taskContext) Deadline() (deadline time.Time, ok bool) {
	return p.dl, !p.dl.IsZero()
}

func (p *taskContext) Done() <-chan struct{} {
	return p.ch
}

// only used internally
// will not return to end user
var errTaskDone = errors.New("task done")

func (p *taskContext) Err() error {
	p.mu.Lock()
	err := p.err
	p.mu.Unlock()
	if err == nil || err == errTaskDone {
		return p.Context.Err()
	}
	return err
}

func (p *taskContext) Cancel(err error) {
	p.mu.Lock()
	if err != nil && p.err == nil {
		p.err = err
		close(p.ch)
	}
	p.mu.Unlock()
}
