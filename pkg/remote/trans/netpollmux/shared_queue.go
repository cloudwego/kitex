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

package netpollmux

import (
	"runtime"
	"sync/atomic"

	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/remote"
)

// BufferGetter is used to get a remote.ByteBuffer.
type BufferGetter func() (buf remote.ByteBuffer, isNil bool)

// DealBufferGetters is used to get deal of remote.ByteBuffer.
type DealBufferGetters func(gts []BufferGetter)

// FlushBufferGetters is used to flush remote.ByteBuffer.
type FlushBufferGetters func()

func newSharedQueue(size int32, deal DealBufferGetters, flush FlushBufferGetters) (queue *sharedQueue) {
	queue = &sharedQueue{
		size:    size,
		deal:    deal,
		flush:   flush,
		getters: make([][]BufferGetter, size),
		swap:    make([]BufferGetter, 0, 64),
		locks:   make([]int32, size),
	}
	for i := range queue.getters {
		queue.getters[i] = make([]BufferGetter, 0, 64)
	}
	return queue
}

type sharedQueue struct {
	idx, size       int32
	deal            DealBufferGetters
	flush           FlushBufferGetters
	getters         [][]BufferGetter // len(getters) = size
	swap            []BufferGetter   // use for swap
	locks           []int32          // len(locks) = size
	trigger, runNum int32
}

// Add adds to q.getters[shared]
func (q *sharedQueue) Add(gts ...BufferGetter) {
	shared := atomic.AddInt32(&q.idx, 1) % q.size
	q.Lock(shared)
	trigger := len(q.getters[shared]) == 0
	q.getters[shared] = append(q.getters[shared], gts...)
	q.Unlock(shared)
	if trigger {
		q.Trigger(shared)
	}
}

// Trigger triggers shared
func (q *sharedQueue) Trigger(shared int32) {
	if atomic.AddInt32(&q.trigger, 1) > 1 {
		return
	}
	q.ForEach(shared)
}

// ForEach swap r & w. It's not concurrency safe.
func (q *sharedQueue) ForEach(shared int32) {
	if atomic.AddInt32(&q.runNum, 1) > 1 {
		return
	}
	gofunc.GoFunc(nil, func() {
		var tmp []BufferGetter
		for ; atomic.LoadInt32(&q.trigger) > 0; shared = (shared + 1) % q.size {
			// lock & swap
			q.Lock(shared)
			if len(q.getters[shared]) == 0 {
				q.Unlock(shared)
				continue
			}
			// swap
			tmp = q.getters[shared]
			q.getters[shared] = q.swap[:0]
			q.swap = tmp
			q.Unlock(shared)
			atomic.AddInt32(&q.trigger, -1)

			// deal
			q.deal(q.swap)
		}
		if q.flush != nil {
			q.flush()
		}

		// quit & check again
		atomic.StoreInt32(&q.runNum, 0)
		if atomic.LoadInt32(&q.trigger) > 0 {
			q.ForEach(shared)
		}
	})
}

// Lock locks shared.
func (q *sharedQueue) Lock(shared int32) {
	for !atomic.CompareAndSwapInt32(&q.locks[shared], 0, 1) {
		runtime.Gosched()
	}
}

// Unlock unlocks shared.
func (q *sharedQueue) Unlock(shared int32) {
	atomic.StoreInt32(&q.locks[shared], 0)
}
