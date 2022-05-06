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

package emu

import (
    `fmt`
    `math/bits`
    `runtime`
    `sync`
    `unsafe`

    `github.com/cloudwego/frugal/internal/atm/hir`
)

type Value struct {
    U uint64
    P unsafe.Pointer
}

type Emulator struct {
    pc *hir.Ir
    uv [6]uint64
    pv [7]unsafe.Pointer
    ar [8]Value
    rv [8]Value
}

var (
    emulatorPool sync.Pool
)

func LoadProgram(p hir.Program) (e *Emulator) {
    if v := emulatorPool.Get(); v == nil {
        return &Emulator{pc: p.Head}
    } else {
        return v.(*Emulator).Reset(p)
    }
}

func bool2u64(val bool) uint64 {
    if val {
        return 1
    } else {
        return 0
    }
}

func (self *Emulator) trap() {
    println("****** DEBUGGER BREAK ******")
    println("Current State:", self.String())
    runtime.Breakpoint()
}

func (self *Emulator) Ru(i int) uint64         { return self.rv[i].U }
func (self *Emulator) Rp(i int) unsafe.Pointer { return self.rv[i].P }

func (self *Emulator) Au(i int, v uint64)         *Emulator { self.ar[i].U = v; return self }
func (self *Emulator) Ap(i int, v unsafe.Pointer) *Emulator { self.ar[i].P = v; return self }

func (self *Emulator) Run() {
    var i uint8
    var v uint64
    var p *hir.Ir
    var q *hir.Ir

    /* run until end */
    for self.pc != nil {
        p, self.pc = self.pc, self.pc.Ln
        self.uv[hir.Rz], self.pv[hir.Pn] = 0, nil

        /* main switch on OpCode */
        switch p.Op {
            default          : return
            case hir.OP_nop   : break
            case hir.OP_ip    : self.pv[p.Pd] = p.Pr
            case hir.OP_lb    : self.uv[p.Rx] = uint64(*(*int8)(unsafe.Pointer(uintptr(self.pv[p.Ps]) + uintptr(p.Iv))))
            case hir.OP_lw    : self.uv[p.Rx] = uint64(*(*int16)(unsafe.Pointer(uintptr(self.pv[p.Ps]) + uintptr(p.Iv))))
            case hir.OP_ll    : self.uv[p.Rx] = uint64(*(*int32)(unsafe.Pointer(uintptr(self.pv[p.Ps]) + uintptr(p.Iv))))
            case hir.OP_lq    : self.uv[p.Rx] = uint64(*(*int64)(unsafe.Pointer(uintptr(self.pv[p.Ps]) + uintptr(p.Iv))))
            case hir.OP_lp    : self.pv[p.Pd] = *(*unsafe.Pointer)(unsafe.Pointer(uintptr(self.pv[p.Ps]) + uintptr(p.Iv)))
            case hir.OP_sb    : *(*int8)(unsafe.Pointer(uintptr(self.pv[p.Pd]) + uintptr(p.Iv))) = int8(self.uv[p.Rx])
            case hir.OP_sw    : *(*int16)(unsafe.Pointer(uintptr(self.pv[p.Pd]) + uintptr(p.Iv))) = int16(self.uv[p.Rx])
            case hir.OP_sl    : *(*int32)(unsafe.Pointer(uintptr(self.pv[p.Pd]) + uintptr(p.Iv))) = int32(self.uv[p.Rx])
            case hir.OP_sq    : *(*int64)(unsafe.Pointer(uintptr(self.pv[p.Pd]) + uintptr(p.Iv))) = int64(self.uv[p.Rx])
            case hir.OP_sp    : *(*unsafe.Pointer)(unsafe.Pointer(uintptr(self.pv[p.Pd]) + uintptr(p.Iv))) = self.pv[p.Ps]
            case hir.OP_ldaq  : self.uv[p.Rx] = self.ar[p.Iv].U
            case hir.OP_ldap  : self.pv[p.Pd] = self.ar[p.Iv].P
            case hir.OP_addp  : self.pv[p.Pd] = unsafe.Pointer(uintptr(self.pv[p.Ps]) + uintptr(self.uv[p.Rx]))
            case hir.OP_subp  : self.pv[p.Pd] = unsafe.Pointer(uintptr(self.pv[p.Ps]) - uintptr(self.uv[p.Rx]))
            case hir.OP_addpi : self.pv[p.Pd] = unsafe.Pointer(uintptr(self.pv[p.Ps]) + uintptr(p.Iv))
            case hir.OP_add   : self.uv[p.Rz] = self.uv[p.Rx] + self.uv[p.Ry]
            case hir.OP_sub   : self.uv[p.Rz] = self.uv[p.Rx] - self.uv[p.Ry]
            case hir.OP_addi  : self.uv[p.Ry] = self.uv[p.Rx] + uint64(p.Iv)
            case hir.OP_muli  : self.uv[p.Ry] = self.uv[p.Rx] * uint64(p.Iv)
            case hir.OP_andi  : self.uv[p.Ry] = self.uv[p.Rx] & uint64(p.Iv)
            case hir.OP_xori  : self.uv[p.Ry] = self.uv[p.Rx] ^ uint64(p.Iv)
            case hir.OP_shri  : self.uv[p.Ry] = self.uv[p.Rx] >> p.Iv
            case hir.OP_bsi   : self.uv[p.Ry] = self.uv[p.Rx] | (1 << p.Iv)
            case hir.OP_swapw : self.uv[p.Ry] = uint64(bits.ReverseBytes16(uint16(self.uv[p.Rx])))
            case hir.OP_swapl : self.uv[p.Ry] = uint64(bits.ReverseBytes32(uint32(self.uv[p.Rx])))
            case hir.OP_swapq : self.uv[p.Ry] = bits.ReverseBytes64(self.uv[p.Rx])
            case hir.OP_sxlq  : self.uv[p.Ry] = uint64(int32(self.uv[p.Rx]))
            case hir.OP_beq   : if       self.uv[p.Rx]  ==       self.uv[p.Ry]  { self.pc = p.Br }
            case hir.OP_bne   : if       self.uv[p.Rx]  !=       self.uv[p.Ry]  { self.pc = p.Br }
            case hir.OP_blt   : if int64(self.uv[p.Rx]) <  int64(self.uv[p.Ry]) { self.pc = p.Br }
            case hir.OP_bltu  : if       self.uv[p.Rx]  <        self.uv[p.Ry]  { self.pc = p.Br }
            case hir.OP_bgeu  : if       self.uv[p.Rx]  >=       self.uv[p.Ry]  { self.pc = p.Br }
            case hir.OP_beqn  : if       self.pv[p.Ps]  ==                 nil  { self.pc = p.Br }
            case hir.OP_bnen  : if       self.pv[p.Ps]  !=                 nil  { self.pc = p.Br }
            case hir.OP_jmp   : self.pc = p.Br
            case hir.OP_bzero : memclrNoHeapPointers(self.pv[p.Pd], uintptr(p.Iv))
            case hir.OP_bcopy : memmove(self.pv[p.Pd], self.pv[p.Ps], uintptr(self.uv[p.Rx]))
            case hir.OP_break : self.trap()

            /* call to C / Go / Go interface functions */
            case hir.OP_ccall: fallthrough
            case hir.OP_gcall: fallthrough
            case hir.OP_icall: hir.LookupCall(p.Iv).Call(self, p)

            /* bit test and set */
            case hir.OP_bts: {
                self.uv[p.Rz] = bool2u64(self.uv[p.Ry] & (1 << self.uv[p.Rx]) != 0)
                self.uv[p.Ry] |= 1 << self.uv[p.Rx]
            }

            /* table switch */
            case hir.OP_bsw: {
                if v = self.uv[p.Rx]; v < uint64(p.Iv) {
                    if q = *(**hir.Ir)(unsafe.Pointer(uintptr(p.Pr) + uintptr(v) * 8)); q != nil {
                        self.pc = q
                    }
                }
            }

            /* return from function */
            case hir.OP_ret: {
                for i, self.pc = 0, nil; i < p.Rn; i++ {
                    if r := p.Rr[i]; r & hir.ArgPointer == 0 {
                        self.rv[i].U = self.uv[r & hir.ArgMask]
                    } else {
                        self.rv[i].P = self.pv[r & hir.ArgMask]
                    }
                }
            }
        }
    }

    /* check for exceptions */
    if self.pc != nil {
        panic(fmt.Sprintf("illegal OpCode: %#02x", self.pc.Op))
    }
}


