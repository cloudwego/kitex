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

package decoder

import (
    `sync`

    `github.com/cloudwego/frugal/internal/atm/hir`
)

const (
    MaxField  = 65536
    MaxBitmap = MaxField / 64
)

var (
    bitmapPool sync.Pool
)

var (
    F_newFieldBitmap   = hir.RegisterGCall(newFieldBitmap, emu_gcall_newFieldBitmap)
    F_FieldBitmap_Free = hir.RegisterGCall((*FieldBitmap).Free, emu_gcall_FieldBitmap_Free)
)

type (
	FieldBitmap [MaxBitmap]int64
)

func newFieldBitmap() *FieldBitmap {
    if v := bitmapPool.Get(); v != nil {
        return v.(*FieldBitmap)
    } else {
        return new(FieldBitmap)
    }
}

func (self *FieldBitmap) Free() {
    bitmapPool.Put(self)
}

func (self *FieldBitmap) Clear() {
    *self = FieldBitmap{}
}

func (self *FieldBitmap) Append(i int) {
    if i < MaxField {
        self[i / 64] |= 1 << (i % 64)
    } else {
        panic("field index too large")
    }
}
