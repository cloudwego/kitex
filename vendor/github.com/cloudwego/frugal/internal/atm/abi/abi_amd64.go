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

package abi

import (
    `fmt`
    `reflect`
    `sort`
    `strings`
    `unsafe`

    `github.com/chenzhuoyu/iasm/x86_64`
    `github.com/cloudwego/frugal/internal/rt`
)

const (
    PtrSize  = 8    // pointer size
    PtrAlign = 8    // pointer alignment
)

type Parameter struct {
    Mem        uintptr
    Reg        x86_64.Register64
    Type       reflect.Type
    InRegister bool
}

var (
    intType = reflect.TypeOf(0)
    ptrType = reflect.TypeOf(unsafe.Pointer(nil))
)

func mkReg(vt reflect.Type, reg x86_64.Register64) (p Parameter) {
    p.Reg = reg
    p.Type = vt
    p.InRegister = true
    return
}

func mkStack(vt reflect.Type, mem uintptr) (p Parameter) {
    p.Mem = mem
    p.Type = vt
    p.InRegister = false
    return
}

func (self Parameter) String() string {
    if self.InRegister {
        return fmt.Sprintf("%%%s", self.Reg)
    } else {
        return fmt.Sprintf("%d(%%rsp)", self.Mem)
    }
}

func (self Parameter) IsPointer() bool {
    switch self.Type.Kind() {
        case reflect.Bool          : fallthrough
        case reflect.Int           : fallthrough
        case reflect.Int8          : fallthrough
        case reflect.Int16         : fallthrough
        case reflect.Int32         : fallthrough
        case reflect.Int64         : fallthrough
        case reflect.Uint          : fallthrough
        case reflect.Uint8         : fallthrough
        case reflect.Uint16        : fallthrough
        case reflect.Uint32        : fallthrough
        case reflect.Uint64        : fallthrough
        case reflect.Uintptr       : return false
        case reflect.Chan          : fallthrough
        case reflect.Func          : fallthrough
        case reflect.Map           : fallthrough
        case reflect.Ptr           : fallthrough
        case reflect.UnsafePointer : return true
        case reflect.Float32       : fallthrough
        case reflect.Float64       : fallthrough
        case reflect.Complex64     : fallthrough
        case reflect.Complex128    : fallthrough
        case reflect.Array         : fallthrough
        case reflect.Struct        : panic("abi: unsupported types")
        default                    : panic("abi: invalid value type")
    }
}

type _StackSlot struct {
    p bool
    m uintptr
}

type FunctionLayout struct {
    Id   int
    Sp   uintptr
    Args []Parameter
    Rets []Parameter
}

func (self *FunctionLayout) String() string {
    if self.Id < 0 {
        return fmt.Sprintf("{func,%s}", self.formatFn())
    } else {
        return fmt.Sprintf("{meth/%d,%s}", self.Id, self.formatFn())
    }
}

func (self *FunctionLayout) StackMap() *rt.StackMap {
    var st []_StackSlot
    var mb rt.StackMapBuilder

    /* add arguments */
    for _, v := range self.Args {
        st = append(st, _StackSlot{
            m: v.Mem,
            p: v.IsPointer(),
        })
    }

    /* add stack-passed return values */
    for _, v := range self.Rets {
        if !v.InRegister {
            st = append(st, _StackSlot{
                m: v.Mem,
                p: v.IsPointer(),
            })
        }
    }

    /* sort by memory offset */
    sort.Slice(st, func(i int, j int) bool {
        return st[i].m < st[j].m
    })

    /* add the bits */
    for _, v := range st {
        mb.AddField(v.p)
    }

    /* build the stack map */
    return mb.Build()
}

func (self *FunctionLayout) formatFn() string {
    return fmt.Sprintf("$%#x,(%s),(%s)", self.Sp, self.formatSeq(self.Args), self.formatSeq(self.Rets))
}

func (self *FunctionLayout) formatSeq(v []Parameter) string {
    nb := len(v)
    mm := make([]string, len(v))

    /* convert each part */
    for i := 0; i < nb; i++ {
        mm[i] = v[i].String()
    }

    /* join them together */
    return strings.Join(mm, ",")
}

type AMD64ABI struct {
    FnTab map[int]*FunctionLayout
}

func ArchCreateABI() *AMD64ABI {
    return &AMD64ABI {
        FnTab: make(map[int]*FunctionLayout),
    }
}

func (self *AMD64ABI) RegisterMethod(id int, mt rt.Method) int {
    self.FnTab[id] = self.LayoutFunc(mt.Id, mt.Vt.Pack().Method(mt.Id).Type)
    return mt.Id
}

func (self *AMD64ABI) RegisterFunction(id int, fn interface{}) (fp uintptr) {
    vv := rt.UnpackEface(fn)
    vt := vv.Type.Pack()

    /* must be a function */
    if vt.Kind() != reflect.Func {
        panic("fn is not a function")
    }

    /* layout the function, and get the real function address */
    self.FnTab[id] = self.LayoutFunc(-1, vt)
    return *(*uintptr)(vv.Value)
}
