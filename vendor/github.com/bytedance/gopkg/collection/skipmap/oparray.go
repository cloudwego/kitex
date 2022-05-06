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

import (
	"sync/atomic"
	"unsafe"
)

const (
	op1 = 4
	op2 = maxLevel - op1
)

type optionalArray struct {
	base  [op1]unsafe.Pointer
	extra *([op2]unsafe.Pointer)
}

func (a *optionalArray) load(i int) unsafe.Pointer {
	if i < op1 {
		return a.base[i]
	}
	return a.extra[i-op1]
}

func (a *optionalArray) store(i int, p unsafe.Pointer) {
	if i < op1 {
		a.base[i] = p
		return
	}
	a.extra[i-op1] = p
}

func (a *optionalArray) atomicLoad(i int) unsafe.Pointer {
	if i < op1 {
		return atomic.LoadPointer(&a.base[i])
	}
	return atomic.LoadPointer(&a.extra[i-op1])
}

func (a *optionalArray) atomicStore(i int, p unsafe.Pointer) {
	if i < op1 {
		atomic.StorePointer(&a.base[i], p)
		return
	}
	atomic.StorePointer(&a.extra[i-op1], p)
}
