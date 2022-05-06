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

package utils

import (
    `unsafe`

    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/rt`
    `github.com/cloudwego/frugal/iov`
)

func emu_wbuf(ctx hir.CallContext) (v iov.BufferWriter) {
    (*rt.GoIface)(unsafe.Pointer(&v)).Itab = ctx.Itab()
    (*rt.GoIface)(unsafe.Pointer(&v)).Value = ctx.Data()
    return
}

func emu_bytes(ctx hir.CallContext, i int) (v []byte) {
    return rt.BytesFrom(ctx.Ap(i), int(ctx.Au(i + 1)), int(ctx.Au(i + 2)))
}

func emu_seterr(ctx hir.CallContext, err error) {
    vv := (*rt.GoIface)(unsafe.Pointer(&err))
    ctx.Rp(0, unsafe.Pointer(vv.Itab))
    ctx.Rp(1, vv.Value)
}

func emu_icall_ZeroCopyWriter_WriteDirect(ctx hir.CallContext) {
    if !ctx.Verify("*iii", "**") {
        panic("invalid ZeroCopyWriter.WriteDirect call")
    } else {
        emu_seterr(ctx, emu_wbuf(ctx).WriteDirect(emu_bytes(ctx, 0), int(ctx.Au(3))))
    }
}
