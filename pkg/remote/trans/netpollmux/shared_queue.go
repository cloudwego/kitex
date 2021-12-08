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
	"sync"
	"sync/atomic"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/gofunc"
)

// BufferGetter is used to get a remote.ByteBuffer.
type BufferGetter func() (buf netpoll.Writer, isNil bool)

// DealBufferGetters is used to get deal of remote.ByteBuffer.
type DealBufferGetters func(gts []BufferGetter)

// FlushBufferGetters is used to flush remote.ByteBuffer.
type FlushBufferGetters func()

func newSharedQueue(size int32, conn netpoll.Connection) (queue *sharedQueue) {
	queue = &sharedQueue{
		size:    size,
		conn:    conn,
		writer:  conn.Writer(),
		getters: make([][]BufferGetter, size),
		swap:    make([]BufferGetter, 0, 64),
		locks:   make([]int32, size),
	}
	for i := range queue.getters {
		queue.getters[i] = make([]BufferGetter, 0, 64)
	}
	queue.list = make([]int32, queue.size)
	return queue
}

type sharedQueue struct {
	idx, size int32
	conn      netpoll.Connection
	writer    netpoll.Writer
	getters   [][]BufferGetter // len(getters) = size
	swap      []BufferGetter   // use for swap
	locks     []int32          // len(locks) = size
	queueTrigger
}

// here for trigger
type queueTrigger struct {
	trigger  int32
	runNum   int32
	list     []int32    // record the triggered shard
	w, r     int32      // ptr of list
	listLock sync.Mutex // list total lock
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
	q.listLock.Lock()
	q.w = (q.w + 1) % q.size
	q.list[q.w] = shared
	q.listLock.Unlock()
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
		var negNum int32 // is negative number of triggerNum
		for triggerNum := atomic.LoadInt32(&q.trigger); triggerNum > 0; {
			q.r = (q.r + 1) % q.size
			shared = q.list[q.r]

			// lock & swap
			q.Lock(shared)
			tmp := q.getters[shared]
			q.getters[shared] = q.swap[:0]
			q.swap = tmp
			q.Unlock(shared)

			// deal
			q.deal(q.swap)
			negNum--
			if triggerNum+negNum == 0 {
				triggerNum = atomic.AddInt32(&q.trigger, negNum)
				negNum = 0
			}
		}
		q.flush()

		// quit & check again
		atomic.StoreInt32(&q.runNum, 0)
		if atomic.LoadInt32(&q.trigger) > 0 {
			q.ForEach(shared)
		}
	})
}

// deal is used to get deal of netpoll.Writer.
func (q *sharedQueue) deal(gts []BufferGetter) {
	var err error
	var buf netpoll.Writer
	var isNil bool
	for _, gt := range gts {
		buf, isNil = gt()
		if !isNil {
			err = q.writer.Append(buf)
			if err != nil {
				q.conn.Close()
				return
			}
		}
	}
}

// flush is used to flush netpoll.Writer.
func (q *sharedQueue) flush() {
	err := q.writer.Flush()
	if err != nil {
		q.conn.Close()
		return
	}
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
