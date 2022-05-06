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
    `github.com/cloudwego/frugal/internal/atm/hir`
)

func bool2u64(v bool) uint64 {
    if v {
        return 1
    } else {
        return 0
    }
}

func emu_gcall_unique32(ctx hir.CallContext) {
    if !ctx.Verify("*i", "i") {
        panic("invalid unique32 call")
    } else {
        ctx.Ru(0, bool2u64(unique32(ctx.Ap(0), int(ctx.Au(1)))))
    }
}

func emu_gcall_unique64(ctx hir.CallContext) {
    if !ctx.Verify("*i", "i") {
        panic("invalid unique64 call")
    } else {
        ctx.Ru(0, bool2u64(unique64(ctx.Ap(0), int(ctx.Au(1)))))
    }
}

func emu_gcall_uniquestr(ctx hir.CallContext) {
    if !ctx.Verify("*i", "i") {
        panic("invalid uniquestr call")
    } else {
        ctx.Ru(0, bool2u64(uniquestr(ctx.Ap(0), int(ctx.Au(1)))))
    }
}
