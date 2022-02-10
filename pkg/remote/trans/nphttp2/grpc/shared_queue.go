// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpc

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/cloudwego/netpoll"
)

/* DOC:
 * ShardQueue uses the netpoll's nocopy API to merge and send data.
 * The Data Flush is passively triggered by ShardQueue.Add and does not require user operations.
 * If there is an error in the data transmission, the connection will be closed.
 *
 * ShardQueue.Add: add the data to be sent.
 * NewShardQueue: create a queue with netpoll.Connection.
 * ShardSize: the recommended number of shards is 32.
 */
var ShardSize int

func init() {
	ShardSize = 1
}

// NewShardQueue .
func NewShardQueue(size int, conn netpoll.Connection) (queue *ShardQueue) {
	queue = &ShardQueue{
		conn:    conn,
		size:    int32(size),
		getters: make([][]WriterGetter, size),
		swap:    make([]WriterGetter, 0, 64),
		locks:   make([]int32, size),
	}
	for i := range queue.getters {
		queue.getters[i] = make([]WriterGetter, 0, 64)
	}
	queue.list = make([]int32, size)
	return queue
}

// WriterGetter is used to get a netpoll.Writer.
type WriterGetter func() (buf netpoll.Writer, isNil bool)

// ShardQueue uses the netpoll's nocopy API to merge and send data.
// The Data Flush is passively triggered by ShardQueue.Add and does not require user operations.
// If there is an error in the data transmission, the connection will be closed.
// ShardQueue.Add: add the data to be sent.
type ShardQueue struct {
	conn      netpoll.Connection
	idx, size int32
	getters   [][]WriterGetter // len(getters) = size
	swap      []WriterGetter   // use for swap
	locks     []int32          // len(locks) = size
	queueTrigger
}

// here for trigger
type queueTrigger struct {
	trigger  int32
	process  int32
	list     []int32    // record the triggered shard
	w, r     int32      // ptr of list
	listLock sync.Mutex // list total lock
}

// Add adds to q.getters[shard]
func (q *ShardQueue) Add(gts ...WriterGetter) {
	shard := atomic.AddInt32(&q.idx, 1) % q.size
	q.lock(shard)
	trigger := len(q.getters[shard]) == 0
	q.getters[shard] = append(q.getters[shard], gts...)
	q.unlock(shard)
	if trigger {
		q.triggering(shard)
	}
}

// triggering shard.
func (q *ShardQueue) triggering(shard int32) {
	q.listLock.Lock()
	q.w = (q.w + 1) % q.size
	q.list[q.w] = shard
	q.listLock.Unlock()

	if atomic.AddInt32(&q.trigger, 1) > 1 {
		return
	}
	q.foreach()
}

// foreach swap r & w. It's not concurrency safe.
func (q *ShardQueue) foreach() {
	if !atomic.CompareAndSwapInt32(&q.process, 0, 1) {
		return
	}
	gopool.CtxGo(nil, func() {
	LOOPWRITE:
		var negNum int32 // is negative number of triggerNum
		for triggerNum := atomic.LoadInt32(&q.trigger); triggerNum > 0; {
			q.r = (q.r + 1) % q.size
			shared := q.list[q.r]

			// lock & swap
			q.lock(shared)
			tmp := q.getters[shared]
			q.getters[shared] = q.swap[:0]
			q.swap = tmp
			q.unlock(shared)

			// deal
			q.deal(q.swap)
			negNum--
			if triggerNum+negNum == 0 {
				triggerNum = atomic.AddInt32(&q.trigger, negNum)
				negNum = 0
				if triggerNum > 0 {
					runtime.Gosched()
				}
			}
		}
		q.flush()

		// quit & check again
		atomic.StoreInt32(&q.process, 0)
		if atomic.LoadInt32(&q.trigger) > 0 && atomic.CompareAndSwapInt32(&q.process, 0, 1) {
			// wait for more data coming
			runtime.Gosched()
			goto LOOPWRITE
		}
	})
}

// deal is used to get deal of netpoll.Writer.
func (q *ShardQueue) deal(gts []WriterGetter) {
	writer := q.conn.Writer()
	for _, gt := range gts {
		buf, isNil := gt()
		if !isNil {
			err := writer.Append(buf)
			if err != nil {
				q.conn.Close()
				return
			}
		}
	}
}

// flush is used to flush netpoll.Writer.
func (q *ShardQueue) flush() {
	err := q.conn.Writer().Flush()
	if err != nil {
		q.conn.Close()
		return
	}
}

// lock shard.
func (q *ShardQueue) lock(shard int32) {
	for !atomic.CompareAndSwapInt32(&q.locks[shard], 0, 1) {
		runtime.Gosched()
	}
}

// unlock shard.
func (q *ShardQueue) unlock(shard int32) {
	atomic.StoreInt32(&q.locks[shard], 0)
}
