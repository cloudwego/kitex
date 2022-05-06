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
    `strconv`
    `strings`
    `unsafe`

    `github.com/cloudwego/frugal/internal/rt`
)

const (
    _LB_jump_pc = "_jump_pc_"
)

type Builder struct {
    i     int
    head  *Ir
    tail  *Ir
    refs  map[string]*Ir
    pends map[string][]**Ir
}

func CreateBuilder() *Builder {
    return newBuilder()
}

func (self *Builder) add(ins *Ir) *Ir {
    self.push(ins)
    return ins
}

func (self *Builder) jmp(p *Ir, to string) *Ir {
    ok := false
    lb := strings.ReplaceAll(to, "{n}", strconv.Itoa(self.i))

    /* check for backward jumps */
    if p.Br, ok = self.refs[lb]; !ok {
        self.pends[lb] = append(self.pends[lb], &p.Br)
    }

    /* add to instruction buffer */
    return self.add(p)
}

func (self *Builder) tab(p *Ir, sw []string) *Ir {
    nb := len(sw)
    sb := make([]*Ir, nb)

    /* patch each branch */
    for i, to := range sw {
        ok := false
        lb := strings.ReplaceAll(to, "{n}", strconv.Itoa(self.i))

        /* check for backward jumps */
        if lb != "" {
            if sb[i], ok = self.refs[lb]; !ok {
                self.pends[lb] = append(self.pends[lb], &sb[i])
            }
        }
    }

    /* save the switch table */
    p.Iv = int64(nb)
    p.Pr = (*rt.GoSlice)(unsafe.Pointer(&sb)).Ptr
    return self.add(p)
}

func (self *Builder) push(ins *Ir) {
    if self.head == nil {
        self.head, self.tail = ins, ins
    } else {
        self.tail.Ln, self.tail = ins, ins
    }
}

func (self *Builder) rejmp(br **Ir) {
    for *br != nil && (*br).Ln != nil && (*br).Op == OP_nop {
        *br = (*br).Ln
    }
}

func (self *Builder) At(pc int) string {
    return _LB_jump_pc + strconv.Itoa(pc)
}

func (self *Builder) Mark(pc int) {
    self.i++
    self.Label(self.At(pc))
}

func (self *Builder) Label(to string) {
    ok := false
    lb := strings.ReplaceAll(to, "{n}", strconv.Itoa(self.i))

    /* check for duplications */
    if _, ok = self.refs[lb]; ok {
        panic("label " + lb + " has already been linked")
    }

    /* get the pending links */
    p := self.NOP()
    v := self.pends[lb]

    /* patch all the pending jumps */
    for _, q := range v {
        *q = p
    }

    /* mark the label as resolved */
    self.refs[lb] = p
    delete(self.pends, lb)
}

func (self *Builder) Build() (r Program) {
    var n int
    var p *Ir
    var q *Ir

    /* check for unresolved labels */
    for key := range self.pends {
        panic("labels are not fully resolved: " + key)
    }

    /* adjust jumps to point at actual instructions */
    for p = self.head; p != nil; p = p.Ln {
        if p.IsBranch() {
            if p.Op != OP_bsw {
                self.rejmp(&p.Br)
            } else {
                for i := int64(0); i < p.Iv * 8; i += 8 {
                    self.rejmp((**Ir)(unsafe.Pointer(uintptr(p.Pr) + uintptr(i))))
                }
            }
        }
    }

    /* remove NOPs at the front */
    for self.head != nil && self.head.Op == OP_nop {
        self.head = self.head.Ln
    }

    /* no instructions left, the program was composed entirely by NOPs */
    if self.head == nil {
        self.tail = nil
        return
    }

    /* remove all the NOPs, there should be no jumps pointing to any NOPs */
    for p = self.head; p != nil; p, n = p.Ln, n + 1 {
        for p.Ln != nil && p.Ln.Op == OP_nop {
            q = p.Ln
            p.Ln = q.Ln
            q.Free()
        }
    }

    /* the Builder's life-time ends here */
    r.Head = self.head
    freeBuilder(self)
    return
}

func (self *Builder) Append(p *Ir) (r *Ir) {
    self.push(p)
    r = self.head
    freeBuilder(self)
    return
}

func (self *Builder) NOP() *Ir {
    return self.add(newInstr(OP_nop))
}

func (self *Builder) IB(v int8, rx GenericRegister) *Ir {
    return self.ADDI(Rz, int64(v), rx)
}

