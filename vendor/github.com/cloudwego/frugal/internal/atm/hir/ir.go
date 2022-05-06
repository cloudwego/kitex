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

package hir

import (
    `fmt`
    `strings`
    `unsafe`

    `github.com/cloudwego/frugal/internal/rt`
)

type OpCode byte

const (
    OP_nop OpCode = iota    // no operation
    OP_ip                   // ptr(Pr) -> Pd
    OP_lb                   // u64(*(* u8)(Ps + Iv)) -> Rx
    OP_lw                   // u64(*(*u16)(Ps + Iv)) -> Rx
    OP_ll                   // u64(*(*u32)(Ps + Iv)) -> Rx
    OP_lq                   //     *(*u64)(Ps + Iv)  -> Rx
    OP_lp                   //     *(*ptr)(Ps + Iv)  -> Pd
    OP_sb                   //  u8(Rx) -> *(*u8)(Pd + Iv)
    OP_sw                   // u16(Rx) -> *(*u16)(Pd + Iv)
    OP_sl                   // u32(Rx) -> *(*u32)(Pd + Iv)
    OP_sq                   //     Rx  -> *(*u64)(Pd + Iv)
    OP_sp                   //     Ps  -> *(*ptr)(Pd + Iv)
    OP_ldaq                 // arg[Iv] -> Rx
    OP_ldap                 // arg[Iv] -> Pd
    OP_addp                 // Ps + Rx -> Pd
    OP_subp                 // Ps - Rx -> Pd
    OP_addpi                // Ps + Iv -> Pd
    OP_add                  // Rx + Ry -> Rz
    OP_sub                  // Rx - Ry -> Rz
    OP_bts                  // Ry & (1 << (Rx % PTR_BITS)) != 0 -> Rz, Ry |= 1 << (Rx % PTR_BITS)
    OP_addi                 // Rx + Iv -> Ry
    OP_muli                 // Rx * Iv -> Ry
    OP_andi                 // Rx & Iv -> Ry
    OP_xori                 // Rx ^ Iv -> Ry
    OP_shri                 // Rx >> Iv -> Ry
    OP_bsi                  // Rx | (1 << Iv) -> Ry
    OP_swapw                // bswap16(Rx) -> Ry
    OP_swapl                // bswap32(Rx) -> Ry
    OP_swapq                // bswap64(Rx) -> Ry
    OP_sxlq                 // sign_extend_32_to_64(Rx) -> Ry
    OP_beq                  // if (Rx == Ry) Br.PC -> PC
    OP_bne                  // if (Rx != Ry) Br.PC -> PC
    OP_blt                  // if (Rx <  Ry) Br.PC -> PC
    OP_bltu                 // if (u(Rx) <  u(Ry)) Br.PC -> PC
    OP_bgeu                 // if (u(Rx) >= u(Ry)) Br.PC -> PC
    OP_bsw                  // if (u(Rx) <  u(An)) Sw[u(Rx)].PC -> PC
    OP_beqn                 // if (Ps == nil) Br.PC -> PC
    OP_bnen                 // if (Ps != nil) Br.PC -> PC
    OP_jmp                  // Br.PC -> PC
    OP_bzero                // memset(Pd, 0, Iv)
    OP_bcopy                // memcpy(Pd, Ps, Rx)
    OP_ccall                // call external C functions
    OP_gcall                // call external Go functions
    OP_icall                // call external Go iface methods
    OP_ret                  // return from function
    OP_break                // trigger a debugger breakpoint
)

type Ir struct {
    Op OpCode
    Rx GenericRegister
    Ry GenericRegister
    Rz GenericRegister
    Ps PointerRegister
    Pd PointerRegister
    An uint8
    Rn uint8
    Ar [8]uint8
    Rr [8]uint8
    Iv int64
    Pr unsafe.Pointer
    Br *Ir
    Ln *Ir
}

func (self *Ir) iv(v int64)           *Ir { self.Iv = v; return self }
func (self *Ir) pr(v unsafe.Pointer)  *Ir { self.Pr = v; return self }
func (self *Ir) rx(v GenericRegister) *Ir { self.Rx = v; return self }
func (self *Ir) ry(v GenericRegister) *Ir { self.Ry = v; return self }
func (self *Ir) rz(v GenericRegister) *Ir { self.Rz = v; return self }
func (self *Ir) ps(v PointerRegister) *Ir { self.Ps = v; return self }
func (self *Ir) pd(v PointerRegister) *Ir { self.Pd = v; return self }

func (self *Ir) A0(v Register) *Ir { self.An, self.Ar[0] = 1, v.A(); return self }
func (self *Ir) A1(v Register) *Ir { self.An, self.Ar[1] = 2, v.A(); return self }
func (self *Ir) A2(v Register) *Ir { self.An, self.Ar[2] = 3, v.A(); return self }
func (self *Ir) A3(v Register) *Ir { self.An, self.Ar[3] = 4, v.A(); return self }
func (self *Ir) A4(v Register) *Ir { self.An, self.Ar[4] = 5, v.A(); return self }
func (self *Ir) A5(v Register) *Ir { self.An, self.Ar[5] = 6, v.A(); return self }
func (self *Ir) A6(v Register) *Ir { self.An, self.Ar[6] = 7, v.A(); return self }
func (self *Ir) A7(v Register) *Ir { self.An, self.Ar[7] = 8, v.A(); return self }

