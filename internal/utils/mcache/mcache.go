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

package mcache

import (
	"sync"
	"unsafe"
)

const maxSize = 46

//go:linkname mallocgc runtime.mallocgc
func mallocgc(size uintptr, typ unsafe.Pointer, needzero bool) unsafe.Pointer

// index contains []byte which cap is 1<<index
var caches [maxSize]sync.Pool

type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

func init() {
	for i := 0; i < maxSize; i++ {
		size := 1 << i
		caches[i].New = func() interface{} {
			return (*byte)(mallocgc(uintptr(size), nil, false))
		}
	}
}

// calculates which pool to get from
func calcIndex(size int) int {
	if size == 0 {
		return 0
	}
	if isPowerOfTwo(size) {
		return bsr(size)
	}
	return bsr(size) + 1
}

// Malloc supports one or two integer argument.
// The size specifies the length of the returned slice, which means len(ret) == size.
// A second integer argument may be provided to specify the minimum capacity, which means cap(ret) >= cap.
func Malloc(size int, capacity ...int) []byte {
	if len(capacity) > 1 {
		panic("too many arguments to Malloc")
	}
	c := size
	if len(capacity) > 0 && capacity[0] > size {
		c = capacity[0]
	}
	if c == 0 {
		return []byte{}
	}
	p := caches[calcIndex(c)].Get().(*byte)
	ret := []byte{}
	h := (*sliceHeader)(unsafe.Pointer(&ret))
	h.Data = unsafe.Pointer(p)
	h.Len = size
	h.Cap = c
	return ret
}

// Free should be called when the buf is no longer used.
func Free(buf []byte) {
	size := cap(buf)
	if size == 0 {
		return
	}
	if !isPowerOfTwo(size) {
		return
	}
	h := (*sliceHeader)(unsafe.Pointer(&buf))
	caches[bsr(size)].Put((*byte)(h.Data))
}