func (self *Builder) IW(v int16, rx GenericRegister) *Ir {
    return self.ADDI(Rz, int64(v), rx)
}

func (self *Builder) IL(v int32, rx GenericRegister) *Ir {
    return self.ADDI(Rz, int64(v), rx)
}

func (self *Builder) IQ(v int64, rx GenericRegister) *Ir {
    return self.ADDI(Rz, v, rx)
}

func (self *Builder) IP(v interface{}, pd PointerRegister) *Ir {
    if vv := rt.UnpackEface(v); !rt.IsPtr(vv.Type) {
        panic("v is not a pointer")
    } else {
        return self.add(newInstr(OP_ip).pr(vv.Value).pd(pd))
    }
}

func (self *Builder) LB(ps PointerRegister, disp int64, rx GenericRegister) *Ir {
    return self.add(newInstr(OP_lb).ps(ps).iv(disp).rx(rx))
}

func (self *Builder) LW(ps PointerRegister, disp int64, rx GenericRegister) *Ir {
    return self.add(newInstr(OP_lw).ps(ps).iv(disp).rx(rx))
}

func (self *Builder) LL(ps PointerRegister, disp int64, rx GenericRegister) *Ir {
    return self.add(newInstr(OP_ll).ps(ps).iv(disp).rx(rx))
}

func (self *Builder) LQ(ps PointerRegister, disp int64, rx GenericRegister) *Ir {
    return self.add(newInstr(OP_lq).ps(ps).iv(disp).rx(rx))
}

func (self *Builder) LP(ps PointerRegister, disp int64, pd PointerRegister) *Ir {
    return self.add(newInstr(OP_lp).ps(ps).iv(disp).pd(pd))
}

func (self *Builder) SB(rx GenericRegister, pd PointerRegister, disp int64) *Ir {
    return self.add(newInstr(OP_sb).rx(rx).pd(pd).iv(disp))
}

func (self *Builder) SW(rx GenericRegister, pd PointerRegister, disp int64) *Ir {
    return self.add(newInstr(OP_sw).rx(rx).pd(pd).iv(disp))
}

func (self *Builder) SL(rx GenericRegister, pd PointerRegister, disp int64) *Ir {
    return self.add(newInstr(OP_sl).rx(rx).pd(pd).iv(disp))
}

func (self *Builder) SQ(rx GenericRegister, pd PointerRegister, disp int64) *Ir {
    return self.add(newInstr(OP_sq).rx(rx).pd(pd).iv(disp))
}

func (self *Builder) SP(ps PointerRegister, pd PointerRegister, disp int64) *Ir {
    return self.add(newInstr(OP_sp).ps(ps).pd(pd).iv(disp))
}

func (self *Builder) MOV(rx GenericRegister, ry GenericRegister) *Ir {
    return self.ADD(rx, Rz, ry)
}

func (self *Builder) MOVP(ps PointerRegister, pd PointerRegister) *Ir {
    return self.ADDP(ps, Rz, pd)
}

func (self *Builder) LDAQ(id int, rx GenericRegister) *Ir {
    return self.add(newInstr(OP_ldaq).iv(int64(id)).rx(rx))
}

func (self *Builder) LDAP(id int, pd PointerRegister) *Ir {
    return self.add(newInstr(OP_ldap).iv(int64(id)).pd(pd))
}

func (self *Builder) ADDP(ps PointerRegister, rx GenericRegister, pd PointerRegister) *Ir {
    return self.add(newInstr(OP_addp).ps(ps).rx(rx).pd(pd))
}

func (self *Builder) SUBP(ps PointerRegister, rx GenericRegister, pd PointerRegister) *Ir {
    return self.add(newInstr(OP_subp).ps(ps).rx(rx).pd(pd))
}

func (self *Builder) ADDPI(ps PointerRegister, im int64, pd PointerRegister) *Ir {
    return self.add(newInstr(OP_addpi).ps(ps).iv(im).pd(pd))
}

func (self *Builder) SUBPI(ps PointerRegister, im int64, pd PointerRegister) *Ir {
    return self.ADDPI(ps, -im, pd)
}

func (self *Builder) ADD(rx GenericRegister, ry GenericRegister, rz GenericRegister) *Ir {
    return self.add(newInstr(OP_add).rx(rx).ry(ry).rz(rz))
}