func (self *Ir) R0(v Register) *Ir { self.Rn, self.Rr[0] = 1, v.A(); return self }
func (self *Ir) R1(v Register) *Ir { self.Rn, self.Rr[1] = 2, v.A(); return self }
func (self *Ir) R2(v Register) *Ir { self.Rn, self.Rr[2] = 3, v.A(); return self }
func (self *Ir) R3(v Register) *Ir { self.Rn, self.Rr[3] = 4, v.A(); return self }
func (self *Ir) R4(v Register) *Ir { self.Rn, self.Rr[4] = 5, v.A(); return self }
func (self *Ir) R5(v Register) *Ir { self.Rn, self.Rr[5] = 6, v.A(); return self }
func (self *Ir) R6(v Register) *Ir { self.Rn, self.Rr[6] = 7, v.A(); return self }
func (self *Ir) R7(v Register) *Ir { self.Rn, self.Rr[7] = 8, v.A(); return self }

func (self *Ir) Sw() (p []*Ir) {
    (*rt.GoSlice)(unsafe.Pointer(&p)).Ptr = self.Pr
    (*rt.GoSlice)(unsafe.Pointer(&p)).Len = int(self.Iv)
    (*rt.GoSlice)(unsafe.Pointer(&p)).Cap = int(self.Iv)
    return
}

func (self *Ir) Free() {
    freeInstr(self)
}

func (self *Ir) IsBranch() bool {
    return self.Op >= OP_beq && self.Op <= OP_jmp
}

func (self *Ir) formatCall() string {
    return fmt.Sprintf(
        "%s, %s",
        self.formatArgs(&self.Ar, self.An),
        self.formatArgs(&self.Rr, self.Rn),
    )
}

func (self *Ir) formatArgs(vv *[8]uint8, nb uint8) string {
    i := uint8(0)
    ret := make([]string, nb)

    /* add each register */
    for i = 0; i < nb; i++ {
        if v := vv[i]; (v & ArgPointer) == 0 {
            ret[i] = "%" + GenericRegister(v & ArgMask).String()
        } else {
            ret[i] = "%" + PointerRegister(v & ArgMask).String()
        }
    }

    /* compose the result */
    return fmt.Sprintf(
        "{%s}",
        strings.Join(ret, ", "),
    )
}

func (self *Ir) formatRefs(refs map[*Ir]string, v *Ir) string {
    if vv, ok := refs[v]; ok {
        return vv
    } else {
        return fmt.Sprintf("@%p", v)
    }
}

func (self *Ir) formatTable(refs map[*Ir]string) string {
    tab := self.Sw()
    ret := make([]string, 0, self.Iv)

    /* empty table */
    if self.Iv == 0 {
        return "{}"
    }

    /* format every label */
    for i, lb := range tab {
        if lb != nil {
            ret = append(ret, fmt.Sprintf("%4ccase $%d: %s,\n", ' ', i, self.formatRefs(refs, lb)))
        }
    }

    /* join them together */
    return fmt.Sprintf(
        "{\n%s}",
        strings.Join(ret, ""),
    )
}

