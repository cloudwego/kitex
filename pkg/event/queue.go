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

package event

import (
	"sync"
	"sync/atomic"
)

const (
	// MaxEventNum is the default size of a event queue.
	MaxEventNum = 200
)

// Queue is a ring to collect events.
type Queue interface {
	Push(e *Event)
	Dump() interface{}
}

// queue implements a fixed size Queue.
type queue struct {
	ring        []*Event
	tail        uint32
	tailVersion map[uint32]*uint32
	mu          sync.RWMutex
}

// NewQueue creates a queue with the given capacity.
func NewQueue(cap int) Queue {
	q := &queue{
		ring:        make([]*Event, cap),
		tailVersion: make(map[uint32]*uint32, cap),
	}
	for i := 0; i <= cap; i++ {
		t := uint32(0)
		q.tailVersion[uint32(i)] = &t
	}
	return q
}

// Push pushes an event to the queue.
func (q *queue) Push(e *Event) {
	for {
		old := atomic.LoadUint32(&q.tail)
		new := old + 1
		if new >= uint32(len(q.ring)) {
			new = 0
		}
		oldV := atomic.LoadUint32(q.tailVersion[old])
		newV := oldV + 1
		if atomic.CompareAndSwapUint32(&q.tail, old, new) && atomic.CompareAndSwapUint32(q.tailVersion[old], oldV, newV) {
			q.mu.Lock()
			q.ring[old] = e
			q.mu.Unlock()
			break
		}
	}
}

// Dump dumps the previously pushed events out in a reversed order.
func (q *queue) Dump() interface{} {
	results := make([]*Event, 0, len(q.ring))
	q.mu.RLock()
	defer q.mu.RUnlock()
	pos := int32(q.tail)
	for i := 0; i < len(q.ring); i++ {
		pos--
		if pos < 0 {
			pos = int32(len(q.ring) - 1)
		}

		e := q.ring[pos]
		if e == nil {
			return results
		}

		results = append(results, e)
	}

	return results
}
