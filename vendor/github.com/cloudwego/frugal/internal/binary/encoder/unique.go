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
    `unsafe`

    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/rt`
)

const (
    _N_u32 = unsafe.Sizeof(uint32(0))
    _N_u64 = unsafe.Sizeof(uint64(0))
    _N_str = unsafe.Sizeof(rt.GoString{})
)

func u32at(p unsafe.Pointer, i int) uint32 {
    return *(*uint32)(unsafe.Pointer(uintptr(p) + uintptr(i) * _N_u32))
}

func u64at(p unsafe.Pointer, i int) uint64 {
    return *(*uint64)(unsafe.Pointer(uintptr(p) + uintptr(i) * _N_u64))
}

func straddr(p unsafe.Pointer, i int) unsafe.Pointer {
    return unsafe.Pointer(uintptr(p) + uintptr(i) * _N_str)
}

func unique32(p unsafe.Pointer, nb int) bool {
    dup := false
    bmp := newBucket(nb * 2)

    /* put all the items */
    for i := 0; !dup && i < nb; i++ {
        dup = bucketAppend64(bmp, uint64(u32at(p, i)))
    }

    /* free the bucket */
    freeBucket(bmp)
    return dup
}

func unique64(p unsafe.Pointer, nb int) bool {
    dup := false
    bmp := newBucket(nb * 2)

    /* put all the items */
    for i := 0; !dup && i < nb; i++ {
        dup = bucketAppend64(bmp, u64at(p, i))
    }

    /* free the bucket */
    freeBucket(bmp)
    return dup
}

func uniquestr(p unsafe.Pointer, nb int) bool {
    dup := false
    bmp := newBucket(nb * 2)

    /* put all the items */
    for i := 0; !dup && i < nb; i++ {
        dup = bucketAppendStr(bmp, straddr(p, i))
    }

    /* free the bucket */
    freeBucket(bmp)
    return dup
}

var (
    F_unique32  = hir.RegisterGCall(unique32, emu_gcall_unique32)
    F_unique64  = hir.RegisterGCall(unique64, emu_gcall_unique64)
    F_uniquestr = hir.RegisterGCall(uniquestr, emu_gcall_uniquestr)
)