func (self *Ir) Disassemble(refs map[*Ir]string) string {
    switch self.Op {
        case OP_nop   : return "nop"
        case OP_ip    : return fmt.Sprintf("ip      $%p, %%%s", self.Pr, self.Pd)
        case OP_lb    : return fmt.Sprintf("lb      %d(%%%s), %%%s", self.Iv, self.Ps, self.Rx)
        case OP_lw    : return fmt.Sprintf("lw      %d(%%%s), %%%s", self.Iv, self.Ps, self.Rx)
        case OP_ll    : return fmt.Sprintf("ll      %d(%%%s), %%%s", self.Iv, self.Ps, self.Rx)
        case OP_lq    : return fmt.Sprintf("lq      %d(%%%s), %%%s", self.Iv, self.Ps, self.Rx)
        case OP_lp    : return fmt.Sprintf("lp      %d(%%%s), %%%s", self.Iv, self.Ps, self.Pd)
        case OP_sb    : return fmt.Sprintf("sb      %%%s, %d(%%%s)", self.Rx, self.Iv, self.Pd)
        case OP_sw    : return fmt.Sprintf("sw      %%%s, %d(%%%s)", self.Rx, self.Iv, self.Pd)
        case OP_sl    : return fmt.Sprintf("sl      %%%s, %d(%%%s)", self.Rx, self.Iv, self.Pd)
        case OP_sq    : return fmt.Sprintf("sq      %%%s, %d(%%%s)", self.Rx, self.Iv, self.Pd)
        case OP_sp    : return fmt.Sprintf("sp      %%%s, %d(%%%s)", self.Ps, self.Iv, self.Pd)
        case OP_ldaq  : return fmt.Sprintf("ldaq    $%d, %%%s", self.Iv, self.Rx)
        case OP_ldap  : return fmt.Sprintf("ldap    $%d, %%%s", self.Iv, self.Pd)
        case OP_addp  : return fmt.Sprintf("addp    %%%s, %%%s, %%%s", self.Ps, self.Rx, self.Pd)
        case OP_subp  : return fmt.Sprintf("subp    %%%s, %%%s, %%%s", self.Ps, self.Rx, self.Pd)
        case OP_addpi : return fmt.Sprintf("addpi   %%%s, $%d, %%%s", self.Ps, self.Iv, self.Pd)
        case OP_add   : return fmt.Sprintf("add     %%%s, %%%s, %%%s", self.Rx, self.Ry, self.Rz)
        case OP_sub   : return fmt.Sprintf("sub     %%%s, %%%s, %%%s", self.Rx, self.Ry, self.Rz)
        case OP_bts   : return fmt.Sprintf("bts     %%%s, %%%s, %%%s", self.Rx, self.Ry, self.Rz)
        case OP_addi  : return fmt.Sprintf("addi    %%%s, $%d, %%%s", self.Rx, self.Iv, self.Ry)
        case OP_muli  : return fmt.Sprintf("muli    %%%s, $%d, %%%s", self.Rx, self.Iv, self.Ry)
        case OP_andi  : return fmt.Sprintf("andi    %%%s, $%d, %%%s", self.Rx, self.Iv, self.Ry)
        case OP_xori  : return fmt.Sprintf("xori    %%%s, $%d, %%%s", self.Rx, self.Iv, self.Ry)
        case OP_shri  : return fmt.Sprintf("shri    %%%s, $%d, %%%s", self.Rx, self.Iv, self.Ry)
        case OP_bsi   : return fmt.Sprintf("bsi     %%%s, $%d, %%%s", self.Rx, self.Iv, self.Ry)
        case OP_swapw : return fmt.Sprintf("swapw   %%%s, %%%s", self.Rx, self.Ry)
        case OP_swapl : return fmt.Sprintf("swapl   %%%s, %%%s", self.Rx, self.Ry)
        case OP_swapq : return fmt.Sprintf("swapq   %%%s, %%%s", self.Rx, self.Ry)
        case OP_sxlq  : return fmt.Sprintf("sxlq    %%%s, %%%s", self.Rx, self.Ry)
        case OP_beq   : return fmt.Sprintf("beq     %%%s, %%%s, %s", self.Rx, self.Ry, self.formatRefs(refs, self.Br))
        case OP_bne   : return fmt.Sprintf("bne     %%%s, %%%s, %s", self.Rx, self.Ry, self.formatRefs(refs, self.Br))
        case OP_blt   : return fmt.Sprintf("blt     %%%s, %%%s, %s", self.Rx, self.Ry, self.formatRefs(refs, self.Br))
        case OP_bltu  : return fmt.Sprintf("bltu    %%%s, %%%s, %s", self.Rx, self.Ry, self.formatRefs(refs, self.Br))
        case OP_bgeu  : return fmt.Sprintf("bgeu    %%%s, %%%s, %s", self.Rx, self.Ry, self.formatRefs(refs, self.Br))
        case OP_bsw   : return fmt.Sprintf("bsw     %%%s, %s", self.Rx, self.formatTable(refs))
        case OP_beqn  : return fmt.Sprintf("beq     %%%s, %%nil, %s", self.Ps, self.formatRefs(refs, self.Br))
        case OP_bnen  : return fmt.Sprintf("bne     %%%s, %%nil, %s", self.Ps, self.formatRefs(refs, self.Br))
        case OP_jmp   : return fmt.Sprintf("jmp     %s", self.formatRefs(refs, self.Br))
        case OP_bzero : return fmt.Sprintf("bzero   $%d, %s", self.Iv, self.Pd)
        case OP_bcopy : return fmt.Sprintf("bcopy   %s, %s, %s", self.Ps, self.Rx, self.Pd)
        case OP_ccall : return fmt.Sprintf("ccall   %s, %s", LookupCall(self.Iv), self.formatCall())
        case OP_gcall : return fmt.Sprintf("gcall   %s, %s", LookupCall(self.Iv), self.formatCall())
        case OP_icall : return fmt.Sprintf("icall   $%d, {%%%s, %%%s}, %s", LookupCall(self.Iv).Slot, self.Ps, self.Pd, self.formatCall())
        case OP_ret   : return fmt.Sprintf("ret     %s", self.formatArgs(&self.Rr, self.Rn))
        case OP_break : return "break"
        default       : panic(fmt.Sprintf("invalid OpCode: 0x%02x", self.Op))
    }
}
