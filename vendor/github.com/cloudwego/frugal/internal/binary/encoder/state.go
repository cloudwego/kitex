/*
 * Copyright 2022 ByteDance Inc.
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

package encoder

import (
    `math`
    `unsafe`

    `github.com/cloudwego/frugal/internal/binary/defs`
    `github.com/cloudwego/frugal/internal/rt`
)

const (
    LnOffset = int64(unsafe.Offsetof(StateItem{}.Ln))
    MiOffset = int64(unsafe.Offsetof(StateItem{}.Mi))
    WpOffset = int64(unsafe.Offsetof(StateItem{}.Wp))
    BmOffset = int64(unsafe.Offsetof(RuntimeState{}.Bm))
)

const (
    MiSize        = int64(unsafe.Sizeof(rt.GoMapIterator{}))
    MiKeyOffset   = int64(unsafe.Offsetof(rt.GoMapIterator{}.K))
    MiValueOffset = int64(unsafe.Offsetof(rt.GoMapIterator{}.V))
)

const (
    StateMax  = (defs.StackSize - 1) * StateSize
    StateSize = int64(unsafe.Sizeof(StateItem{}))
)

const (
    RangeUint8  = math.MaxUint8 + 1
    RangeUint16 = math.MaxUint16 + 1
)

type StateItem struct {
    Ln uintptr
    Wp unsafe.Pointer
    Mi rt.GoMapIterator
}

type RuntimeState struct {
    St [defs.StackSize]StateItem    // Must be the first field.
    Bm [1024]uint64                 // Bitmap, used for uniqueness check of set<i8> and set<i16>.
}
