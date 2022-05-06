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
    `fmt`
    `reflect`
    `sort`
    `strconv`
    `strings`
    `unsafe`

    `github.com/cloudwego/frugal/internal/binary/defs`
    `github.com/cloudwego/frugal/internal/rt`
    `github.com/cloudwego/frugal/internal/utils`
)

type Instr struct {
    Op OpCode
    Tx defs.Tag
    Id uint16
    To int
    Iv int64
    Sw *int
    Vt *rt.GoType
}

func (self Instr) stab() string {
    t := mkstab(self.Sw, self.Iv)
    s := make([]string, 0, self.Iv)

    /* convert to strings */
    for i, v := range t {
        if v >= 0 {
            s = append(s, fmt.Sprintf("%4ccase %d: L_%d\n", ' ', i, v))
        }
    }

    /* join them together */
    return fmt.Sprintf(
        "{\n%s}",
        strings.Join(s, ""),
    )
}

func (self Instr) rtab() string {
    t := mkstab(self.Sw, self.Iv)
    s := make([]string, 0, self.Iv)

    /* convert to strings */
    for _, v := range t {
        s = append(s, strconv.Itoa(v))
    }

    /* join with ',' */
    return strings.Join(s, ", ")
}

func (self Instr) Disassemble() string {
    switch self.Op {
        case OP_int               : fallthrough
        case OP_size              : fallthrough
        case OP_seek              : fallthrough
        case OP_struct_mark_tag   : return fmt.Sprintf("%-18s%d", self.Op, self.Iv)
        case OP_type              : return fmt.Sprintf("%-18s%d", self.Op, self.Tx)
        case OP_deref             : fallthrough
        case OP_map_alloc         : fallthrough
        case OP_map_set_i8        : fallthrough
        case OP_map_set_i16       : fallthrough
        case OP_map_set_i32       : fallthrough
        case OP_map_set_i64       : fallthrough
        case OP_map_set_str       : fallthrough
        case OP_map_set_enum      : fallthrough
        case OP_map_set_pointer   : fallthrough
        case OP_list_alloc        : fallthrough
        case OP_construct         : fallthrough
        case OP_defer             : return fmt.Sprintf("%-18s%s", self.Op, self.Vt)
        case OP_ctr_is_zero       : fallthrough
        case OP_struct_is_stop    : fallthrough
        case OP_goto              : return fmt.Sprintf("%-18sL_%d", self.Op, self.To)
        case OP_struct_bitmap     : fallthrough
        case OP_struct_require    : return fmt.Sprintf("%-18s%s", self.Op, self.rtab())
        case OP_struct_switch     : return fmt.Sprintf("%-18s%s", self.Op, self.stab())
        case OP_struct_check_type : return fmt.Sprintf("%-18s%d, L_%d", self.Op, self.Tx, self.To)
        default                   : return self.Op.String()
    }
}

func mkins(op OpCode, dt defs.Tag, id uint16, to int, iv int64, sw []int, vt reflect.Type) Instr {
    return Instr {
        Op: op,
        Tx: dt,
        Id: id,
        To: to,
        Vt: rt.UnpackType(vt),
        Iv: int64(len(sw)) | iv,
        Sw: (*int)((*rt.GoSlice)(unsafe.Pointer(&sw)).Ptr),
    }
}

type (
    Program  []Instr
    Compiler map[reflect.Type]bool
)

func (self Program) pc() int {
    return len(self)
}

func (self Program) pin(i int) {
    self[i].To = self.pc()
}

func (self Program) def(n int) {
    if n >= defs.StackSize {
        panic("type nesting too deep")
    }
}

func (self *Program) ins(iv Instr)                             { *self = append(*self, iv) }
func (self *Program) add(op OpCode)                            { self.ins(mkins(op, 0, 0, 0, 0, nil, nil)) }
func (self *Program) jmp(op OpCode, to int)                    { self.ins(mkins(op, 0, 0, to, 0, nil, nil)) }
func (self *Program) i64(op OpCode, iv int64)                  { self.ins(mkins(op, 0, 0, 0, iv, nil, nil)) }
func (self *Program) tab(op OpCode, tv []int)                  { self.ins(mkins(op, 0, 0, 0, 0, tv, nil)) }
func (self *Program) tag(op OpCode, vt defs.Tag)               { self.ins(mkins(op, vt, 0, 0, 0, nil, nil)) }
func (self *Program) rtt(op OpCode, vt reflect.Type)           { self.ins(mkins(op, 0, 0, 0, 0, nil, vt)) }
func (self *Program) jcc(op OpCode, vt defs.Tag, to int)       { self.ins(mkins(op, vt, 0, to, 0, nil, nil)) }
func (self *Program) req(op OpCode, vt reflect.Type, fv []int) { self.ins(mkins(op, 0, 0, 0, 0, fv, vt)) }

