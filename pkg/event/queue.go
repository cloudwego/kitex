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

var defaultEventNum = int64(MaxEventNum)

// GetDefaultEventNum returns the default event number for Queue
//
// concurrent safe.
func GetDefaultEventNum() int {
	return int(atomic.LoadInt64(&defaultEventNum))
}

// SetDefaultEventNum sets the default event number for Queue to num.
// It only affects queues created after the call; existing queues are not resized.
// It is not exposed as client/server option because the queue is used for internal debugging workflows,
// and users do not need to be aware of it.
//
// num must be > 0; non-positive values are ignored and the default remains unchanged.
// concurrent safe.
func SetDefaultEventNum(num int) {
	if num <= 0 {
		return
	}
	atomic.StoreInt64(&defaultEventNum, int64(num))
}

const (
	// MaxEventNum is the initial default size of an event queue.
	// Use GetDefaultEventNum() to read the runtime default,
	// which can be changed via SetDefaultEventNum().
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
	q.mu.Lock()
	defer q.mu.Unlock()

	q.ring[q.tail] = e

	newVersion := (*(q.tailVersion[q.tail])) + 1
	q.tailVersion[q.tail] = &newVersion

	q.tail = (q.tail + 1) % uint32(len(q.ring))
}

// Dump dumps the previously pushed events out in a reversed order.
func (q *queue) Dump() interface{} {
	q.mu.RLock()
	results := make([]*Event, 0, len(q.ring))
	pos := int32(q.tail)
	for i := 0; i < len(q.ring); i++ {
		pos--
		if pos < 0 {
			pos = int32(len(q.ring) - 1)
		}

		e := q.ring[pos]
		if e == nil {
			break
		}
		results = append(results, e)
	}
	q.mu.RUnlock()

	for i, res := range results {
		if lazy, ok := res.Extra.(lazyExtra); ok {
			// copy a new event to prevent Extra field race
			cp := *res
			cp.Extra = lazy.KitexDumpLazyExtra()
			results[i] = &cp
		}
	}

	return results
}
