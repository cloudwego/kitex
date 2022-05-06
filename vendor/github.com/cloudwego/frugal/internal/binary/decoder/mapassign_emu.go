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

func emu_string(ctx hir.CallContext, i int) (v string) {
    (*rt.GoString)(unsafe.Pointer(&v)).Ptr = ctx.Ap(i)
    (*rt.GoString)(unsafe.Pointer(&v)).Len = int(ctx.Au(i + 1))
    return
}

func emu_gcall_mapassign(ctx hir.CallContext) {
    if !ctx.Verify("***", "*") {
        panic("invalid mapassign call")
    } else {
        ctx.Rp(0, mapassign((*rt.GoMapType)(ctx.Ap(0)), (*rt.GoMap)(ctx.Ap(1)), ctx.Ap(2)))
    }
}

func emu_gcall_mapassign_fast32(ctx hir.CallContext) {
    if !ctx.Verify("**i", "*") {
        panic("invalid mapassign_fast32 call")
    } else {
        ctx.Rp(0, mapassign_fast32((*rt.GoMapType)(ctx.Ap(0)), (*rt.GoMap)(ctx.Ap(1)), uint32(ctx.Au(2))))
    }
}

func emu_gcall_mapassign_fast64(ctx hir.CallContext) {
    if !ctx.Verify("**i", "*") {
        panic("invalid mapassign_fast64 call")
    } else {
        ctx.Rp(0, mapassign_fast64((*rt.GoMapType)(ctx.Ap(0)), (*rt.GoMap)(ctx.Ap(1)), ctx.Au(2)))
    }
}

func emu_gcall_mapassign_faststr(ctx hir.CallContext) {
    if !ctx.Verify("***i", "*") {
        panic("invalid mapassign_faststr call")
    } else {
        ctx.Rp(0, mapassign_faststr((*rt.GoMapType)(ctx.Ap(0)), (*rt.GoMap)(ctx.Ap(1)), emu_string(ctx, 2)))
    }
}

func emu_gcall_mapassign_fast64ptr(ctx hir.CallContext) {
    if !ctx.Verify("***", "*") {
        panic("invalid mapassign_fast64 call")
    } else {
        ctx.Rp(0, mapassign_fast64ptr((*rt.GoMapType)(ctx.Ap(0)), (*rt.GoMap)(ctx.Ap(1)), ctx.Ap(2)))
    }
}