func (self *Emulator) Free() {
    emulatorPool.Put(self)
}

func (self *Emulator) Reset(p hir.Program) *Emulator {
    *self = Emulator{pc: p.Head}
    return self
}

/** Implementation of ir.CallState **/

func (self *Emulator) Gr(id hir.GenericRegister) uint64 {
    return self.uv[id]
}

func (self *Emulator) Pr(id hir.PointerRegister) unsafe.Pointer {
    return self.pv[id]
}

func (self *Emulator) SetGr(id hir.GenericRegister, val uint64) {
    self.uv[id] = val
}

func (self *Emulator) SetPr(id hir.PointerRegister, val unsafe.Pointer) {
    self.pv[id] = val
}

/** State Dumping **/

const _F_emulator = `Emulator {
    pc  (%p)%s
    r0  %#x
    r1  %#x
    r2  %#x
    r3  %#x
    r4  %#x
    r5  %#x
   ----
    p0  %p
    p1  %p
    p2  %p
    p3  %p
    p4  %p
    p5  %p
    p6  %p
}`

func (self *Emulator) String() string {
    return fmt.Sprintf(
        _F_emulator,
        self.pc,
        self.pc.Disassemble(nil),
        self.uv[0],
        self.uv[1],
        self.uv[2],
        self.uv[3],
        self.uv[4],
        self.uv[5],
        self.pv[0],
        self.pv[1],
        self.pv[2],
        self.pv[3],
        self.pv[4],
        self.pv[5],
        self.pv[6],
    )
}