func (self *Builder) SUB(rx GenericRegister, ry GenericRegister, rz GenericRegister) *Ir {
    return self.add(newInstr(OP_sub).rx(rx).ry(ry).rz(rz))
}

func (self *Builder) BTS(rx GenericRegister, ry GenericRegister, rz GenericRegister) *Ir {
    return self.add(newInstr(OP_bts).rx(rx).ry(ry).rz(rz))
}

func (self *Builder) ADDI(rx GenericRegister, im int64, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_addi).rx(rx).iv(im).ry(ry))
}

func (self *Builder) SUBI(rx GenericRegister, im int64, ry GenericRegister) *Ir {
    return self.ADDI(rx, -im, ry)
}

func (self *Builder) MULI(rx GenericRegister, im int64, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_muli).rx(rx).iv(im).ry(ry))
}

func (self *Builder) ANDI(rx GenericRegister, im int64, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_andi).rx(rx).iv(im).ry(ry))
}

func (self *Builder) XORI(rx GenericRegister, im int64, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_xori).rx(rx).iv(im).ry(ry))
}

func (self *Builder) SHRI(rx GenericRegister, im int64, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_shri).rx(rx).iv(im).ry(ry))
}

func (self *Builder) BSI(rx GenericRegister, im int64, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_bsi).rx(rx).iv(im).ry(ry))
}

func (self *Builder) SWAPW(rx GenericRegister, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_swapw).rx(rx).ry(ry))
}

func (self *Builder) SWAPL(rx GenericRegister, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_swapl).rx(rx).ry(ry))
}

func (self *Builder) SWAPQ(rx GenericRegister, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_swapq).rx(rx).ry(ry))
}

func (self *Builder) SXLQ(rx GenericRegister, ry GenericRegister) *Ir {
    return self.add(newInstr(OP_sxlq).rx(rx).ry(ry))
}

func (self *Builder) BEQ(rx GenericRegister, ry GenericRegister, to string) *Ir {
    return self.jmp(newInstr(OP_beq).rx(rx).ry(ry), to)
}

func (self *Builder) BNE(rx GenericRegister, ry GenericRegister, to string) *Ir {
    return self.jmp(newInstr(OP_bne).rx(rx).ry(ry), to)
}

func (self *Builder) BLT(rx GenericRegister, ry GenericRegister, to string) *Ir {
    return self.jmp(newInstr(OP_blt).rx(rx).ry(ry), to)
}

func (self *Builder) BLTU(rx GenericRegister, ry GenericRegister, to string) *Ir {
    return self.jmp(newInstr(OP_bltu).rx(rx).ry(ry), to)
}

func (self *Builder) BGEU(rx GenericRegister, ry GenericRegister, to string) *Ir {
    return self.jmp(newInstr(OP_bgeu).rx(rx).ry(ry), to)
}

func (self *Builder) BSW(rx GenericRegister, sw []string) *Ir {
    return self.tab(newInstr(OP_bsw).rx(rx), sw)
}

func (self *Builder) BEQN(ps PointerRegister, to string) *Ir {
    return self.jmp(newInstr(OP_beqn).ps(ps), to)
}

func (self *Builder) BNEN(ps PointerRegister, to string) *Ir {
    return self.jmp(newInstr(OP_bnen).ps(ps), to)
}

func (self *Builder) JMP(to string) *Ir {
    return self.jmp(newInstr(OP_jmp), to)
}

func (self *Builder) BZERO(nb int64, pd PointerRegister) *Ir {
    return self.add(newInstr(OP_bzero).iv(nb).pd(pd))
}

func (self *Builder) BCOPY(ps PointerRegister, rx GenericRegister, pd PointerRegister) *Ir {
    return self.add(newInstr(OP_bcopy).ps(ps).rx(rx).pd(pd))
}

func (self *Builder) CCALL(fn *CallHandle) *Ir {
    return self.add(newInstr(OP_ccall).iv(int64(fn.Id)))
}

func (self *Builder) GCALL(fn *CallHandle) *Ir {
    return self.add(newInstr(OP_gcall).iv(int64(fn.Id)))
}

func (self *Builder) ICALL(vt PointerRegister, vp PointerRegister, mt *CallHandle) *Ir {
    return self.add(newInstr(OP_icall).ps(vt).pd(vp).iv(int64(mt.Id)))
}

func (self *Builder) RET() *Ir {
    return self.add(newInstr(OP_ret))
}

func (self *Builder) BREAK() *Ir {
    return self.add(newInstr(OP_break))
}
