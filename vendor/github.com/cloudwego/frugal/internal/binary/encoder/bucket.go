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
    `sync`
    `unsafe`

    `github.com/cloudwego/frugal/internal/rt`
)

type _Bucket struct {
    h int
    v uint64
    p unsafe.Pointer
}

func (self _Bucket) String() string {
    return rt.StringFrom(self.p, int(self.v))
}

var (
    bucketPool sync.Pool
)

const (
    _BucketSize = unsafe.Sizeof(_Bucket{})
)

//go:noescape
//go:linkname memclrNoHeapPointers runtime.memclrNoHeapPointers
//goland:noinspection GoUnusedParameter
func memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr)

func newBucket(n int) []_Bucket {
    var r []_Bucket
    var v interface{}

    /* attempt to get one from pool */
    if v = bucketPool.Get(); v == nil {
        return make([]_Bucket, n)
    }

    /* check for capacity, and update it's capacity */
    if r = v.([]_Bucket); n <= cap(r) {
        return bucketClear(r[:n:cap(r)])
    }

    /* not enough space, reallocate a new one */
    bucketPool.Put(v)
    return make([]_Bucket, n)
}

func freeBucket(p []_Bucket) {
    bucketPool.Put(p)
}

func bucketClear(bm []_Bucket) []_Bucket {
    v := (*rt.GoSlice)(unsafe.Pointer(&bm))
    memclrNoHeapPointers(v.Ptr, uintptr(v.Len) * _BucketSize)
    return bm
}

func bucketAppend64(bm []_Bucket, v uint64) bool {
    h := hash64(v)
    i := h % len(bm)

    /* search for an empty slot (linear probing, this must
     * success, since item count never exceeds the bucket size) */
    for bm[i].h != 0 {
        if bm[i].v == v {
            return true
        } else {
            i = (i + 1) % len(bm)
        }
    }

    /* assign the slot */
    bm[i].h = h
    bm[i].v = v
    return false
}

func bucketAppendStr(bm []_Bucket, p unsafe.Pointer) bool {
    h := hashstr(p)
    i := h % len(bm)
    v := (*rt.GoString)(p)

    /* search for an empty slot (linear probing, this must
     * success, since item count never exceeds the bucket size) */
    for bm[i].h != 0 {
        if bm[i].h == h && bm[i].v == uint64(v.Len) && (bm[i].p == v.Ptr || bm[i].String() == *(*string)(p)) {
            return true
        } else {
            i = (i + 1) % len(bm)
        }
    }

    /* assign the slot */
    bm[i].h = h
    bm[i].p = v.Ptr
    bm[i].v = uint64(v.Len)
    return false
}
