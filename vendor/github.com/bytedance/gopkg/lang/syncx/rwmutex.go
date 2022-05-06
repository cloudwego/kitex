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

package syncx

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/bytedance/gopkg/internal/runtimex"

	"golang.org/x/sys/cpu"
)

const (
	cacheLineSize = unsafe.Sizeof(cpu.CacheLinePad{})
)

var (
	shardsLen int
)

// RWMutex is a p-shard mutex, which has better performance when there's much more read than write.
type RWMutex []rwMutexShard

type rwMutexShard struct {
	sync.RWMutex
	_pad [cacheLineSize - unsafe.Sizeof(sync.RWMutex{})]byte
}

func init() {
	shardsLen = runtime.GOMAXPROCS(0)
}

// NewRWMutex creates a new RWMutex.
func NewRWMutex() RWMutex {
	return make([]rwMutexShard, shardsLen)
}

func (m RWMutex) Lock() {
	for shard := range m {
		m[shard].Lock()
	}
}

func (m RWMutex) Unlock() {
	for shard := range m {
		m[shard].Unlock()
	}
}

func (m RWMutex) RLocker() sync.Locker {
	return m[runtimex.Pid()%shardsLen].RWMutex.RLocker()
}
