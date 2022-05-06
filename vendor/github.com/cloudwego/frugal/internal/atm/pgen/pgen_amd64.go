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
    `fmt`
    `math`
    `math/bits`
    `reflect`
    `sort`
    `sync/atomic`

    `github.com/chenzhuoyu/iasm/expr`
    `github.com/chenzhuoyu/iasm/x86_64`
    `github.com/cloudwego/frugal/internal/atm/abi`
    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/rt`
)

type _SwitchTable struct {
    ref *x86_64.Label
    tab []*x86_64.Label
}

var (
    stabCount uint64
)

func newSwitchTable(n int) (v _SwitchTable) {
    return _SwitchTable {
        tab: make([]*x86_64.Label, n),
        ref: x86_64.CreateLabel(fmt.Sprintf("_table_%d", atomic.AddUint64(&stabCount, 1))),
    }
}

func (self *_SwitchTable) link(p *x86_64.Program) {
    p.Link(self.ref)
    self.refs(p, self.ref)
}

func (self *_SwitchTable) mark(i int, to *x86_64.Label) {
    if i >= len(self.tab) {
        panic("pgen: stab: index out of bound")
    } else {
        self.tab[i] = to
    }
}

func (self *_SwitchTable) refs(p *x86_64.Program, to *x86_64.Label) {
    for _, v := range self.tab {
        p.Long(expr.Ref(v).Sub(expr.Ref(to)))
    }
}

type _DeferBlock struct {
    ref *x86_64.Label
    def func(p *x86_64.Program)
}

type _RegSeq []hir.Register
func (self _RegSeq) Len() int               { return len(self) }
func (self _RegSeq) Swap(i int, j int)      { self[i], self[j] = self[j], self[i] }
func (self _RegSeq) Less(i int, j int) bool { return self[i].A() < self[j].A() }

/** Frame Structure of the Generated Function
 *
 *                 (Previous Frame)
 *      prev() ------------------------
 *                    Return PC
 *      size() ------------------------
 *                    Saved RBP             |
 *      offs() ------------------------     |
 *                Reserved Registers        | (decrease)
 *      rsvd() ------------------------     |
 *                   Spill Slots            ↓
 *      save() ------------------------
 *                Outgoing Arguments
 *         RSP ------------------------
 */

type _FrameInfo struct {
    alen int
    regs _RegSeq
    desc *abi.FunctionLayout
    regi map[hir.Register]int32
    regr map[x86_64.Register64]int32
}

func (self *_FrameInfo) regc() int {
    return len(self.regs)
}

func (self *_FrameInfo) argc() int {
    return len(self.desc.Args)
}

func (self *_FrameInfo) retc() int {
    return len(self.desc.Rets)
}

func (self *_FrameInfo) save() int32 {
    return int32(self.alen)
}

func (self *_FrameInfo) prev() int32 {
    return self.size() + abi.PtrSize
}

func (self *_FrameInfo) size() int32 {
    return self.offs() + abi.PtrSize
}

func (self *_FrameInfo) offs() int32 {
    return self.rsvd() + int32(len(self.regr)) *abi.PtrSize
}

func (self *_FrameInfo) rsvd() int32 {
    return self.save() + int32(len(self.regs)) *abi.PtrSize
}

func (self *_FrameInfo) argv(i int) *x86_64.MemoryOperand {
    return Ptr(RSP, self.prev() + int32(self.desc.Args[i].Mem))
}

func (self *_FrameInfo) retv(i int) *x86_64.MemoryOperand {
    return Ptr(RSP, self.prev() + int32(self.desc.Rets[i].Mem))
}

func (self *_FrameInfo) slot(r hir.Register) *x86_64.MemoryOperand {
    return Ptr(RSP, self.save() + self.regi[r] *abi.PtrSize)
}

func (self *_FrameInfo) rslot(r x86_64.Register64) *x86_64.MemoryOperand {
    return Ptr(RSP, self.rsvd() + self.regr[r] *abi.PtrSize)
}

func (self *_FrameInfo) ralloc(r hir.Register) {
    self.regs = append(self.regs, r)
    sort.Sort(self.regs)

    /* assign slots in ascending order */
    for i, v := range self.regs {
        self.regi[v] = int32(i)
    }
}

func (self *_FrameInfo) require(n uintptr) {
    if self.alen < int(n) {
        self.alen = int(n)
    }
}

func (self *_FrameInfo) ArgPtrs() *rt.StackMap {
    return self.desc.StackMap()
}

func (self *_FrameInfo) LocalPtrs() *rt.StackMap {
    var v hir.Register
    var m rt.StackMapBuilder

    /* register spill slots */
    for _, v = range self.regs {
        m.AddField(v.A() & hir.ArgPointer != 0)
    }

    /* reserved registers */
    m.AddFields(len(self.regr), false)
    return m.Build()
}

func newContext(proto interface{}) (ret _FrameInfo) {
    vt := reflect.TypeOf(proto)
    vk := vt.Kind()

    /* must be a function */
    if vk != reflect.Func {
        panic("pgen: proto must be a function")
    }

    /* layout the function */
    ret.regr = abi.ABI.Reserved()
    ret.desc = abi.ABI.LayoutFunc(-1, vt)
    ret.regi = make(map[hir.Register]int32)
    return
}

type (
    Operands uint16
)

const (
    Orx Operands = 1 << iota    // read Rx register
    Ory                         // read Ry register
    Owx                         // write Rx register
    Owy                         // write Ry register
    Owz                         // write ir.Rz register
    Ops                         // read Ps register
    Opd                         // write Pd register
    Ojmp                        // unconditional jumps
    Oret                        // function returns
    Ocall                       // function calls
    Octrl                       // control OpCodes
)

var _OperandMask = [256]Operands {
    hir.OP_nop   : Octrl,
    hir.OP_ip    : Opd,
    hir.OP_lb    : Owx | Ops,
    hir.OP_lw    : Owx | Ops,
    hir.OP_ll    : Owx | Ops,
    hir.OP_lq    : Owx | Ops,
    hir.OP_lp    : Ops | Opd,
    hir.OP_sb    : Orx | Opd,
    hir.OP_sw    : Orx | Opd,
    hir.OP_sl    : Orx | Opd,
    hir.OP_sq    : Orx | Opd,
    hir.OP_sp    : Ops | Opd,
    hir.OP_ldaq  : Owx,
    hir.OP_ldap  : Opd,
    hir.OP_addp  : Orx | Ops | Opd,
    hir.OP_subp  : Orx | Ops | Opd,
    hir.OP_addpi : Ops | Opd,
    hir.OP_add   : Orx | Ory | Owz,
    hir.OP_sub   : Orx | Ory | Owz,
    hir.OP_bts   : Orx | Ory | Owy | Owz,
    hir.OP_addi  : Orx | Owy,
    hir.OP_muli  : Orx | Owy,
    hir.OP_andi  : Orx | Owy,
    hir.OP_xori  : Orx | Owy,
    hir.OP_shri  : Orx | Owy,
    hir.OP_bsi   : Orx | Owy,
    hir.OP_swapw : Orx | Owy,
    hir.OP_swapl : Orx | Owy,
    hir.OP_swapq : Orx | Owy,
    hir.OP_sxlq  : Orx | Owy,
    hir.OP_beq   : Orx | Ory,
    hir.OP_bne   : Orx | Ory,
    hir.OP_blt   : Orx | Ory,
    hir.OP_bltu  : Orx | Ory,
    hir.OP_bgeu  : Orx | Ory,
    hir.OP_bsw   : Orx,
    hir.OP_beqn  : Ops,
    hir.OP_bnen  : Ops,
    hir.OP_jmp   : Ojmp,
    hir.OP_bzero : Opd,
    hir.OP_bcopy : Orx | Ops | Opd,
    hir.OP_ccall : Ocall,
    hir.OP_gcall : Ocall,
    hir.OP_icall : Ocall,
    hir.OP_ret   : Oret,
    hir.OP_break : Octrl,
}

type Func struct {
    Code  []byte
    Frame rt.Frame
}

type CodeGen struct {
    regi int
    ctxt _FrameInfo
    arch *x86_64.Arch
    head *x86_64.Label
    tail *x86_64.Label
    halt *x86_64.Label
    defs []_DeferBlock
    stab []_SwitchTable
    abix _CodeGenExtension
    jmps map[string]*x86_64.Label
    regs map[hir.Register]x86_64.Register64
}

func CreateCodeGen(proto interface{}) *CodeGen {
    return &CodeGen {
        ctxt: newContext(proto),
        arch: x86_64.DefaultArch,
        jmps: make(map[string]*x86_64.Label),
        regs: make(map[hir.Register]x86_64.Register64),
    }
}

func (self *CodeGen) Generate(s hir.Program, sp uintptr) *Func {
    h := 0
    p := self.arch.CreateProgram()

    /* find the halting points */
    for v := s.Head; h < 2 && v != nil; v = v.Ln {
        if v.Op == hir.OP_ret {
            h++
        }
    }

    /* program must halt exactly once */
    switch h {
        case 1  : break
        case 0  : panic("pgen: program does not halt")
        default : panic("pgen: program halts more than once")
    }

    /* static register allocation */
    for v := s.Head; v != nil; v = v.Ln {
        self.rcheck(v, _OperandMask[v.Op])
        self.walloc(v, _OperandMask[v.Op])
    }

    /* argument space calculation */
    for v := s.Head; v != nil; v = v.Ln {
        switch v.Op {
            case hir.OP_gcall: fallthrough
            case hir.OP_icall: self.ctxt.require(abi.ABI.FnTab[hir.LookupCall(v.Iv).Id].Sp)
            case hir.OP_bcopy: self.ctxt.require(_M_memcpyargs)
        }
    }

    /* create the labels for stack management */
    entry := x86_64.CreateLabel("_entry")
    stack := x86_64.CreateLabel("_stack_grow")

    /* create key anchor points */
    self.head = x86_64.CreateLabel("_head")
    self.tail = x86_64.CreateLabel("_tail")
    self.halt = x86_64.CreateLabel("_halt")

    /* stack checking */
    p.Link(entry)
    self.abiStackCheck(p, stack, sp)

    /* program prologue */
    p.SUBQ(self.ctxt.size(), RSP)
    p.Link(self.head)
    p.MOVQ(RBP, Ptr(RSP, self.ctxt.offs()))
    p.LEAQ(Ptr(RSP, self.ctxt.offs()), RBP)

    /* ABI-specific prologue */
    self.abiSaveReserved(p)
    self.abiPrologue(p)

    /* clear all the pointer registers */
    for lr := range self.regs {
        if lr.A() & hir.ArgPointer != 0 {
            self.clr(p, lr)
        }
    }

    /* clear all the spill slots, if any */
    if i, n := 0, self.ctxt.regc(); n != 0 {
        if  n >= 2 { p.PXOR   (XMM15, XMM15) }
        for n >= 2 { p.MOVDQU (XMM15, Ptr(RSP, self.ctxt.save() + int32(i) *abi.PtrSize)); i += 2; n -= 2 }
        if  n != 0 { p.MOVQ   (0, Ptr(RSP, self.ctxt.save() + int32(i) *abi.PtrSize)) }
    }

    /* translate the entire program */
    for v := s.Head; v != nil; v = v.Ln {
        self.translate(p, v)
    }

    /* generate all defered blocks */
    for _, fp := range self.defs {
        p.Link(fp.ref)
        fp.def(p)
    }

    /* ABI-specific epilogue */
    p.Link(self.halt)
    self.abiEpilogue(p)
    self.abiLoadReserved(p)

    /* program epilogue */
    p.MOVQ(Ptr(RSP, self.ctxt.offs()), RBP)
    p.ADDQ(self.ctxt.size(), RSP)
    p.Link(self.tail)
    p.RET()

    /* stack grow */
    p.Link(stack)
    self.abiStackGrow(p)
    p.JMP(entry)

    /* link all the lookup tables */
    for _, v := range self.stab {
        v.link(p)
    }

    /* assemble the function */
    ret := &Func {
        Code  : p.Assemble(0),
        Frame : rt.Frame {
            Head      : toAddress(self.head),
            Tail      : toAddress(self.tail),
            Size      : int(self.ctxt.size()),
            ArgSize   : int(self.ctxt.save()),
            ArgPtrs   : self.ctxt.ArgPtrs(),
            LocalPtrs : self.ctxt.LocalPtrs(),
        },
    }

    /* free the assembler */
    p.Free()
    return ret
}

func (self *CodeGen) later(ref *x86_64.Label, def func(*x86_64.Program)) {
    self.defs = append(self.defs, _DeferBlock {
        ref: ref,
        def: def,
    })
}

func (self *CodeGen) translate(p *x86_64.Program, v *hir.Ir) {
    if p.Link(self.to(v)); v.Op != hir.OP_nop {
        if fp := translators[v.Op]; fp != nil {
            fp(self, p, v)
        } else {
            panic("pgen: invalid instruction: " + v.Disassemble(nil))
        }
    }
}

/** Register Allocation **/

type _Check struct {
    bv Operands
    fn func(*hir.Ir) hir.Register
}

var _readChecks = [...]_Check {
    { bv: Orx, fn: func(p *hir.Ir) hir.Register { return p.Rx } },
    { bv: Ory, fn: func(p *hir.Ir) hir.Register { return p.Ry } },
    { bv: Ops, fn: func(p *hir.Ir) hir.Register { return p.Ps } },
}

var _writeAllocs = [...]_Check {
    { bv: Owx, fn: func(p *hir.Ir) hir.Register { return p.Rx } },
    { bv: Owy, fn: func(p *hir.Ir) hir.Register { return p.Ry } },
    { bv: Owz, fn: func(p *hir.Ir) hir.Register { return p.Rz } },
    { bv: Opd, fn: func(p *hir.Ir) hir.Register { return p.Pd } },
}

func (self *CodeGen) rload(v *hir.Ir, r hir.Register) {
    if _, ok := self.regs[r]; !ok && !r.Z() {
        panic(fmt.Sprintf("pgen: access to unallocated register %s: %s", r.String(), v.Disassemble(nil)))
    }
}

func (self *CodeGen) rstore(v *hir.Ir, r hir.Register) {
    if _, ok := self.regs[r]; !ok && !r.Z() {
        if self.ctxt.ralloc(r); self.regi < len(allocationOrder) {
            self.regi, self.regs[r] = self.regi + 1, allocationOrder[self.regi]
        } else {
            panic("pgen: program is too complex to translate on x86_64 (requiring too many registers): " + v.Disassemble(nil))
        }
    }
}

func (self *CodeGen) ldret(v *hir.Ir) {
    for i := 0; i < int(v.Rn); i++ {
        if r := v.Rr[i]; r & hir.ArgPointer == 0 {
            self.rload(v, hir.GenericRegister(r))
        } else {
            self.rload(v, hir.PointerRegister(r & hir.ArgMask))
        }
    }
}

func (self *CodeGen) ldcall(v *hir.Ir) {
    for i := 0; i < int(v.An); i++ {
        if r := v.Ar[i]; r & hir.ArgPointer == 0 {
            self.rload(v, hir.GenericRegister(r))
        } else {
            self.rload(v, hir.PointerRegister(r & hir.ArgMask))
        }
    }
}

func (self *CodeGen) stcall(v *hir.Ir) {
    for i := 0; i < int(v.Rn); i++ {
        if r := v.Rr[i]; r & hir.ArgPointer == 0 {
            self.rstore(v, hir.GenericRegister(r))
        } else {
            self.rstore(v, hir.PointerRegister(r & hir.ArgMask))
        }
    }
}

func (self *CodeGen) rcheck(v *hir.Ir, p Operands) {
    for _, cc := range _readChecks {
        if p & Oret  != 0 { self.ldret(v) }
        if p & Ocall != 0 { self.ldcall(v) }
        if p & cc.bv != 0 { self.rload(v, cc.fn(v)) }
    }
}

func (self *CodeGen) walloc(v *hir.Ir, p Operands) {
    for _, cc := range _writeAllocs {
        if p & Ocall != 0 { self.stcall(v) }
        if p & cc.bv != 0 { self.rstore(v, cc.fn(v)) }
    }
}

func (self *CodeGen) rindex(r x86_64.Register64) hir.Register {
    for k, v := range self.regs { if v == r { return k } }
    return nil
}

/** Generator Helpers **/

func (self *CodeGen) r(reg hir.Register) x86_64.Register64 {
    if rr, ok := self.regs[reg]; !ok {
        panic("pgen: access to unallocated register: " + reg.String())
    } else {
        return rr
    }
}

func (self *CodeGen) to(v *hir.Ir) *x86_64.Label {
    return self.ref(fmt.Sprintf("_PC_%p", v))
}

func (self *CodeGen) tab(i int64) *_SwitchTable {
    p := len(self.stab)
    self.stab = append(self.stab, newSwitchTable(int(i)))
    return &self.stab[p]
}

func (self *CodeGen) ref(s string) *x86_64.Label {
    var k bool
    var p *x86_64.Label

    /* check for existance */
    if p, k = self.jmps[s]; k {
        return p
    }

    /* create a new label if not */
    p = x86_64.CreateLabel(s)
    self.jmps[s] = p
    return p
}

func (self *CodeGen) i32(p *x86_64.Program, v *hir.Ir) interface{} {
    if isInt32(v.Iv) {
        return v.Iv
    } else {
        p.MOVQ(v.Iv, RAX)
        return RAX
    }
}

func (self *CodeGen) ptr(p *x86_64.Program, r hir.PointerRegister, d int64) *x86_64.MemoryOperand {
    if isInt32(d) {
        return Ptr(self.r(r), int32(d))
    } else {
        p.MOVQ(d, RAX)
        return Sib(self.r(r), RAX, 1, 0)
    }
}

func (self *CodeGen) clr(p *x86_64.Program, r hir.Register) {
    rx := self.r(r)
    p.XORL(x86_64.Register32(rx), x86_64.Register32(rx))
}

func (self *CodeGen) set(p *x86_64.Program, r hir.Register, i int64) {
    if i == 0 {
        self.clr(p, r)
    } else {
        p.MOVQ(i, self.r(r))
    }
}

func (self *CodeGen) dup(p *x86_64.Program, r hir.Register, d hir.Register) {
    if r != d {
        p.MOVQ(self.r(r), self.r(d))
    }
}

/** OpCode Generators **/

var translators = [256]func(*CodeGen, *x86_64.Program, *hir.Ir) {
    hir.OP_ip    : (*CodeGen).translate_OP_ip,
    hir.OP_lb    : (*CodeGen).translate_OP_lb,
    hir.OP_lw    : (*CodeGen).translate_OP_lw,
    hir.OP_ll    : (*CodeGen).translate_OP_ll,
    hir.OP_lq    : (*CodeGen).translate_OP_lq,
    hir.OP_lp    : (*CodeGen).translate_OP_lp,
    hir.OP_sb    : (*CodeGen).translate_OP_sb,
    hir.OP_sw    : (*CodeGen).translate_OP_sw,
    hir.OP_sl    : (*CodeGen).translate_OP_sl,
    hir.OP_sq    : (*CodeGen).translate_OP_sq,
    hir.OP_sp    : (*CodeGen).translate_OP_sp,
    hir.OP_ldaq  : (*CodeGen).translate_OP_ldaq,
    hir.OP_ldap  : (*CodeGen).translate_OP_ldap,
    hir.OP_addp  : (*CodeGen).translate_OP_addp,
    hir.OP_subp  : (*CodeGen).translate_OP_subp,
    hir.OP_addpi : (*CodeGen).translate_OP_addpi,
    hir.OP_add   : (*CodeGen).translate_OP_add,
    hir.OP_sub   : (*CodeGen).translate_OP_sub,
    hir.OP_bts   : (*CodeGen).translate_OP_bts,
    hir.OP_addi  : (*CodeGen).translate_OP_addi,
    hir.OP_muli  : (*CodeGen).translate_OP_muli,
    hir.OP_andi  : (*CodeGen).translate_OP_andi,
    hir.OP_xori  : (*CodeGen).translate_OP_xori,
    hir.OP_shri  : (*CodeGen).translate_OP_shri,
    hir.OP_bsi   : (*CodeGen).translate_OP_bsi,
    hir.OP_swapw : (*CodeGen).translate_OP_swapw,
    hir.OP_swapl : (*CodeGen).translate_OP_swapl,
    hir.OP_swapq : (*CodeGen).translate_OP_swapq,
    hir.OP_sxlq  : (*CodeGen).translate_OP_sxlq,
    hir.OP_beq   : (*CodeGen).translate_OP_beq,
    hir.OP_bne   : (*CodeGen).translate_OP_bne,
    hir.OP_blt   : (*CodeGen).translate_OP_blt,
    hir.OP_bltu  : (*CodeGen).translate_OP_bltu,
    hir.OP_bgeu  : (*CodeGen).translate_OP_bgeu,
    hir.OP_bsw   : (*CodeGen).translate_OP_bsw,
    hir.OP_beqn  : (*CodeGen).translate_OP_beqn,
    hir.OP_bnen  : (*CodeGen).translate_OP_bnen,
    hir.OP_jmp   : (*CodeGen).translate_OP_jmp,
    hir.OP_bzero : (*CodeGen).translate_OP_bzero,
    hir.OP_bcopy : (*CodeGen).translate_OP_bcopy,
    hir.OP_ccall : (*CodeGen).translate_OP_ccall,
    hir.OP_gcall : (*CodeGen).translate_OP_gcall,
    hir.OP_icall : (*CodeGen).translate_OP_icall,
    hir.OP_ret   : (*CodeGen).translate_OP_ret,
    hir.OP_break : (*CodeGen).translate_OP_break,
}

func (self *CodeGen) translate_OP_ip(p *x86_64.Program, v *hir.Ir) {
    if v.Pd != hir.Pn {
        if addr := uintptr(v.Pr); addr > math.MaxUint32 {
            p.MOVQ(addr, self.r(v.Pd))
        } else {
            p.MOVL(addr, x86_64.Register32(self.r(v.Pd)))
        }
    }
}

func (self *CodeGen) translate_OP_lb(p *x86_64.Program, v *hir.Ir) {
    if v.Rx != hir.Rz {
        if v.Ps == hir.Pn {
            panic("lb: load from nil pointer")
        } else {
            p.MOVZBQ(self.ptr(p, v.Ps, v.Iv), self.r(v.Rx))
        }
    }
}

func (self *CodeGen) translate_OP_lw(p *x86_64.Program, v *hir.Ir) {
    if v.Rx != hir.Rz {
        if v.Ps == hir.Pn {
            panic("lw: load from nil pointer")
        } else {
            p.MOVZWQ(self.ptr(p, v.Ps, v.Iv), self.r(v.Rx))
        }
    }
}

func (self *CodeGen) translate_OP_ll(p *x86_64.Program, v *hir.Ir) {
    if v.Rx != hir.Rz {
        if v.Ps == hir.Pn {
            panic("ll: load from nil pointer")
        } else {
            p.MOVL(self.ptr(p, v.Ps, v.Iv), x86_64.Register32(self.r(v.Rx)))
        }
    }
}

func (self *CodeGen) translate_OP_lq(p *x86_64.Program, v *hir.Ir) {
    if v.Rx != hir.Rz {
        if v.Ps == hir.Pn {
            panic("lq: load from nil pointer")
        } else {
            p.MOVQ(self.ptr(p, v.Ps, v.Iv), self.r(v.Rx))
        }
    }
}

func (self *CodeGen) translate_OP_lp(p *x86_64.Program, v *hir.Ir) {
    if v.Pd != hir.Pn {
        if v.Ps == hir.Pn {
            panic("lp: load from nil pointer")
        } else {
            p.MOVQ(self.ptr(p, v.Ps, v.Iv), self.r(v.Pd))
        }
    }
}

func (self *CodeGen) translate_OP_sb(p *x86_64.Program, v *hir.Ir) {
    if v.Pd == hir.Pn {
        panic("sb: store to nil pointer")
    } else if v.Rx == hir.Rz {
        p.MOVB(0, self.ptr(p, v.Pd, v.Iv))
    } else {
        p.MOVB(x86_64.Register8(self.r(v.Rx)), self.ptr(p, v.Pd, v.Iv))
    }
}

func (self *CodeGen) translate_OP_sw(p *x86_64.Program, v *hir.Ir) {
    if v.Pd == hir.Pn {
        panic("sw: store to nil pointer")
    } else if v.Rx == hir.Rz {
        p.MOVW(0, self.ptr(p, v.Pd, v.Iv))
    } else {
        p.MOVW(x86_64.Register16(self.r(v.Rx)), self.ptr(p, v.Pd, v.Iv))
    }
}

func (self *CodeGen) translate_OP_sl(p *x86_64.Program, v *hir.Ir) {
    if v.Pd == hir.Pn {
        panic("sl: store to nil pointer")
    } else if v.Rx == hir.Rz {
        p.MOVL(0, self.ptr(p, v.Pd, v.Iv))
    } else {
        p.MOVL(x86_64.Register32(self.r(v.Rx)), self.ptr(p, v.Pd, v.Iv))
    }
}

func (self *CodeGen) translate_OP_sq(p *x86_64.Program, v *hir.Ir) {
    if v.Pd == hir.Pn {
        panic("sq: store to nil pointer")
    } else if v.Rx == hir.Rz {
        p.MOVQ(0, self.ptr(p, v.Pd, v.Iv))
    } else {
        p.MOVQ(self.r(v.Rx), self.ptr(p, v.Pd, v.Iv))
    }
}

func (self *CodeGen) translate_OP_sp(p *x86_64.Program, v *hir.Ir) {
    if v.Pd == hir.Pn {
        panic("sp: store to nil pointer")
    } else {
        self.wbStorePointer(p, v.Ps, self.ptr(p, v.Pd, v.Iv))
    }
}

func (self *CodeGen) translate_OP_ldaq(p *x86_64.Program, v *hir.Ir) {
    if v.Rx != hir.Rz {
        if i := int(v.Iv); i < self.ctxt.argc() {
            self.abiLoadInt(p, i, v.Rx)
        } else {
            panic(fmt.Sprintf("ldaq: argument index out of range: %d", i))
        }
    }
}

func (self *CodeGen) translate_OP_ldap(p *x86_64.Program, v *hir.Ir) {
    if v.Pd != hir.Pn {
        if i := int(v.Iv); i < self.ctxt.argc() {
            self.abiLoadPtr(p, i, v.Pd)
        } else {
            panic(fmt.Sprintf("ldap: argument index out of range: %d", v.Iv))
        }
    }
}

func (self *CodeGen) translate_OP_addp(p *x86_64.Program, v *hir.Ir) {
    if v.Pd != hir.Pn {
        if v.Ps == hir.Pn {
            if v.Rx != hir.Rz {
                panic("addp: direct conversion of integer to pointer")
            } else {
                self.clr(p, v.Pd)
            }
        } else {
            if v.Rx != hir.Rz {
                p.LEAQ(Sib(self.r(v.Ps), self.r(v.Rx), 1, 0), self.r(v.Pd))
            } else {
                self.dup(p, v.Ps, v.Pd)
            }
        }
    }
}

func (self *CodeGen) translate_OP_subp(p *x86_64.Program, v *hir.Ir) {
    if v.Pd != hir.Pn {
        if v.Ps == hir.Pn {
            panic("subp: direct conversion of integer to pointer")
        } else if self.dup(p, v.Ps, v.Pd); v.Rx != hir.Rz {
            p.SUBQ(self.r(v.Rx), self.r(v.Pd))
        }
    }
}

func (self *CodeGen) translate_OP_addpi(p *x86_64.Program, v *hir.Ir) {
    if v.Pd != hir.Pn {
        if v.Ps == hir.Pn {
            if v.Iv != 0 {
                panic("addpi: direct conversion of integer to pointer")
            } else {
                self.clr(p, v.Pd)
            }
        } else {
            if !isInt32(v.Iv) {
                panic("addpi: offset too large, may result in an invalid pointer")
            } else {
                p.LEAQ(Ptr(self.r(v.Ps), int32(v.Iv)), self.r(v.Pd))
            }
        }
    }
}

func (self *CodeGen) translate_OP_add(p *x86_64.Program, v *hir.Ir) {
    if v.Rz != hir.Rz {
        if v.Rx == hir.Rz {
            if v.Ry == hir.Rz {
                self.clr(p, v.Rz)
            } else {
                self.dup(p, v.Ry, v.Rz)
            }
        } else {
            if v.Ry == hir.Rz {
                self.dup(p, v.Rx, v.Rz)
            } else if v.Ry == v.Rz {
                p.ADDQ(self.r(v.Rx), self.r(v.Rz))
            } else {
                self.dup(p, v.Rx, v.Rz)
                p.ADDQ(self.r(v.Ry), self.r(v.Rz))
            }
        }
    }
}

func (self *CodeGen) translate_OP_sub(p *x86_64.Program, v *hir.Ir) {
    if v.Rz != hir.Rz {
        if v.Rx == hir.Rz {
            if v.Ry == hir.Rz {
                self.clr(p, v.Rz)
            } else {
                self.dup(p, v.Ry, v.Rz)
            }
        } else {
            if v.Ry == hir.Rz {
                self.dup(p, v.Rx, v.Rz)
            } else if v.Ry == v.Rz {
                p.SUBQ(self.r(v.Rx), self.r(v.Rz))
                p.NEGQ(self.r(v.Rz))
            } else {
                self.dup(p, v.Rx, v.Rz)
                p.SUBQ(self.r(v.Ry), self.r(v.Rz))
            }
        }
    }
}

func (self *CodeGen) translate_OP_bts(p *x86_64.Program, v *hir.Ir) {
    x := v.Rx
    y := v.Ry
    z := v.Rz

    /* special case: y is zero */
    if y == hir.Rz {
        return
    }

    /* testing and setting the bits at the same time */
    if x == hir.Rz {
        p.BTSQ(0, self.r(y))
    } else {
        p.BTSQ(self.r(x), self.r(y))
    }

    /* set the result if expected */
    if z != hir.Rz {
        p.SETC(x86_64.Register8(self.r(z)))
        p.ANDL(1, x86_64.Register32(self.r(z)))
    }
}

func (self *CodeGen) translate_OP_addi(p *x86_64.Program, v *hir.Ir) {
    if v.Ry != hir.Rz {
        if v.Rx != hir.Rz {
            if self.dup(p, v.Rx, v.Ry); v.Iv != 0 {
                p.ADDQ(self.i32(p, v), self.r(v.Ry))
            }
        } else {
            if v.Iv == 0 {
                self.clr(p, v.Ry)
            } else if !isInt32(v.Iv) {
                p.MOVQ(v.Iv, self.r(v.Ry))
            } else {
                p.MOVL(v.Iv, x86_64.Register32(self.r(v.Ry)))
            }
        }
    }
}

func (self *CodeGen) translate_OP_muli(p *x86_64.Program, v *hir.Ir) {
    var z x86_64.Register
    var x x86_64.Register64
    var y x86_64.Register64

    /* no need to calculate if the result was to be discarded */
    if v.Ry == hir.Rz {
        return
    }

    /* multiply anything by zero is zero */
    if v.Rx == hir.Rz {
        self.clr(p, v.Ry)
        return
    }

    /* get the allocated registers */
    x = self.r(v.Rx)
    y = self.r(v.Ry)

    /* optimized multiplication */
    switch {
        case v.Iv == 0: self.clr(p, v.Ry)           // x * 0 == 0
        case v.Iv == 1: self.dup(p, v.Rx, v.Ry)     // x * 1 == x

        /* multiply by 2, 4 or 8, choose between ADD / SHL and LEA */
        case v.Iv == 2: if x == y { p.ADDQ(x, y) } else { p.LEAQ(Sib(x, x, 1, 0), y) }
        case v.Iv == 4: if x == y { p.SHLQ(2, y) } else { p.LEAQ(Sib(z, x, 4, 0), y) }
        case v.Iv == 8: if x == y { p.SHLQ(3, y) } else { p.LEAQ(Sib(z, x, 8, 0), y) }

        /* small multipliers, use optimized multiplication algorithm */
        case v.Iv == 3  : p.LEAQ(Sib(x, x, 2, 0), y)                                // x * 3  == x + x * 2
        case v.Iv == 5  : p.LEAQ(Sib(x, x, 4, 0), y)                                // x * 5  == x + x * 4
        case v.Iv == 6  : p.LEAQ(Sib(x, x, 2, 0), y); p.ADDQ(y, y)                  // x * 6  == x * 3 * 2
        case v.Iv == 9  : p.LEAQ(Sib(x, x, 8, 0), y)                                // x * 9  == x + x * 8
        case v.Iv == 10 : p.LEAQ(Sib(x, x, 4, 0), y); p.ADDQ(y, y)                  // x * 10 == x * 5 * 2
        case v.Iv == 12 : p.LEAQ(Sib(x, x, 2, 0), y); p.SHLQ(2, y)                  // x * 12 == x * 3 * 4
        case v.Iv == 15 : p.LEAQ(Sib(x, x, 4, 0), y); p.LEAQ(Sib(y, y, 2, 0), y)    // x * 15 == x * 5 * 3
        case v.Iv == 18 : p.LEAQ(Sib(x, x, 8, 0), y); p.ADDQ(y, y)                  // x * 18 == x * 9 * 2
        case v.Iv == 20 : p.LEAQ(Sib(x, x, 4, 0), y); p.SHLQ(2, y)                  // x * 20 == x * 5 * 4
        case v.Iv == 24 : p.LEAQ(Sib(x, x, 2, 0), y); p.SHLQ(3, y)                  // x * 24 == x * 3 * 8
        case v.Iv == 25 : p.LEAQ(Sib(x, x, 4, 0), y); p.LEAQ(Sib(y, y, 4, 0), y)    // x * 25 == x * 5 * 5
        case v.Iv == 27 : p.LEAQ(Sib(x, x, 8, 0), y); p.LEAQ(Sib(y, y, 2, 0), y)    // x * 27 == x * 9 * 3
        case v.Iv == 36 : p.LEAQ(Sib(x, x, 8, 0), y); p.SHLQ(2, y)                  // x * 36 == x * 9 * 4
        case v.Iv == 40 : p.LEAQ(Sib(x, x, 4, 0), y); p.SHLQ(3, y)                  // x * 40 == x * 5 * 8
        case v.Iv == 45 : p.LEAQ(Sib(x, x, 8, 0), y); p.LEAQ(Sib(y, y, 4, 0), y)    // x * 45 == x * 9 * 5
        case v.Iv == 48 : p.LEAQ(Sib(x, x, 2, 0), y); p.SHLQ(4, y)                  // x * 48 == x * 3 * 16
        case v.Iv == 72 : p.LEAQ(Sib(x, x, 8, 0), y); p.SHLQ(3, y)                  // x * 72 == x * 9 * 8
        case v.Iv == 80 : p.LEAQ(Sib(x, x, 4, 0), y); p.SHLQ(4, y)                  // x * 80 == x * 5 * 16
        case v.Iv == 81 : p.LEAQ(Sib(x, x, 8, 0), y); p.LEAQ(Sib(y, y, 8, 0), y)    // x * 81 == x * 9 * 9

        /* multiplier is a power of 2, use shifts */
        case isPow2(v.Iv): {
            self.dup(p, v.Rx, v.Ry)
            p.SHLQ(bits.TrailingZeros64(uint64(v.Iv)), y)
        }

        /* multiplier can fit into a 32-bit integer, use 3-operand IMUL instruction */
        case isInt32(v.Iv): {
            p.IMULQ(v.Iv, x, y)
        }

        /* none of above matches, we need an extra temporary register */
        default: {
            self.dup(p, v.Rx, v.Ry)
            p.MOVQ(v.Iv, RAX)
            p.IMULQ(RAX, y)
        }
    }
}

func (self *CodeGen) translate_OP_andi(p *x86_64.Program, v *hir.Ir) {
    if v.Ry != hir.Rz {
        if v.Iv == 0 || v.Rx == hir.Rz {
            self.clr(p, v.Ry)
        } else {
            self.dup(p, v.Rx, v.Ry)
            p.ANDQ(self.i32(p, v), self.r(v.Ry))
        }
    }
}

func (self *CodeGen) translate_OP_xori(p *x86_64.Program, v *hir.Ir) {
    if v.Ry != hir.Rz {
        if v.Rx != hir.Rz {
            if self.dup(p, v.Rx, v.Ry); v.Iv != 0 {
                p.XORQ(self.i32(p, v), self.r(v.Ry))
            }
        } else {
            if v.Iv == 0 {
                self.clr(p, v.Ry)
            } else if !isInt32(v.Iv) {
                p.MOVQ(v.Iv, self.r(v.Ry))
            } else {
                p.MOVL(v.Iv, x86_64.Register32(self.r(v.Ry)))
            }
        }
    }
}

func (self *CodeGen) translate_OP_shri(p *x86_64.Program, v *hir.Ir) {
    if v.Ry != hir.Rz {
        if v.Iv < 0 {
            panic("shri: negative bit count")
        } else if v.Iv >= 64 || v.Rx == hir.Rz {
            p.XORL(self.r(v.Ry), self.r(v.Ry))
        } else if self.dup(p, v.Rx, v.Ry); v.Iv != 0 {
            p.SHRQ(v.Iv, self.r(v.Ry))
        }
    }
}

func (self *CodeGen) translate_OP_bsi(p *x86_64.Program, v *hir.Ir) {
    if v.Ry != hir.Rz {
        if v.Iv < 0 {
            panic("sbiti: negative bit index")
        } else if v.Rx == hir.Rz {
            self.set(p, v.Ry, 1 << v.Iv)
        } else if self.dup(p, v.Rx, v.Ry); v.Iv < 31 {
            p.ORQ(1 << v.Iv, self.r(v.Ry))
        } else if v.Iv < 64 {
            p.BTSQ(v.Iv, self.r(v.Ry))
        }
    }
}

func (self *CodeGen) translate_OP_swapw(p *x86_64.Program, v *hir.Ir) {
    if v.Ry != hir.Rz {
        self.dup(p, v.Rx, v.Ry)
        p.ROLW(8, x86_64.Register16(self.r(v.Ry)))
    }
}

func (self *CodeGen) translate_OP_swapl(p *x86_64.Program, v *hir.Ir) {
    if v.Ry != hir.Rz {
        self.dup(p, v.Rx, v.Ry)
        p.BSWAPL(x86_64.Register32(self.r(v.Ry)))
    }
}

func (self *CodeGen) translate_OP_swapq(p *x86_64.Program, v *hir.Ir) {
    if v.Ry != hir.Rz {
        self.dup(p, v.Rx, v.Ry)
        p.BSWAPQ(self.r(v.Ry))
    }
}

func (self *CodeGen) translate_OP_sxlq(p *x86_64.Program, v *hir.Ir) {
    if v.Ry != hir.Rz {
        if v.Rx == hir.Rz {
            self.clr(p, v.Ry)
        } else {
            p.MOVSLQ(x86_64.Register32(self.r(v.Rx)), self.r(v.Ry))
        }
    }
}

func (self *CodeGen) translate_OP_beq(p *x86_64.Program, v *hir.Ir) {
    if v.Rx == v.Ry {
        p.JMP(self.to(v.Br))
    } else if v.Rx == hir.Rz {
        p.TESTQ(self.r(v.Ry), self.r(v.Ry))
        p.JZ(self.to(v.Br))
    } else if v.Ry == hir.Rz {
        p.TESTQ(self.r(v.Rx), self.r(v.Rx))
        p.JZ(self.to(v.Br))
    } else {
        p.CMPQ(self.r(v.Ry), self.r(v.Rx))
        p.JE(self.to(v.Br))
    }
}

func (self *CodeGen) translate_OP_bne(p *x86_64.Program, v *hir.Ir) {
    if v.Rx != v.Ry {
        if v.Rx == hir.Rz {
            p.TESTQ(self.r(v.Ry), self.r(v.Ry))
            p.JNZ(self.to(v.Br))
        } else if v.Ry == hir.Rz {
            p.TESTQ(self.r(v.Rx), self.r(v.Rx))
            p.JNZ(self.to(v.Br))
        } else {
            p.CMPQ(self.r(v.Ry), self.r(v.Rx))
            p.JNE(self.to(v.Br))
        }
    }
}

func (self *CodeGen) translate_OP_blt(p *x86_64.Program, v *hir.Ir) {
    if v.Rx != v.Ry {
        if v.Rx == hir.Rz {
            p.TESTQ(self.r(v.Ry), self.r(v.Ry))
            p.JNS(self.to(v.Br))
        } else if v.Ry == hir.Rz {
            p.TESTQ(self.r(v.Rx), self.r(v.Rx))
            p.JS(self.to(v.Br))
        } else {
            p.CMPQ(self.r(v.Ry), self.r(v.Rx))
            p.JL(self.to(v.Br))
        }
    }
}

func (self *CodeGen) translate_OP_bltu(p *x86_64.Program, v *hir.Ir) {
    if v.Rx != v.Ry {
        if v.Rx == hir.Rz {
            p.TESTQ(self.r(v.Ry), self.r(v.Ry))
            p.JNZ(self.to(v.Br))
        } else if v.Ry != hir.Rz {
            p.CMPQ(self.r(v.Ry), self.r(v.Rx))
            p.JB(self.to(v.Br))
        }
    }
}

func (self *CodeGen) translate_OP_bgeu(p *x86_64.Program, v *hir.Ir) {
    if v.Ry == hir.Rz || v.Rx == v.Ry {
        p.JMP(self.to(v.Br))
    } else if v.Rx == hir.Rz {
        p.TESTQ(self.r(v.Ry), self.r(v.Ry))
        p.JZ(self.to(v.Br))
    } else {
        p.CMPQ(self.r(v.Ry), self.r(v.Rx))
        p.JAE(self.to(v.Br))
    }
}

func (self *CodeGen) translate_OP_bsw(p *x86_64.Program, v *hir.Ir) {
    nsw := v.Iv
    tab := v.Sw()

    /* empty switch */
    if nsw == 0 {
        return
    }

    /* allocate switch buffer and default switch label */
    buf := self.tab(nsw)
    def := x86_64.CreateLabel("_default")

    /* set default switch targets */
    for i := 0; i < int(v.Iv); i++ {
        buf.mark(i, def)
    }

    /* assign the specified switch targets */
    for i, ref := range tab {
        if ref != nil {
            buf.mark(i, self.to(ref))
        }
    }

    /* switch on v.Rx */
    p.CMPQ   (nsw, self.r(v.Rx))
    p.JAE    (def)
    p.LEAQ   (x86_64.Ref(buf.ref), RAX)
    p.MOVSLQ (Sib(RAX, self.r(v.Rx), 4, 0), RSI)
    p.ADDQ   (RSI, RAX)
    p.JMPQ   (RAX)
    p.Link   (def)
}

func (self *CodeGen) translate_OP_beqn(p *x86_64.Program, v *hir.Ir) {
    if v.Ps == hir.Pn {
        p.JMP(self.to(v.Br))
    } else {
        p.TESTQ(self.r(v.Ps), self.r(v.Ps))
        p.JZ(self.to(v.Br))
    }
}

func (self *CodeGen) translate_OP_bnen(p *x86_64.Program, v *hir.Ir) {
    if v.Ps != hir.Pn {
        p.TESTQ(self.r(v.Ps), self.r(v.Ps))
        p.JNZ(self.to(v.Br))
    }
}

func (self *CodeGen) translate_OP_jmp(p *x86_64.Program, v *hir.Ir) {
    p.JMP(self.to(v.Br))
}

func (self *CodeGen) translate_OP_bzero(p *x86_64.Program, v *hir.Ir) {
    if v.Pd == hir.Pn {
        panic("bzero: zeroing nil pointer")
    } else if v.Iv != 0 {
        self.abiBlockZero(p, v.Pd, v.Iv)
    }
}

func (self *CodeGen) translate_OP_bcopy(p *x86_64.Program, v *hir.Ir) {
    if v.Ps == hir.Pn {
        panic("bcopy: copy from nil pointer")
    } else if v.Pd == hir.Pn {
        panic("bcopy: copy into nil pointer")
    } else if v.Rx != hir.Rz && v.Ps != v.Pd {
        self.abiBlockCopy(p, v.Pd, v.Ps, v.Rx)
    }
}

func (self *CodeGen) translate_OP_ccall(p *x86_64.Program, v *hir.Ir) {
    self.abiCallNative(p, v)
}

func (self *CodeGen) translate_OP_gcall(p *x86_64.Program, v *hir.Ir) {
    self.abiCallGo(p, v)
}

func (self *CodeGen) translate_OP_icall(p *x86_64.Program, v *hir.Ir) {
    self.abiCallMethod(p, v)
}

func (self *CodeGen) translate_OP_ret(p *x86_64.Program, v *hir.Ir) {
    var i uint8
    var r uint8

    /* store each return values */
    for i = 0; i < v.Rn; i++ {
        if r = v.Rr[i]; r & hir.ArgPointer == 0 {
            self.abiStoreInt(p, hir.GenericRegister(r & hir.ArgMask), int(i))
        } else {
            self.abiStorePtr(p, hir.PointerRegister(r & hir.ArgMask), int(i))
        }
    }

    /* return by jumping to the epilogue */
    p.JMP(self.halt)
}

func (self *CodeGen) translate_OP_break(p *x86_64.Program, _ *hir.Ir) {
    p.INT(3)
}
