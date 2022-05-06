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

package defs

import (
    `fmt`
    `reflect`
    `strings`
    `sync`
    `unicode`

    `github.com/cloudwego/frugal/internal/utils`
)

type Tag uint8

const (
    T_bool    Tag = 2
    T_i8      Tag = 3
    T_double  Tag = 4
    T_i16     Tag = 6
    T_i32     Tag = 8
    T_i64     Tag = 10
    T_string  Tag = 11
    T_struct  Tag = 12
    T_map     Tag = 13
    T_set     Tag = 14
    T_list    Tag = 15
    T_enum    Tag = 0x80
    T_binary  Tag = 0x81
    T_pointer Tag = 0x82
)

var wireTags = [256]bool {
    T_bool   : true,
    T_i8     : true,
    T_double : true,
    T_i16    : true,
    T_i32    : true,
    T_i64    : true,
    T_string : true,
    T_struct : true,
    T_map    : true,
    T_set    : true,
    T_list   : true,
}

var keywordTab = [256]string {
    T_bool   : "bool",
    T_i8     : "i8 or byte",
    T_double : "double",
    T_i16    : "i16",
    T_i32    : "i32",
    T_i64    : "i64",
    T_string : "string",
    T_binary : "binary",
    T_struct : "struct",
    T_map    : "map",
}

var (
    i64type = reflect.TypeOf(int64(0))
)

func T_int() Tag {
    switch IntSize {
        case 4  : return T_i32
        case 8  : return T_i64
        default : panic("invalid int size")
    }
}

func (self Tag) IsWireTag() bool {
    return wireTags[self]
}

type Type struct {
    T Tag
    K *Type
    V *Type
    S reflect.Type
}

var (
    typePool sync.Pool
)

func newType() *Type {
    if v := typePool.Get(); v == nil {
        return new(Type)
    } else {
        return resetType(v.(*Type))
    }
}

func resetType(p *Type) *Type {
    *p = Type{}
    return p
}

func (self *Type) Tag() Tag {
    switch self.T {
        case T_enum    : return T_i32
        case T_binary  : return T_string
        case T_pointer : return self.V.Tag()
        default        : return self.T
    }
}

func (self *Type) Free() {
    typePool.Put(self)
}

func (self *Type) String() string {
    switch self.T {
        case T_bool    : return "bool"
        case T_i8      : return "i8"
        case T_double  : return "double"
        case T_i16     : return "i16"
        case T_i32     : return "i32"
        case T_i64     : return "i64"
        case T_string  : return "string"
        case T_struct  : return self.S.Name()
        case T_map     : return fmt.Sprintf("map<%s:%s>", self.K.String(), self.V.String())
        case T_set     : return fmt.Sprintf("set<%s>", self.V.String())
        case T_list    : return fmt.Sprintf("list<%s>", self.V.String())
        case T_enum    : return "enum"
        case T_binary  : return "binary"
        case T_pointer : return "*" + self.V.String()
        default        : return fmt.Sprintf("Type(Tag(%d))", self.T)
    }
}

func (self *Type) isKeyType() bool {
    switch self.T {
        case T_bool    : return true
        case T_i8      : return true
        case T_double  : return true
        case T_i16     : return true
        case T_i32     : return true
        case T_i64     : return true
        case T_string  : return true
        case T_enum    : return true
        case T_pointer : return self.V.T == T_struct
        default        : return false
    }
}

func ParseType(vt reflect.Type, def string) *Type {
    var i int
    return doParseType(vt, def, &i, true)
}

func isident(c byte) bool {
    return isident0(c) || c >= '0' && c <= '9'
}

