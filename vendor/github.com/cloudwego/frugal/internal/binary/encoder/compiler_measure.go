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
    `github.com/cloudwego/frugal/internal/atm/abi`
    `github.com/cloudwego/frugal/internal/binary/defs`
)

func (self Compiler) measure(p *Program, sp int, vt *defs.Type, maxpc int) {
    rt := vt.S
    tt := vt.T

    /* check for loops, recursive only on structs with inlining depth limit */
    if tt == defs.T_struct && (self[rt] || p.pc() >= maxpc || sp >= defs.MaxNesting) {
        p.rtt(OP_size_defer, rt)
        return
    }

    /* measure the type recursively */
    self[rt] = true
    self.measureOne(p, sp, vt, maxpc)
    delete(self, rt)
}

func (self Compiler) measureOne(p *Program, sp int, vt *defs.Type, maxpc int) {
    switch vt.T {
        case defs.T_bool    : p.i64(OP_size_const, 1)
        case defs.T_i8      : p.i64(OP_size_const, 1)
        case defs.T_i16     : p.i64(OP_size_const, 2)
        case defs.T_i32     : p.i64(OP_size_const, 4)
        case defs.T_i64     : p.i64(OP_size_const, 8)
        case defs.T_enum    : p.i64(OP_size_const, 4)
        case defs.T_double  : p.i64(OP_size_const, 8)
        case defs.T_string  : p.i64(OP_size_const, 4); p.dyn(OP_size_dyn, abi.PtrSize, 1)
        case defs.T_binary  : p.i64(OP_size_const, 4); p.dyn(OP_size_dyn, abi.PtrSize, 1)
        case defs.T_map     : self.measureMap(p, sp, vt, maxpc)
        case defs.T_set     : self.measureSeq(p, sp, vt, maxpc)
        case defs.T_list    : self.measureSeq(p, sp, vt, maxpc)
        case defs.T_struct  : self.measureStruct(p, sp, vt, maxpc)
        case defs.T_pointer : self.measurePtr(p, sp, vt, maxpc)
        default             : panic("measureOne: unreachable")
    }
}

func (self Compiler) measurePtr(p *Program, sp int, vt *defs.Type, maxpc int) {
    i := p.pc()
    p.tag(sp)
    p.add(OP_if_nil)
    p.add(OP_make_state)
    p.add(OP_deref)
    self.measure(p, sp + 1, vt.V, maxpc)
    p.add(OP_drop_state)
    p.pin(i)
}

func (self Compiler) measureMap(p *Program, sp int, vt *defs.Type, maxpc int) {
    nk := defs.GetSize(vt.K.S)
    nv := defs.GetSize(vt.V.S)

    /* 6-byte map header */
    p.tag(sp)
    p.i64(OP_size_const, 6)

    /* check for nil maps */
    i := p.pc()
    p.add(OP_if_nil)

    /* key and value are both trivially measuable */
    if nk > 0 && nv > 0 {
        p.i64(OP_size_map, int64(nk + nv))
        p.pin(i)
        return
    }

    /* key or value is trivially measuable */
    if nk > 0 { p.i64(OP_size_map, int64(nk)) }
    if nv > 0 { p.i64(OP_size_map, int64(nv)) }

    /* complex maps */
    j := p.pc()
    p.add(OP_map_if_empty)
    p.add(OP_make_state)
    p.rtt(OP_map_begin, vt.S)
    k := p.pc()

    /* complex keys */
    if nk <= 0 {
        p.add(OP_map_key)
        self.measure(p, sp + 1, vt.K, maxpc)
    }

    /* complex values */
    if nv <= 0 {
        p.add(OP_map_value)
        self.measure(p, sp + 1, vt.V, maxpc)
    }

    /* move to the next state */
    p.add(OP_map_next)
    p.jmp(OP_map_if_next, k)
    p.add(OP_drop_state)
    p.pin(i)
    p.pin(j)
}

