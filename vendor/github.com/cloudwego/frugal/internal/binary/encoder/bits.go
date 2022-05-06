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
    `math/bits`
)

func bswap16(v int64) int16 {
    return int16(bits.ReverseBytes16(uint16(v)))
}

func bswap32(v int64) int32 {
    return int32(bits.ReverseBytes32(uint32(v)))
}

func bswap64(v int64) int64 {
    return int64(bits.ReverseBytes64(uint64(v)))
}