func (self Program) Free() {
    freeProgram(self)
}

func (self Program) Disassemble() string {
    nb  := len(self)
    tab := make([]bool, nb + 1)
    ret := make([]string, 0, nb + 1)

    /* prescan to get all the labels */
    for _, ins := range self {
        if _OpBranches[ins.Op] {
            if ins.Op != OP_struct_switch {
                tab[ins.To] = true
            } else {
                for _, v := range mkstab(ins.Sw, ins.Iv) {
                    if v >= 0 {
                        tab[v] = true
                    }
                }
            }
        }
    }

    /* disassemble each instruction */
    for i, ins := range self {
        ln := ""
        ds := ins.Disassemble()

        /* check for label reference */
        if tab[i] {
            ret = append(ret, fmt.Sprintf("L_%d:", i))
        }

        /* indent each line */
        for _, ln = range strings.Split(ds, "\n") {
            ret = append(ret, "    " + ln)
        }
    }

    /* add the last label, if needed */
    if tab[nb] {
        ret = append(ret, fmt.Sprintf("L_%d:", nb))
    }

    /* add an "end" indicator, and join all the strings */
    return strings.Join(append(ret, "    end"), "\n")
}

func CreateCompiler() Compiler {
    return newCompiler()
}

func (self Compiler) rescue(ep *error) {
    if val := recover(); val != nil {
        if err, ok := val.(error); ok {
            *ep = err
        } else {
            panic(val)
        }
    }
}

func (self Compiler) compileOne(p *Program, sp int, vt *defs.Type) {
    if vt.T == defs.T_pointer {
        self.compilePtr(p, sp, vt)
    } else if _, ok := self[vt.S]; vt.T != defs.T_struct || (!ok && sp < defs.MaxNesting && p.pc() < defs.MaxILBuffer) {
        self.compileTag(p, sp, vt)
    } else {
        p.rtt(OP_defer, vt.S)
    }
}

func (self Compiler) compileTag(p *Program, sp int, vt *defs.Type) {
    self[vt.S] = true
    self.compileRec(p, sp, vt)
    delete(self, vt.S)
}

func (self Compiler) compileRec(p *Program, sp int, vt *defs.Type) {
    switch vt.T {
        case defs.T_bool   : p.i64(OP_size, 1); p.i64(OP_int, 1)
        case defs.T_i8     : p.i64(OP_size, 1); p.i64(OP_int, 1)
        case defs.T_i16    : p.i64(OP_size, 2); p.i64(OP_int, 2)
        case defs.T_i32    : p.i64(OP_size, 4); p.i64(OP_int, 4)
        case defs.T_i64    : p.i64(OP_size, 8); p.i64(OP_int, 8)
        case defs.T_double : p.i64(OP_size, 8); p.i64(OP_int, 8)
        case defs.T_string : p.i64(OP_size, 4); p.add(OP_str)
        case defs.T_binary : p.i64(OP_size, 4); p.add(OP_bin)
        case defs.T_enum   : p.i64(OP_size, 4); p.add(OP_enum)
        case defs.T_struct : self.compileStruct  (p, sp, vt)
        case defs.T_map    : self.compileMap     (p, sp, vt)
        case defs.T_set    : self.compileSetList (p, sp, vt.V)
        case defs.T_list   : self.compileSetList (p, sp, vt.V)
        default            : panic("unreachable")
    }
}

func (self Compiler) compilePtr(p *Program, sp int, vt *defs.Type) {
    p.add(OP_make_state)
    p.rtt(OP_deref, vt.V.S)
    self.compileOne(p, sp + 1, vt.V)
    p.add(OP_drop_state)
}

func (self Compiler) compileMap(p *Program, sp int, vt *defs.Type) {
    p.def(sp)
    p.i64(OP_size, 6)
    p.tag(OP_type, vt.K.Tag())
    p.tag(OP_type, vt.V.Tag())
    p.add(OP_make_state)
    p.add(OP_ctr_load)
    p.rtt(OP_map_alloc, vt.S)
    i := p.pc()
    p.add(OP_ctr_is_zero)
    self.compileKey(p, sp + 1, vt)
    self.compileOne(p, sp + 1, vt.V)
    p.add(OP_ctr_decr)
    p.jmp(OP_goto, i)
    p.pin(i)
    p.add(OP_map_close)
    p.add(OP_drop_state)
}