func (self Compiler) measureSeq(p *Program, sp int, vt *defs.Type, maxpc int) {
    et := vt.V
    nb := defs.GetSize(et.S)

    /* 5-byte list or set header */
    p.tag(sp)
    p.i64(OP_size_const, 5)

    /* check for nil slice */
    i := p.pc()
    p.add(OP_if_nil)

    /* element is trivially measuable */
    if nb > 0 {
        p.dyn(OP_size_dyn, abi.PtrSize, int64(nb))
        p.pin(i)
        return
    }

    /* complex lists or sets */
    j := p.pc()
    p.add(OP_list_if_empty)
    p.add(OP_make_state)
    p.add(OP_list_begin)
    k := p.pc()
    p.add(OP_goto)
    r := p.pc()
    p.i64(OP_seek, int64(et.S.Size()))
    p.pin(k)
    self.measure(p, sp + 1, et, maxpc)
    p.add(OP_list_decr)
    p.jmp(OP_list_if_next, r)
    p.add(OP_drop_state)
    p.pin(i)
    p.pin(j)
}

func (self Compiler) measureStruct(p *Program, sp int, vt *defs.Type, maxpc int) {
    var off int
    var err error
    var fvs []defs.Field

    /* struct is trivially measuable */
    if nb := defs.GetSize(vt.S); nb > 0 {
        p.i64(OP_size_const, int64(nb))
        return
    }

    /* resolve the field */
    if fvs, err = defs.ResolveFields(vt.S); err != nil {
        panic(err)
    }

    /* empty structs */
    if len(fvs) == 0 {
        p.i64(OP_size_const, 4)
        return
    }

    /* 1-byte stop field */
    p.tag(sp)
    p.i64(OP_size_const, 1)

    /* measure every field */
    for _, fv := range fvs {
        p.i64(OP_seek, int64(fv.F - off))
        self.measureField(p, sp + 1, fv, maxpc)
        off = fv.F
    }
}

func (self Compiler) measureField(p *Program, sp int, fv defs.Field, maxpc int) {
    switch fv.Type.T {
        default: {
            panic("fatal: invalid field type: " + fv.Type.String())
        }

        /* non-pointer types */
        case defs.T_bool   : fallthrough
        case defs.T_i8     : fallthrough
        case defs.T_double : fallthrough
        case defs.T_i16    : fallthrough
        case defs.T_i32    : fallthrough
        case defs.T_i64    : fallthrough
        case defs.T_string : fallthrough
        case defs.T_struct : fallthrough
        case defs.T_enum   : fallthrough
        case defs.T_binary : self.measureStructRequired(p, sp, fv, maxpc)

        /* sequencial types */
        case defs.T_map  : fallthrough
        case defs.T_set  : fallthrough
        case defs.T_list : {
            if fv.Spec != defs.Optional {
                self.measureStructRequired(p, sp, fv, maxpc)
            } else {
                self.measureStructIterable(p, sp, fv, maxpc)
            }
        }

        /* pointers */
        case defs.T_pointer: {
            if fv.Spec == defs.Optional {
                self.measureStructOptional(p, sp, fv, maxpc)
            } else if fv.Type.V.T == defs.T_struct {
                self.measureStructPointer(p, sp, fv, maxpc)
            } else {
                panic("fatal: non-optional non-struct pointers")
            }
        }
    }
}

func (self Compiler) measureStructPointer(p *Program, sp int, fv defs.Field, maxpc int) {
    i := p.pc()
    p.add(OP_if_nil)
    p.i64(OP_size_const, 3)
    p.add(OP_make_state)
    p.add(OP_deref)
    self.measure(p, sp + 1, fv.Type.V, maxpc)
    p.add(OP_drop_state)
    j := p.pc()
    p.add(OP_goto)
    p.pin(i)
    p.i64(OP_size_const, 4)
    p.pin(j)
}

func (self Compiler) measureStructOptional(p *Program, sp int, fv defs.Field, maxpc int) {
    i := p.pc()
    p.add(OP_if_nil)
    p.i64(OP_size_const, 3)
    p.add(OP_make_state)
    p.add(OP_deref)
    self.measure(p, sp + 1, fv.Type.V, maxpc)
    p.add(OP_drop_state)
    p.pin(i)
}

func (self Compiler) measureStructRequired(p *Program, sp int, fv defs.Field, maxpc int) {
    p.i64(OP_size_const, 3)
    self.measure(p, sp, fv.Type, maxpc)
}

func (self Compiler) measureStructIterable(p *Program, sp int, fv defs.Field, maxpc int) {
    i := p.pc()
    p.add(OP_if_nil)
    p.i64(OP_size_const, 3)
    self.measure(p, sp, fv.Type, maxpc)
    p.pin(i)
}