func isident0(c byte) bool {
    return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func nextToken(src string, i *int) string {
    return readToken(src, i, false)
}

func readToken(src string, i *int, eofok bool) string {
    p := *i
    n := len(src)

    /* skip the spaces */
    for p < n && unicode.IsSpace(rune(src[p])) {
        p++
    }

    /* check for EOF */
    if p == n {
        if eofok {
            return ""
        } else {
            panic(utils.ESyntax(p, src, "unexpected EOF"))
        }
    }

    /* skip the character */
    q := p
    p++

    /* check for identifiers */
    if isident0(src[q]) {
        for p < n && isident(src[p]) {
            p++
        }
    }

    /* slice the source */
    *i = p
    return src[q:p]
}

func mkMistyped(pos int, src string, tv string, tag Tag, vt reflect.Type) utils.SyntaxError {
    if tag != T_struct {
        return utils.ESyntax(pos, src, fmt.Sprintf("type mismatch, %s expected, got %s", keywordTab[tag], tv))
    } else {
        return utils.ESyntax(pos, src, fmt.Sprintf("struct name mismatch, %s expected, got %s", vt.Name(), tv))
    }
}

func doParseType(vt reflect.Type, def string, i *int, allowPtrs bool) *Type {
    tag := Tag(0)
    ret := newType()

    /* dereference the pointer if possible */
    if allowPtrs && vt.Kind() == reflect.Ptr {
        ret.S = vt
        ret.T = T_pointer
        ret.V = doParseType(vt.Elem(), def, i, false)
        return ret
    }

    /* check for value kind */
    switch vt.Kind() {
        case reflect.Bool    : tag = T_bool
        case reflect.Int     : tag = T_int()
        case reflect.Int8    : tag = T_i8
        case reflect.Int16   : tag = T_i16
        case reflect.Int32   : tag = T_i32
        case reflect.Int64   : tag = T_i64
        case reflect.Uint    : panic(utils.ENotSupp(vt, "int"))
        case reflect.Uint8   : panic(utils.ENotSupp(vt, "int8"))
        case reflect.Uint16  : panic(utils.ENotSupp(vt, "int16"))
        case reflect.Uint32  : panic(utils.ENotSupp(vt, "int32"))
        case reflect.Uint64  : panic(utils.ENotSupp(vt, "int64"))
        case reflect.Float32 : panic(utils.ENotSupp(vt, "float64"))
        case reflect.Float64 : tag = T_double
        case reflect.Array   : panic(utils.ENotSupp(vt, "[]" + vt.Elem().String()))
        case reflect.Map     : tag = T_map
        case reflect.Slice   : break
        case reflect.String  : tag = T_string
        case reflect.Struct  : tag = T_struct
        default              : panic(utils.EType(vt))
    }

    /* it's a slice, check for byte slice */
    if tag == 0 {
        if et := vt.Elem(); utils.IsByteType(et) {
            tag = T_binary
        } else if def == "" {
            panic(utils.ESetList(*i, def, et))
        } else {
            return doParseSlice(vt, et, def, i, ret)
        }
    }

    /* match the type if any */
    if def != "" {
        if tv := nextToken(def, i); !strings.Contains(keywordTab[tag], tv) {
            if !isident0(tv[0]) || !doMatchStruct(vt, def, i, &tv) {
                panic(mkMistyped(*i - len(tv), def, tv, tag, vt))
            } else if tag == T_i64 && vt != i64type {
                tag = T_enum
            }
        }
    }

    /* simple types */
    if tag != T_map {
        ret.S = vt
        ret.T = tag
        return ret
    }

    /* map begin */
    if def != "" {
        if tk := nextToken(def, i); tk != "<" {
            panic(utils.ESyntax(*i - len(tk), def, "'<' expected"))
        }
    }

    /* parse the key type */
    vi := *i
    kt := vt.Key()
    ret.K = doParseType(kt, def, i, true)

    /* validate map key */
    if !ret.K.isKeyType() {
        panic(utils.ESyntax(vi, def, fmt.Sprintf("'%s' is not a valid map key", ret.K)))
    }

    /* key-value delimiter */
    if def != "" {
        if tk := nextToken(def, i); tk != ":" {
            panic(utils.ESyntax(*i - len(tk), def, "':' expected"))
        }
    }

    /* parse the value type */
    et := vt.Elem()
    ret.V = doParseType(et, def, i, true)

    /* map end */
    if def != "" {
        if tk := nextToken(def, i); tk != ">" {
            panic(utils.ESyntax(*i - len(tk), def, "'>' expected"))
        }
    }

    /* set the type tag */
    ret.S = vt
    ret.T = T_map
    return ret
}

func doParseSlice(vt reflect.Type, et reflect.Type, def string, i *int, rt *Type) *Type {
    tk := nextToken(def, i)
    tp := *i - len(tk)

    /* identify "set" or "list" */
    if tk == "set" {
        rt.T = T_set
    } else if tk == "list" {
        rt.T = T_list
    } else {
        panic(utils.ESyntax(tp, def, `"set" or "list" expected`))
    }

    /* list or set begin */
    if tk = nextToken(def, i); tk != "<" {
        panic(utils.ESyntax(*i - len(tk), def, "'<' expected"))
    }

    /* set or list element */
    rt.V = doParseType(et, def, i, true)
    tk   = nextToken(def, i)

    /* list or set end */
    if tk != ">" {
        panic(utils.ESyntax(*i - len(tk), def, "'>' expected"))
    }

    /* set the type */
    rt.S = vt
    return rt
}

func doMatchStruct(vt reflect.Type, def string, i *int, tv *string) bool {
    sp := *i
    tn := vt.Name()
    tk := readToken(def, &sp, true)

    /* just a simple type with no qualifiers */
    if tk == "" || tk == ":" || tk == ">" {
        return tn == *tv
    }

    /* otherwise, it must be a "." */
    if tk != "." {
        panic(utils.ESyntax(sp, def, "'.' or '>' expected"))
    }

    /* must be an identifier */
    if *tv = nextToken(def, &sp); !isident0((*tv)[0]) {
        panic(utils.ESyntax(sp, def, "struct name expected"))
    }

    /* update parsing position */
    *i = sp
    return tn == *tv
}