func (self Compiler) compileKey(p *Program, sp int, vt *defs.Type) {
    switch vt.K.T {
        case defs.T_bool    : p.i64(OP_size, 1); p.rtt(OP_map_set_i8, vt.S)
        case defs.T_i8      : p.i64(OP_size, 1); p.rtt(OP_map_set_i8, vt.S)
        case defs.T_double  : p.i64(OP_size, 8); p.rtt(OP_map_set_i64, vt.S)
        case defs.T_i16     : p.i64(OP_size, 2); p.rtt(OP_map_set_i16, vt.S)
        case defs.T_i32     : p.i64(OP_size, 4); p.rtt(OP_map_set_i32, vt.S)
        case defs.T_i64     : p.i64(OP_size, 8); p.rtt(OP_map_set_i64, vt.S)
        case defs.T_binary  : p.i64(OP_size, 4); p.rtt(OP_map_set_str, vt.S)
        case defs.T_string  : p.i64(OP_size, 4); p.rtt(OP_map_set_str, vt.S)
        case defs.T_enum    : p.i64(OP_size, 4); p.rtt(OP_map_set_enum, vt.S)
        case defs.T_pointer : self.compileKeyPtr(p, sp, vt)
        default             : panic("unreachable")
    }
}

func (self Compiler) compileKeyPtr(p *Program, sp int, vt *defs.Type) {
    pt := vt.K
    st := pt.V

    /* must be a struct */
    if st.T != defs.T_struct {
        panic("map key cannot be non-struct pointers")
    }

    /* construct a new object */
    p.rtt(OP_construct, st.S)
    self.compileOne(p, sp, st)
    p.rtt(OP_map_set_pointer, vt.S)
}

func (self Compiler) compileStruct(p *Program, sp int, vt *defs.Type) {
    var fid int
    var err error
    var req []int
    var fvs []defs.Field

    /* resolve the fields */
    if fvs, err = defs.ResolveFields(vt.S); err != nil {
        panic(err)
    }

    /* empty struct */
    if len(fvs) == 0 {
        p.add(OP_struct_ignore)
        return
    }

    /* find the maximum field IDs */
    for _, fv := range fvs {
        if fid = utils.MaxInt(fid, int(fv.ID)); fv.Spec == defs.Required {
            req = append(req, int(fv.ID))
        }
    }

    /* save the current state */
    p.def(sp)
    p.add(OP_make_state)

    /* allocate bitmap for required fields, if needed */
    if sort.Ints(req); len(req) != 0 {
        p.tab(OP_struct_bitmap, req)
    }

    /* switch jump buffer */
    i := p.pc()
    s := make([]int, fid + 1)

    /* set the default branch */
    for v := range s {
        s[v] = -1
    }

    /* dispatch the next field */
    p.i64(OP_size, 1)
    p.add(OP_struct_read_type)
    j := p.pc()
    p.add(OP_struct_is_stop)
    p.i64(OP_size, 2)
    p.tab(OP_struct_switch, s)
    k := p.pc()
    p.add(OP_struct_skip)
    p.jmp(OP_goto, i)

    /* assemble every field */
    for _, fv := range fvs {
        s[fv.ID] = p.pc()
        p.jcc(OP_struct_check_type, fv.Type.Tag(), k)

        /* mark the field as seen, if needed */
        if fv.Spec == defs.Required {
            p.i64(OP_struct_mark_tag, int64(fv.ID))
        }

        /* seek and parse the field */
        p.i64(OP_seek, int64(fv.F))
        self.compileOne(p, sp + 1, fv.Type)
        p.i64(OP_seek, -int64(fv.F))
        p.jmp(OP_goto, i)
    }

    /* no required fields */
    if p.pin(j); len(req) == 0 {
        p.add(OP_drop_state)
        return
    }

    /* check all the required fields */
    p.req(OP_struct_require, vt.S, req)
    p.add(OP_drop_state)
}

func (self Compiler) compileSetList(p *Program, sp int, et *defs.Type) {
    p.def(sp)
    p.i64(OP_size, 5)
    p.tag(OP_type, et.Tag())
    p.add(OP_make_state)
    p.add(OP_ctr_load)
    p.rtt(OP_list_alloc, et.S)
    i := p.pc()
    p.add(OP_ctr_is_zero)
    j := p.pc()
    self.compileOne(p, sp + 1, et)
    p.add(OP_ctr_decr)
    k := p.pc()
    p.add(OP_ctr_is_zero)
    p.i64(OP_seek, int64(et.S.Size()))
    p.jmp(OP_goto, j)
    p.pin(i)
    p.pin(k)
    p.add(OP_drop_state)
}

func (self Compiler) Free() {
    freeCompiler(self)
}

func (self Compiler) Compile(vt reflect.Type) (_ Program, err error) {
    ret := newProgram()
    vtp := defs.ParseType(vt, "")

    /* catch the exceptions, and free the type */
    defer self.rescue(&err)
    defer vtp.Free()

    /* compile the actual type */
    self.compileOne(&ret, 0, vtp)
    ret.add(OP_halt)
    return ret, nil
}

func (self Compiler) CompileAndFree(vt reflect.Type) (ret Program, err error) {
    ret, err = self.Compile(vt)
    self.Free()
    return
}
