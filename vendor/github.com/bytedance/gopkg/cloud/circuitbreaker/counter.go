// Copyright 2021 ByteDance Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package circuitbreaker

import (
	"runtime"
	"sync/atomic"

	"github.com/bytedance/gopkg/internal/runtimex"
)

type Counter interface {
	Add(i int64)
	Get() int64
	Zero()
}

type atomicCounter struct {
	x int64
}

func (c *atomicCounter) Add(i int64) {
	atomic.AddInt64(&c.x, i)
}

func (c *atomicCounter) Get() int64 {
	return atomic.LoadInt64(&c.x)
}

func (c *atomicCounter) Zero() {
	atomic.StoreInt64(&c.x, 0)
}

const (
	cacheLineSize = 64
)

var (
	countersLen int
)

func init() {
	countersLen = runtime.GOMAXPROCS(0)
}

type counterShard struct {
	x int64
	_ [cacheLineSize - 8]byte
}

type perPCounter []counterShard

func newPerPCounter() perPCounter {
	return make([]counterShard, countersLen)
}

func (c perPCounter) Add(i int64) {
	tid := runtimex.Pid()
	atomic.AddInt64(&c[tid%countersLen].x, i)
}

// Get is not precise, but it's ok in this scenario.
func (c perPCounter) Get() int64 {
	var n int64
	for i := range c {
		n += atomic.LoadInt64(&c[i].x)
	}
	return n
}

func (c perPCounter) Zero() {
	for i := range c {
		atomic.StoreInt64(&c[i].x, 0)
	}
}
