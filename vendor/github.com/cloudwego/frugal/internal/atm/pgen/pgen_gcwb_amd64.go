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

package pgen

import (
    `github.com/chenzhuoyu/iasm/x86_64`
    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/atm/rtx`
)

func (self *CodeGen) wbStorePointer(p *x86_64.Program, s hir.PointerRegister, d *x86_64.MemoryOperand) {
    wb := x86_64.CreateLabel("_wb_store")
    rt := x86_64.CreateLabel("_wb_return")

    /* check for write barrier */
    p.MOVQ (rtx.V_pWriteBarrier, RAX)
    p.CMPB (0, Ptr(RAX, 0))
    p.JNE  (wb)

    /* check for storing nil */
    if s == hir.Pn {
        p.MOVQ(0, d)
    } else {
        p.MOVQ(self.r(s), d)
    }

    /* set source pointer */
    wbSetSrc := func() {
        if s == hir.Pn {
            p.XORL(EAX, EAX)
        } else {
            p.MOVQ(self.r(s), RAX)
        }
    }

    /* set target slot pointer */
    wbSetSlot := func() {
        if !isSimpleMem(d) {
            p.LEAQ(d.Retain(), RDI)
        } else {
            p.MOVQ(d.Addr.Memory.Base, RDI)
        }
    }

    /* write barrier wrapper */
    wbStoreFn := func(p *x86_64.Program) {
        wbSetSrc                ()
        wbSetSlot               ()
        self.abiSpillReserved   (p)
        self.abiLoadReserved    (p)
        p.MOVQ                  (int64(rtx.F_gcWriteBarrier), RSI)
        p.CALLQ                 (RSI)
        self.abiSaveReserved    (p)
        self.abiRestoreReserved (p)
        p.JMP                   (rt)
    }

    /* defer the call to the end of generated code */
    p.Link(rt)
    self.later(wb, wbStoreFn)
}
