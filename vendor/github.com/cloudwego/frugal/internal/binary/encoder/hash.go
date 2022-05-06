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
)

//go:noescape
//go:linkname strhash runtime.strhash
//goland:noinspection GoUnusedParameter
func strhash(p unsafe.Pointer, h uintptr) uintptr

func mkhash(v int) int {
    if v != 0 {
        return v
    } else {
        return 1
    }
}

func hash64(h uint64) int {
    h ^= h >> 33
    h *= 0x9e3779b97f4a7c15
    return mkhash(int(h &^ (1 << 63)))
}

func hashstr(p unsafe.Pointer) int {
    return mkhash(int(strhash(p, 0) &^ (1 << 63)))
}
