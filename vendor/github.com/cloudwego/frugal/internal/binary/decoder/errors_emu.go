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

func emu_seterr(ctx hir.CallContext, i int, err error) {
    vv := (*rt.GoIface)(unsafe.Pointer(&err))
    ctx.Rp(i, unsafe.Pointer(vv.Itab))
    ctx.Rp(i + 1, vv.Value)
}

func emu_gcall_error_eof(ctx hir.CallContext) {
    if !ctx.Verify("i", "**") {
        panic("invalid error_eof call")
    } else {
        emu_seterr(ctx, 0, error_eof(int(ctx.Au(0))))
    }
}

func emu_gcall_error_skip(ctx hir.CallContext) {
    if !ctx.Verify("i", "**") {
        panic("invalid error_skip call")
    } else {
        emu_seterr(ctx, 0, error_skip(int(ctx.Au(0))))
    }
}

func emu_gcall_error_type(ctx hir.CallContext) {
    if !ctx.Verify("ii", "**") {
        panic("invalid error_type call")
    } else {
        emu_seterr(ctx, 0, error_type(uint8(ctx.Au(0)), uint8(ctx.Au(1))))
    }
}

func emu_gcall_error_missing(ctx hir.CallContext) {
    if !ctx.Verify("*ii", "**") {
        panic("invalid error_skip call")
    } else {
        emu_seterr(ctx, 0, error_missing((*rt.GoType)(ctx.Ap(0)), int(ctx.Au(1)), ctx.Au(2)))
    }
}
