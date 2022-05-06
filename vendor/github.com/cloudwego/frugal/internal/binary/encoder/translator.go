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
    `fmt`
    `os`
    `reflect`

    `github.com/cloudwego/frugal/internal/atm/abi`
    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/binary/defs`
    `github.com/cloudwego/frugal/internal/rt`
    `github.com/cloudwego/frugal/internal/utils`
)

/** Function Prototype
 *
 *      func (
 *          buf unsafe.Pointer,
 *          len int,
 *          mem iov.BufferWriter,
 *          p   unsafe.Pointer,
 *          rs  *RuntimeState,
 *          st  int,
 *      ) (
 *          pos int,
 *          err error,
 *      )
 */

const (
    ARG_buf      = 0
    ARG_len      = 1
    ARG_mem_itab = 2
    ARG_mem_data = 3
    ARG_p        = 4
    ARG_rs       = 5
    ARG_st       = 6
)

/** Register Allocations
 *
 *      P1      Current Working Pointer
 *      P2      Output Buffer Pointer
 *      P3      Runtime State Pointer
 *      P4      Error Type Pointer
 *      P5      Error Value Pointer
 *
 *      R2      Output Buffer Length
 *      R3      Output Buffer Capacity
 *      R4      State Index
 */

const (
    WP = hir.P1
    RP = hir.P2
    RS = hir.P3
    ET = hir.P4 // may also be used as a temporary pointer register
    EP = hir.P5 // may also be used as a temporary pointer register
)

const (
    RL = hir.R2
    RC = hir.R3
    ST = hir.R4
)

const (
    TP = hir.P0
    TR = hir.R0
    UR = hir.R1
)

const (
    LB_halt       = "_halt"
    LB_error      = "_error"
    LB_nomem      = "_nomem"
    LB_overflow   = "_overflow"
    LB_duplicated = "_duplicated"
)

var (
    _N_page       = int64(os.Getpagesize())
    _E_nomem      = fmt.Errorf("frugal: buffer is too small")
    _E_overflow   = fmt.Errorf("frugal: encoder stack overflow")
    _E_duplicated = fmt.Errorf("frugal: duplicated element within sets")
)

func Translate(s Program) hir.Program {
    p := hir.CreateBuilder()
    prologue (p)
    program  (p, s)
    epilogue (p)
    errors   (p)
    return p.Build()
}

func errors(p *hir.Builder) {
    p.Label (LB_nomem)
    p.MOV   (UR, RL)
    p.IP    (&_E_nomem, TP)
    p.JMP   ("_basic_error")
    p.Label (LB_overflow)
    p.IP    (&_E_overflow, TP)
    p.JMP   ("_basic_error")
    p.Label (LB_duplicated)
    p.IP    (&_E_duplicated, TP)
    p.Label ("_basic_error")
    p.LP    (TP, 0, ET)
    p.LP    (TP, 8, EP)
    p.JMP   (LB_error)
}

func program(p *hir.Builder, s Program) {
    for i, v := range s {
        p.Mark(i)
        translators[v.Op](p, v)
    }
}

func prologue(p *hir.Builder) {
    p.LDAP  (ARG_buf, RP)
    p.LDAQ  (ARG_len, RC)
    p.LDAP  (ARG_p, WP)
    p.LDAP  (ARG_rs, RS)
    p.LDAQ  (ARG_st, ST)
    p.MOV   (hir.Rz, UR)
    p.MOV   (hir.Rz, RL)
}

func epilogue(p *hir.Builder) {
    p.Label (LB_halt)
    p.MOVP  (hir.Pn, ET)
    p.MOVP  (hir.Pn, EP)
    p.Label (LB_error)
    p.RET   ().
      R0    (RL).
      R1    (ET).
      R2    (EP)
}

var translators = [256]func(*hir.Builder, Instr) {
    OP_size_check    : translate_OP_size_check,
    OP_size_const    : translate_OP_size_const,
    OP_size_dyn      : translate_OP_size_dyn,
    OP_size_map      : translate_OP_size_map,
    OP_size_defer    : translate_OP_size_defer,
    OP_byte          : translate_OP_byte,
    OP_word          : translate_OP_word,
    OP_long          : translate_OP_long,
    OP_quad          : translate_OP_quad,
    OP_sint          : translate_OP_sint,
    OP_length        : translate_OP_length,
    OP_memcpy_be     : translate_OP_memcpy_be,
    OP_seek          : translate_OP_seek,
    OP_deref         : translate_OP_deref,
    OP_defer         : translate_OP_defer,
    OP_map_len       : translate_OP_map_len,
    OP_map_key       : translate_OP_map_key,
    OP_map_next      : translate_OP_map_next,
    OP_map_value     : translate_OP_map_value,
    OP_map_begin     : translate_OP_map_begin,
    OP_map_if_next   : translate_OP_map_if_next,
    OP_map_if_empty  : translate_OP_map_if_empty,
    OP_list_decr     : translate_OP_list_decr,
    OP_list_begin    : translate_OP_list_begin,
    OP_list_if_next  : translate_OP_list_if_next,
    OP_list_if_empty : translate_OP_list_if_empty,
    OP_unique        : translate_OP_unique,
    OP_goto          : translate_OP_goto,
    OP_if_nil        : translate_OP_if_nil,
    OP_if_hasbuf     : translate_OP_if_hasbuf,
    OP_make_state    : translate_OP_make_state,
    OP_drop_state    : translate_OP_drop_state,
    OP_halt          : translate_OP_halt,
}

func translate_OP_size_check(p *hir.Builder, v Instr) {
    p.ADDI  (RL, v.Iv, UR)
    p.BLTU  (RC, UR, LB_nomem)
}

func translate_OP_size_const(p *hir.Builder, v Instr) {
    p.ADDI  (RL, v.Iv, RL)
}

func translate_OP_size_dyn(p *hir.Builder, v Instr) {
    p.LQ    (WP, int64(v.Uv), TR)
    p.MULI  (TR, v.Iv, TR)
    p.ADD   (RL, TR, RL)
}

func translate_OP_size_map(p *hir.Builder, v Instr) {
    p.LP    (WP, 0, TP)
    p.LQ    (TP, 0, TR)
    p.MULI  (TR, v.Iv, TR)
    p.ADD   (RL, TR, RL)
}

func translate_OP_size_defer(p *hir.Builder, v Instr) {
    p.IP    (v.Vt, TP)
    p.GCALL (F_encode).
      A0    (TP).
      A1    (hir.Pn).
      A2    (hir.Rz).
      A3    (hir.Pn).
      A4    (hir.Pn).
      A5    (WP).
      A6    (RS).
      A7    (ST).
      R0    (TR).
      R1    (ET).
      R2    (EP)
    p.BNEN  (ET, LB_error)
    p.ADD   (RL, TR, RL)
}

func translate_OP_byte(p *hir.Builder, v Instr) {
    p.ADDP  (RP, RL, TP)
    p.ADDI  (RL, 1, RL)
    p.IB    (int8(v.Iv), TR)
    p.SB    (TR, TP, 0)
}

func translate_OP_word(p *hir.Builder, v Instr) {
    p.ADDP  (RP, RL, TP)
    p.ADDI  (RL, 2, RL)
    p.IW    (bswap16(v.Iv), TR)
    p.SW    (TR, TP, 0)
}

func translate_OP_long(p *hir.Builder, v Instr) {
    p.ADDP  (RP, RL, TP)
    p.ADDI  (RL, 4, RL)
    p.IL    (bswap32(v.Iv), TR)
    p.SL    (TR, TP, 0)
}

func translate_OP_quad(p *hir.Builder, v Instr) {
    p.ADDP  (RP, RL, TP)
    p.ADDI  (RL, 8, RL)
    p.IQ    (bswap64(v.Iv), TR)
    p.SQ    (TR, TP, 0)
}

func translate_OP_sint(p *hir.Builder, v Instr) {
    p.ADDP  (RP, RL, TP)
    p.ADDI  (RL, v.Iv, RL)

    /* check for copy size */
    switch v.Iv {
        case 1  : p.LB(WP, 0, TR);                  p.SB(TR, TP, 0)
        case 2  : p.LW(WP, 0, TR); p.SWAPW(TR, TR); p.SW(TR, TP, 0)
        case 4  : p.LL(WP, 0, TR); p.SWAPL(TR, TR); p.SL(TR, TP, 0)
        case 8  : p.LQ(WP, 0, TR); p.SWAPQ(TR, TR); p.SQ(TR, TP, 0)
        default : panic("can only convert 1, 2, 4 or 8 bytes at a time")
    }
}

func translate_OP_length(p *hir.Builder, v Instr) {
    p.LL    (WP, v.Iv, TR)
    p.SWAPL (TR, TR)
    p.ADDP  (RP, RL, TP)
    p.ADDI  (RL, 4, RL)
    p.SL    (TR, TP, 0)
}

func translate_OP_memcpy_1(p *hir.Builder) {
    p.IQ    (_N_page, UR)
    p.BGEU  (UR, TR, "_do_copy_{n}")
    p.LDAP  (ARG_mem_itab, ET)
    p.LDAP  (ARG_mem_data, EP)
    p.BEQN  (EP, "_do_copy_{n}")
    p.SUB   (RC, RL, UR)
    p.ICALL (ET, EP, utils.FnWrite).
      A0    (TP).
      A1    (TR).
      A2    (TR).
      A3    (UR).
      R0    (ET).
      R1    (EP)
    p.BNEN  (ET, LB_error)
    p.JMP   ("_done_{n}")
    p.Label ("_do_copy_{n}")
    p.ADD   (RL, TR, UR)
    p.BLTU  (RC, UR, LB_nomem)
    p.ADDP  (RP, RL, EP)
    p.MOV   (UR, RL)
    p.BCOPY (TP, TR, EP)
    p.Label ("_done_{n}")
}

func translate_OP_memcpy_be(p *hir.Builder, v Instr) {
    p.LQ    (WP, int64(v.Uv), TR)
    p.BEQ   (TR, hir.Rz, "_done_{n}")
    p.LP    (WP, 0, TP)

    /* special case: unit of a single byte */
    if v.Iv == 1 {
        translate_OP_memcpy_1(p)
        return
    }

    /* adjust the buffer length */
    p.MULI  (TR, v.Iv, UR)
    p.ADD   (RL, UR, UR)
    p.BLTU  (RC, UR, LB_nomem)
    p.ADDP  (RP, RL, EP)
    p.MOV   (UR, RL)
    p.Label ("_loop_{n}")
    p.BEQ   (TR, hir.Rz, "_done_{n}")

    /* load-swap-store sequence */
    switch v.Iv {
        case 2  : p.LW(TP, 0, UR); p.SWAPW(UR, UR); p.SW(UR, EP, 0)
        case 4  : p.LL(TP, 0, UR); p.SWAPL(UR, UR); p.SL(UR, EP, 0)
        case 8  : p.LQ(TP, 0, UR); p.SWAPQ(UR, UR); p.SQ(UR, EP, 0)
        default : panic("can only swap 2, 4 or 8 bytes at a time")
    }

    /* update loop counter */
    p.SUBI  (TR, 1, TR)
    p.ADDPI (TP, v.Iv, TP)
    p.ADDPI (EP, v.Iv, EP)
    p.JMP   ("_loop_{n}")
    p.Label ("_done_{n}")
}

func translate_OP_seek(p *hir.Builder, v Instr) {
    p.ADDPI (WP, v.Iv, WP)
}

func translate_OP_deref(p *hir.Builder, _ Instr) {
    p.LP    (WP, 0, WP)
}

func translate_OP_defer(p *hir.Builder, v Instr) {
    p.IP    (v.Vt, TP)
    p.LDAP  (ARG_mem_itab, ET)
    p.LDAP  (ARG_mem_data, EP)
    p.SUB   (RC, RL, TR)
    p.ADDP  (RP, RL, RP)
    p.GCALL (F_encode).
      A0    (TP).
      A1    (RP).
      A2    (TR).
      A3    (ET).
      A4    (EP).
      A5    (WP).
      A6    (RS).
      A7    (ST).
      R0    (TR).
      R1    (ET).
      R2    (EP)
    p.SUBP  (RP, RL, RP)
    p.BNEN  (ET, LB_error)
    p.ADD   (RL, TR, RL)
}

func translate_OP_map_len(p *hir.Builder, _ Instr) {
    p.LP    (WP, 0, TP)
    p.LQ    (TP, 0, TR)
    p.SWAPL (TR, TR)
    p.ADDP  (RP, RL, TP)
    p.ADDI  (RL, 4, RL)
    p.SL    (TR, TP, 0)
}

func translate_OP_map_key(p *hir.Builder, _ Instr) {
    p.ADDP  (RS, ST, TP)
    p.LP    (TP, MiOffset + MiKeyOffset, WP)
}

func translate_OP_map_next(p *hir.Builder, _ Instr) {
    p.ADDP  (RS, ST, TP)
    p.ADDPI (TP, MiOffset, TP)
    p.GCALL (F_mapiternext).A0(TP)
}

func translate_OP_map_value(p *hir.Builder, _ Instr) {
    p.ADDP  (RS, ST, TP)
    p.LP    (TP, MiOffset + MiValueOffset, WP)
}

func translate_OP_map_begin(p *hir.Builder, v Instr) {
    p.IP    (v.Vt, ET)
    p.LP    (WP, 0, EP)
    p.ADDP  (RS, ST, TP)
    p.ADDPI (TP, MiOffset, TP)
    p.BZERO (MiSize, TP)
    p.GCALL (F_mapiterinit).
      A0    (ET).
      A1    (EP).
      A2    (TP)
}

func translate_OP_map_if_next(p *hir.Builder, v Instr) {
    p.ADDP  (RS, ST, TP)
    p.LP    (TP, MiOffset + MiKeyOffset, TP)
    p.BNEN  (TP, p.At(v.To))
}

func translate_OP_map_if_empty(p *hir.Builder, v Instr) {
    p.LP    (WP, 0, TP)
    p.LQ    (TP, 0, TR)
    p.BEQ   (TR, hir.Rz, p.At(v.To))
}

func translate_OP_list_decr(p *hir.Builder, _ Instr) {
    p.ADDP  (RS, ST, TP)
    p.LQ    (TP, LnOffset, TR)
    p.SUBI  (TR, 1, TR)
    p.SQ    (TR, TP, LnOffset)
}

func translate_OP_list_begin(p *hir.Builder, _ Instr) {
    p.LQ    (WP, abi.PtrSize, TR)
    p.LP    (WP, 0, WP)
    p.ADDP  (RS, ST, TP)
    p.SQ    (TR, TP, LnOffset)
}

func translate_OP_list_if_next(p *hir.Builder, v Instr) {
    p.ADDP  (RS, ST, TP)
    p.LQ    (TP, LnOffset, TR)
    p.BNE   (TR, hir.Rz, p.At(v.To))
}

func translate_OP_list_if_empty(p *hir.Builder, v Instr) {
    p.LQ    (WP, abi.PtrSize, TR)
    p.BEQ   (TR, hir.Rz, p.At(v.To))
}

func translate_OP_unique(p *hir.Builder, v Instr) {
    p.IB    (2, UR)
    p.LQ    (WP, abi.PtrSize, TR)
    p.BLTU  (TR, UR, "_ok_{n}")
    translate_OP_unique_type(p, v.Vt)
    p.Label ("_ok_{n}")
}

func translate_OP_unique_type(p *hir.Builder, vt *rt.GoType) {
    switch vt.Kind() {
        case reflect.Bool    : translate_OP_unique_b(p)
        case reflect.Int     : translate_OP_unique_int(p)
        case reflect.Int8    : translate_OP_unique_i8(p)
        case reflect.Int16   : translate_OP_unique_i16(p)
        case reflect.Int32   : translate_OP_unique_i32(p)
        case reflect.Int64   : translate_OP_unique_i64(p)
        case reflect.Float64 : translate_OP_unique_i64(p)
        case reflect.Map     : break
        case reflect.Ptr     : break
        case reflect.Slice   : break
        case reflect.String  : translate_OP_unique_str(p)
        case reflect.Struct  : break
        default              : panic("unique: invalid type: " + vt.String())
    }
}

func translate_OP_unique_b(p *hir.Builder) {
    p.BLTU  (UR, TR, LB_duplicated)
    p.LP    (WP, 0, TP)
    p.LB    (TP, 0, TR)
    p.LB    (TP, 1, UR)
    p.BEQ   (TR, UR, LB_duplicated)
}

func translate_OP_unique_i8(p *hir.Builder) {
    p.IQ    (RangeUint8, UR)
    p.BLTU  (UR, TR, LB_duplicated)
    translate_OP_unique_small(p, RangeUint8 / 8, 1, p.LB)
}

func translate_OP_unique_i16(p *hir.Builder) {
    p.IQ    (RangeUint16, UR)
    p.BLTU  (UR, TR, LB_duplicated)
    translate_OP_unique_small(p, RangeUint16 / 8, 2, p.LW)
}

func translate_OP_unique_int(p *hir.Builder) {
    switch defs.IntSize {
        case 4  : translate_OP_unique_i32(p)
        case 8  : translate_OP_unique_i64(p)
        default : panic("invalid int size")
    }
}

func translate_OP_unique_small(p *hir.Builder, nb int64, dv int64, ld func(hir.PointerRegister, int64, hir.GenericRegister) *hir.Ir) {
    p.ADDPI (RS, BmOffset, ET)
    p.BZERO (nb, ET)
    p.LP    (WP, 0, EP)
    p.JMP   ("_first_{n}")
    p.Label ("_loop_{n}")
    p.ADDPI (EP, dv, EP)
    p.Label ("_first_{n}")
    ld      (EP, 0, RC)
    p.SHRI  (RC, 3, UR)
    p.ANDI  (RC, 0x3f, RC)
    p.ANDI  (UR, ^0x3f, UR)
    p.ADDP  (ET, UR, TP)
    p.LQ    (TP, 0, UR)
    p.BTS   (RC, UR, RC)
    p.SQ    (UR, TP, 0)
    p.BNE   (RC, hir.Rz, LB_duplicated)
    p.SUBI  (TR, 1, TR)
    p.BNE   (TR, hir.Rz, "_loop_{n}")
    p.LDAQ  (ARG_len, RC)
}

func translate_OP_unique_i32(p *hir.Builder) {
    p.LP    (WP, 0, TP)
    p.GCALL (F_unique32).
      A0    (TP).
      A1    (TR).
      R0    (TR)
    p.BNE   (TR, hir.Rz, LB_duplicated)
}

func translate_OP_unique_i64(p *hir.Builder) {
    p.LP    (WP, 0, TP)
    p.GCALL (F_unique64).
      A0    (TP).
      A1    (TR).
      R0    (TR)
    p.BNE   (TR, hir.Rz, LB_duplicated)
}

func translate_OP_unique_str(p *hir.Builder) {
    p.LP    (WP, 0, TP)
    p.GCALL (F_uniquestr).
      A0    (TP).
      A1    (TR).
      R0    (TR)
    p.BNE   (TR, hir.Rz, LB_duplicated)
}

func translate_OP_goto(p *hir.Builder, v Instr) {
    p.JMP   (p.At(v.To))
}

func translate_OP_if_nil(p *hir.Builder, v Instr) {
    p.LP    (WP, 0, TP)
    p.BEQN  (TP, p.At(v.To))
}

func translate_OP_if_hasbuf(p *hir.Builder, v Instr) {
    p.BNEN  (RP, p.At(v.To))
}

func translate_OP_make_state(p *hir.Builder, _ Instr) {
    p.IQ    (StateMax, TR)
    p.BGEU  (ST, TR, LB_overflow)
    p.ADDP  (RS, ST, TP)
    p.SP    (WP, TP, WpOffset)
    p.ADDI  (ST, StateSize, ST)
}

func translate_OP_drop_state(p *hir.Builder, _ Instr) {
    p.SUBI  (ST, StateSize, ST)
    p.ADDP  (RS, ST, TP)
    p.LP    (TP, WpOffset, WP)
    p.SP    (hir.Pn, TP, WpOffset)
}

func translate_OP_halt(p *hir.Builder, _ Instr) {
    p.JMP   (LB_halt)
}
