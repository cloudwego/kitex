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

package skipmap

import "sync/atomic"

const (
	fullyLinked = 1 << iota
	marked
)

// concurrent-safe bitflag.
type bitflag struct {
	data uint32
}

func (f *bitflag) SetTrue(flags uint32) {
	for {
		old := atomic.LoadUint32(&f.data)
		if old&flags != flags {
			// Flag is 0, need set it to 1.
			n := old | flags
			if atomic.CompareAndSwapUint32(&f.data, old, n) {
				return
			}
			continue
		}
		return
	}
}

func (f *bitflag) SetFalse(flags uint32) {
	for {
		old := atomic.LoadUint32(&f.data)
		check := old & flags
		if check != 0 {
			// Flag is 1, need set it to 0.
			n := old ^ check
			if atomic.CompareAndSwapUint32(&f.data, old, n) {
				return
			}
			continue
		}
		return
	}
}

func (f *bitflag) Get(flag uint32) bool {
	return (atomic.LoadUint32(&f.data) & flag) != 0
}

func (f *bitflag) MGet(check, expect uint32) bool {
	return (atomic.LoadUint32(&f.data) & check) == expect
}
