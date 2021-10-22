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
	"errors"
	"runtime"
	"sync/atomic"
)

// ErrRingFull means the ring is full.
var ErrRingFull = errors.New("ring is full")

const (
	patch = 1 << 32
	lower = int64(0xFFFFFFFF)
)

type node struct {
	data interface{}
	next int64
}

// Ring implements a fixed size ring buffer to manage data
type Ring struct {
	objs []node
	free int64 // upper 32 bits for version stamp, lower bits for index
	used int64 // upper 32 bits for version stamp, lower bits for index
}

// NewRing creates a ringbuffer with fixed size.
func NewRing(size int) *Ring {
	if size <= 0 {
		// When size is an invalid number, we still return an instance
		// with zero-size to reduce error checks of the callers.
		size = 0
	}

	r := &Ring{
		objs: make([]node, size),
		free: int64(size - 1),
		used: -1,
	}

	for i := 0; i < size; i++ {
		r.objs[i].next = int64(i - 1)
	}
	return r
}

// Push appends item to the ring.
func (r *Ring) Push(obj interface{}) error {
	visit := func(n *node) { n.data = obj }
	if r.move(&r.free, &r.used, visit) {
		return nil
	}
	return ErrRingFull
}

// Pop returns the last item and removes it from the ring.
func (r *Ring) Pop() (result interface{}) {
	visit := func(n *node) { result = n.data; n.data = nil }
	if r.move(&r.used, &r.free, visit) {
		return
	}
	return nil
}

func (r *Ring) move(src, dst *int64, visit func(n *node)) bool {
	for {
		idx := atomic.LoadInt64(src)
		cur := int32(idx & lower)
		if cur == -1 {
			break
		}
		obj := &r.objs[cur]
		nxt := atomic.LoadInt64(&obj.next)

		tmp := ((idx &^ lower) | (nxt & lower)) + patch
		// cut the link and seize the node
		if atomic.CompareAndSwapInt64(src, idx, tmp) {
			visit(obj)

			// add to the other link
			for {
				idx = atomic.LoadInt64(dst)
				atomic.StoreInt64(&obj.next, idx&lower)
				tmp = ((idx &^ lower) | int64(cur)) + patch

				if atomic.CompareAndSwapInt64(dst, idx, tmp) {
					return true
				}
				runtime.Gosched()
			}
		}
		runtime.Gosched()
	}
	return false
}
