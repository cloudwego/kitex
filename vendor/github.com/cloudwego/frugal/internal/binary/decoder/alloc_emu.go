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
    `unsafe`

    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/rt`
)

func emu_gcall_makemap(ctx hir.CallContext) {
    if !ctx.Verify("*i*", "*") {
        panic("invalid makemap call")
    } else {
        ctx.Rp(0, unsafe.Pointer(makemap((*rt.GoMapType)(ctx.Ap(0)), int(ctx.Au(1)), (*rt.GoMap)(ctx.Ap(2)))))
    }
}

func emu_gcall_mallocgc(ctx hir.CallContext) {
    if !ctx.Verify("i*i", "*") {
        panic("invalid mallocgc call")
    } else {
        ctx.Rp(0, mallocgc(uintptr(ctx.Au(0)), (*rt.GoType)(ctx.Ap(1)), ctx.Au(2) != 0))
    }
}
