// +build go1.17

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

/** Go Internal ABI implementation
 *
 *  This module implements the function layout algorithm described by the Go internal ABI.
 *  See https://github.com/golang/go/blob/master/src/cmd/compile/abi-internal.md for more info.
 */

package abi

import (
    `reflect`

    `github.com/chenzhuoyu/iasm/x86_64`
)

var regOrder = [...]x86_64.Register64 {
    x86_64.RAX,
    x86_64.RBX,
    x86_64.RCX,
    x86_64.RDI,
    x86_64.RSI,
    x86_64.R8,
    x86_64.R9,
    x86_64.R10,
    x86_64.R11,
}

type _StackAlloc struct {
    i int
    s uintptr
}

func (self *_StackAlloc) reg(vt reflect.Type) (p Parameter) {
    p = mkReg(vt, regOrder[self.i])
    self.i++
    return
}

func (self *_StackAlloc) stack(vt reflect.Type) (p Parameter) {
    p = mkStack(vt, self.s)
    self.s += vt.Size()
    return
}

func (self *_StackAlloc) spill(n uintptr, a int) uintptr {
    self.s = alignUp(self.s, a) + n
    return self.s
}

func (self *_StackAlloc) alloc(p []Parameter, vt reflect.Type) []Parameter {
    nb := vt.Size()
    vk := vt.Kind()

    /* zero-sized objects are allocated on stack */
    if nb == 0 {
        return append(p, mkStack(intType, self.s))
    }

    /* check for value type */
    switch vk {
        case reflect.Bool          : return self.valloc(p, reflect.TypeOf(false))
        case reflect.Int           : return self.valloc(p, intType)
        case reflect.Int8          : return self.valloc(p, reflect.TypeOf(int8(0)))
        case reflect.Int16         : return self.valloc(p, reflect.TypeOf(int16(0)))
        case reflect.Int32         : return self.valloc(p, reflect.TypeOf(int32(0)))
        case reflect.Int64         : return self.valloc(p, reflect.TypeOf(int64(0)))
        case reflect.Uint          : return self.valloc(p, reflect.TypeOf(uint(0)))
        case reflect.Uint8         : return self.valloc(p, reflect.TypeOf(uint8(0)))
        case reflect.Uint16        : return self.valloc(p, reflect.TypeOf(uint16(0)))
        case reflect.Uint32        : return self.valloc(p, reflect.TypeOf(uint32(0)))
        case reflect.Uint64        : return self.valloc(p, reflect.TypeOf(uint64(0)))
        case reflect.Uintptr       : return self.valloc(p, reflect.TypeOf(uintptr(0)))
        case reflect.Float32       : panic("abi: go117: not implemented: float32")
        case reflect.Float64       : panic("abi: go117: not implemented: float64")
        case reflect.Complex64     : panic("abi: go117: not implemented: complex64")
        case reflect.Complex128    : panic("abi: go117: not implemented: complex128")
        case reflect.Array         : panic("abi: go117: not implemented: arrays")
        case reflect.Chan          : return self.valloc(p, reflect.TypeOf((chan int)(nil)))
        case reflect.Func          : return self.valloc(p, reflect.TypeOf((func())(nil)))
        case reflect.Map           : return self.valloc(p, reflect.TypeOf((map[int]int)(nil)))
        case reflect.Ptr           : return self.valloc(p, reflect.TypeOf((*int)(nil)))
        case reflect.UnsafePointer : return self.valloc(p, ptrType)
        case reflect.Interface     : return self.valloc(p, ptrType, ptrType)
        case reflect.Slice         : return self.valloc(p, ptrType, intType, intType)
        case reflect.String        : return self.valloc(p, ptrType, intType)
        case reflect.Struct        : panic("abi: go117: not implemented: structs")
        default                    : panic("abi: invalid value type")
    }
}

func (self *_StackAlloc) ralloc(p []Parameter, vts ...reflect.Type) []Parameter {
    for _, vt := range vts { p = append(p, self.reg(vt)) }
    return p
}

func (self *_StackAlloc) salloc(p []Parameter, vts ...reflect.Type) []Parameter {
    for _, vt := range vts { p = append(p, self.stack(vt)) }
    return p
}

func (self *_StackAlloc) valloc(p []Parameter, vts ...reflect.Type) []Parameter {
    if self.i + len(vts) <= len(regOrder) {
        return self.ralloc(p, vts...)
    } else {
        return self.salloc(p, vts...)
    }
}

func (self *AMD64ABI) Reserved() map[x86_64.Register64]int32 {
    return map[x86_64.Register64]int32 {
        x86_64.R14: 0, // current goroutine
        x86_64.R15: 1, // GOT reference
    }
}

func (self *AMD64ABI) LayoutFunc(id int, ft reflect.Type) *FunctionLayout {
    var sa _StackAlloc
    var fn FunctionLayout

    /* allocate the receiver if any (interface call always uses pointer) */
    if id >= 0 {
        fn.Args = sa.alloc(fn.Args, ptrType)
    }

    /* assign every arguments */
    for i := 0; i < ft.NumIn(); i++ {
        fn.Args = sa.alloc(fn.Args, ft.In(i))
    }

    /* reset the register counter, and add a pointer alignment field */
    sa.i = 0
    sa.spill(0, PtrAlign)

    /* assign every return value */
    for i := 0; i < ft.NumOut(); i++ {
        fn.Rets = sa.alloc(fn.Rets, ft.Out(i))
    }

    /* assign spill slots */
    for i := 0; i < len(fn.Args); i++ {
        if fn.Args[i].InRegister {
            fn.Args[i].Mem = sa.spill(PtrSize, PtrAlign) - PtrSize
        }
    }

    /* add the final pointer alignment field */
    fn.Id = id
    fn.Sp = sa.spill(0, PtrAlign)
    return &fn
}
