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
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/profiler"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

var poolTask = sync.Pool{
	New: func() any {
		return &task{}
	},
}

// task is the function that the worker will execute.
type task struct {
	ctx *taskContext

	wg sync.WaitGroup

	req, resp any
	ep        endpoint.Endpoint

	err atomic.Value
}

func newTask(ctx context.Context, timeout time.Duration,
	req, resp any, ep endpoint.Endpoint,
) *task {
	t := poolTask.Get().(*task)

	// taskContext must not be reused,
	// coz user may keep ref to it even though after calling endpoint.Endpoint
	t.ctx = newTaskContext(ctx, timeout)

	t.req, t.resp = req, resp
	t.ep = ep
	t.err = atomic.Value{}
	t.wg.Add(1) // for Wait, Wait must be called before Recycle()
	return t
}

func (t *task) recycle() {
	// make sure Wait done before returning it to pool
	t.wg.Wait()

	t.ctx = nil
	t.req, t.resp = nil, nil
	t.ep = nil
	t.err = atomic.Value{}
	poolTask.Put(t)
}

func (t *task) Cancel(err error) {
	t.ctx.Cancel(err)
}

// Run must be called in a separated goroutine
func (t *task) Run() {
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			ri := rpcinfo.GetRPCInfo(t.ctx)
			if ri != nil {
				t.err.Store(rpcinfo.ClientPanicToErr(t.ctx, panicInfo, ri, true))
			} else {
				t.err.Store(fmt.Errorf("KITEX: panic without rpcinfo, error=%v\nstack=%s",
					panicInfo, debug.Stack()))
			}
		}
		t.Cancel(errTaskDone)
		t.recycle()
	}()
	var err error
	if profiler.IsEnabled(t.ctx) {
		profiler.Tag(t.ctx)
		err = t.ep(t.ctx, t.req, t.resp)
		profiler.Untag(t.ctx)
	} else {
		err = t.ep(t.ctx, t.req, t.resp)
	}
	if err != nil {
		t.err.Store(err) // fix store nil value ...
	}
}

var poolTimer = sync.Pool{
	New: func() any {
		return time.NewTimer(time.Second)
	},
}

// Wait waits Run finishes and returns result
func (t *task) Wait() (context.Context, error) {
	defer t.wg.Done()
	dl, ok := t.ctx.Deadline()
	if !ok {
		return t.waitNoTimeout()
	}
	d := time.Until(dl)
	if d < 0 {
		t.Cancel(context.DeadlineExceeded)
		return t.ctx, t.ctx.Err()
	}
	tm := poolTimer.Get().(*time.Timer)
	if !tm.Stop() {
		select { // it may be expired or stopped
		case <-tm.C:
		default:
		}
	}
	defer poolTimer.Put(tm)
	tm.Reset(d)
	select {
	case <-t.ctx.Done():
		// Run returned before timeout
	case <-t.ctx.Context.Done():
		t.Cancel(t.ctx.Context.Err())
	case <-tm.C:
		t.Cancel(context.DeadlineExceeded)
	}
	if v := t.err.Load(); v != nil {
		return t.ctx, v.(error)
	}
	return t.ctx, t.ctx.Err()
}

func (t *task) waitNoTimeout() (context.Context, error) {
	select {
	case <-t.ctx.Done():
		// Run returned before timeout
	case <-t.ctx.Context.Done():
		t.Cancel(t.ctx.Context.Err())
	}
	if v := t.err.Load(); v != nil {
		return t.ctx, v.(error)
	}
	return t.ctx, t.ctx.Err()
}
