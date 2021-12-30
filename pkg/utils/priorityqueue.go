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

package utils

import (
	"container/heap"
	"sync"
)

type PQItem interface {
	Less(i interface{}) bool
}

type pQueue []PQItem

func (q pQueue) Len() int { return len(q) }

func (q pQueue) Less(i, j int) bool {
	return q[i].Less(q[j])
}

func (q pQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func (q *pQueue) Push(i interface{}) {
	*q = append(*q, i.(PQItem))
}

func (q *pQueue) Pop() interface{} {
	n := len(*q)
	if n == 0 {
		return nil
	}
	item := (*q)[n-1]
	*q = (*q)[0 : n-1]
	return item
}

type PriorityQueue struct {
	queue *pQueue
	m     sync.Mutex
}

func (pq *PriorityQueue) Pop() PQItem {
	pq.m.Lock()
	defer pq.m.Unlock()

	if pq.queue.Len() == 0 {
		return nil
	}

	return heap.Pop(pq.queue).(PQItem)
}

func (pq *PriorityQueue) Push(i PQItem) {
	pq.m.Lock()
	defer pq.m.Unlock()

	heap.Push(pq.queue, i)
}

func (pq *PriorityQueue) Top() PQItem {
	pq.m.Lock()
	defer pq.m.Unlock()

	if pq.queue == nil || len(*pq.queue) == 0 {
		return nil
	}
	return (*pq.queue)[0]
}

func (pq *PriorityQueue) Len() int {
	pq.m.Lock()
	defer pq.m.Unlock()

	return pq.queue.Len()
}

func (pq *PriorityQueue) Close() {
	pq.m.Lock()
	defer pq.m.Unlock()

	pq.queue = nil
}

func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{queue: new(pQueue)}
	heap.Init(pq.queue)
	return pq
}
