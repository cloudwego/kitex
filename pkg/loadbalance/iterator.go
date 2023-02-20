/*
 * Copyright 2023 CloudWeGo Authors
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

package loadbalance

import (
	"sync"
	"sync/atomic"

	"github.com/bytedance/gopkg/lang/fastrand"
)

var _ iterator = (*round)(nil)
var _ iterator = (*roughRound)(nil)

type iterator interface {
	Next() uint64
}

// round implement a strict Round Robin algorithm
type round struct {
	state uint64    // 8 bytes
	_     [7]uint64 // + 7 * 8 bytes
	// = 64 bytes
}

func (r *round) Next() uint64 {
	return atomic.AddUint64(&r.state, 1)
}

func newRound() iterator {
	r := &round{
		state: fastrand.Uint64(), // every thread have a rand start order
	}
	return r
}

// roughRound implement a rough Round Robin algorithm
// It could promise that every thread has a strict round-robin order, but cannot give any promise when cross threads.
// The rough algorithm can help to reduce contention between threads.
type roughRound struct {
	pool sync.Pool
}

func newRoughRound() iterator {
	return &roughRound{}
}

func (i *roughRound) Next() uint64 {
	x, ok := i.pool.Get().(*round)
	if !ok || x == nil {
		x = newRound().(*round)
	}
	x.state += 1
	n := x.state
	i.pool.Put(x)
	return n
}
