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

    `github.com/cloudwego/frugal/internal/atm/emu`
    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/rt`
)

func link_emu(prog hir.Program) Decoder {
    return func(buf unsafe.Pointer, nb int, i int, p unsafe.Pointer, rs *RuntimeState, st int) (pos int, err error) {
        ctx := emu.LoadProgram(prog)
        ret := (*rt.GoIface)(unsafe.Pointer(&err))
        ctx.Ap(0, buf)
        ctx.Au(1, uint64(nb))
        ctx.Au(2, uint64(i))
        ctx.Ap(3, p)
        ctx.Ap(4, unsafe.Pointer(rs))
        ctx.Au(5, uint64(st))
        ctx.Run()
        pos = int(ctx.Ru(0))
        ret.Itab = (*rt.GoItab)(ctx.Rp(1))
        ret.Value = ctx.Rp(2)
        ctx.Free()
        return
    }
}

func emu_decode(ctx hir.CallContext) (int, error) {
    return decode(
        (*rt.GoType)(ctx.Ap(0)),
        ctx.Ap(1),
        int(ctx.Au(2)),
        int(ctx.Au(3)),
        ctx.Ap(4),
        (*RuntimeState)(ctx.Ap(5)),
        int(ctx.Au(6)),
    )
}

func emu_mkreturn(ctx hir.CallContext) func(int, error) {
    return func(ret int, err error) {
        ctx.Ru(0, uint64(ret))
        emu_seterr(ctx, 1, err)
    }
}

func emu_gcall_decode(ctx hir.CallContext) {
    if !ctx.Verify("**ii**i", "i**") {
        panic("invalid decode call")
    } else {
        emu_mkreturn(ctx)(emu_decode(ctx))
    }
}