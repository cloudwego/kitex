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

    `github.com/cloudwego/frugal/internal/atm/emu`
    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/rt`
    `github.com/cloudwego/frugal/iov`
)

func link_emu(prog hir.Program) Encoder {
    return func(buf unsafe.Pointer, len int, mem iov.BufferWriter, p unsafe.Pointer, rs *RuntimeState, st int) (ret int, err error) {
        ctx := emu.LoadProgram(prog)
        exc := (*rt.GoIface)(unsafe.Pointer(&err))
        iop := (*rt.GoIface)(unsafe.Pointer(&mem))
        ctx.Ap(0,buf)
        ctx.Au(1, uint64(len))
        ctx.Ap(2, unsafe.Pointer(iop.Itab))
        ctx.Ap(3, iop.Value)
        ctx.Ap(4, p)
        ctx.Ap(5, unsafe.Pointer(rs))
        ctx.Au(6, uint64(st))
        ctx.Run()
        ret = int(ctx.Ru(0))
        exc.Itab = (*rt.GoItab)(ctx.Rp(1))
        exc.Value = ctx.Rp(2)
        ctx.Free()
        return
    }
}

func emu_wbuf(ctx hir.CallContext, i int) (v iov.BufferWriter) {
    (*rt.GoIface)(unsafe.Pointer(&v)).Itab = (*rt.GoItab)(ctx.Ap(i))
    (*rt.GoIface)(unsafe.Pointer(&v)).Value = ctx.Ap(i + 1)
    return
}

func emu_setret(ctx hir.CallContext) func(int, error) {
    return func(ret int, err error) {
        vv := (*rt.GoIface)(unsafe.Pointer(&err))
        ctx.Ru(0, uint64(ret))
        ctx.Rp(1, unsafe.Pointer(vv.Itab))
        ctx.Rp(2, vv.Value)
    }
}

func emu_encode(ctx hir.CallContext) (int, error) {
    return encode(
        (*rt.GoType)(ctx.Ap(0)),
        ctx.Ap(1),
        int(ctx.Au(2)),
        emu_wbuf(ctx, 3),
        ctx.Ap(5),
        (*RuntimeState)(ctx.Ap(6)),
        int(ctx.Au(7)),
    )
}

func emu_gcall_encode(ctx hir.CallContext) {
    if !ctx.Verify("**i****i", "i**") {
        panic("invalid encode call")
    } else {
        emu_setret(ctx)(emu_encode(ctx))
    }
}
