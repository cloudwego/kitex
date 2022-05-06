// +build !go1.17

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
    `reflect`

    `github.com/chenzhuoyu/iasm/x86_64`
)

func salloc(p []Parameter, sp uintptr, vt reflect.Type) (uintptr, []Parameter) {
    switch vt.Kind() {
        case reflect.Bool          : return sp + 8, append(p, mkStack(reflect.TypeOf(false), sp))
        case reflect.Int           : return sp + 8, append(p, mkStack(intType, sp))
        case reflect.Int8          : return sp + 8, append(p, mkStack(reflect.TypeOf(int8(0)), sp))
        case reflect.Int16         : return sp + 8, append(p, mkStack(reflect.TypeOf(int16(0)), sp))
        case reflect.Int32         : return sp + 8, append(p, mkStack(reflect.TypeOf(int32(0)), sp))
        case reflect.Int64         : return sp + 8, append(p, mkStack(reflect.TypeOf(int64(0)), sp))
        case reflect.Uint          : return sp + 8, append(p, mkStack(reflect.TypeOf(uint(0)), sp))
        case reflect.Uint8         : return sp + 8, append(p, mkStack(reflect.TypeOf(uint8(0)), sp))
        case reflect.Uint16        : return sp + 8, append(p, mkStack(reflect.TypeOf(uint16(0)), sp))
        case reflect.Uint32        : return sp + 8, append(p, mkStack(reflect.TypeOf(uint32(0)), sp))
        case reflect.Uint64        : return sp + 8, append(p, mkStack(reflect.TypeOf(uint64(0)), sp))
        case reflect.Uintptr       : return sp + 8, append(p, mkStack(reflect.TypeOf(uintptr(0)), sp))
        case reflect.Float32       : panic("abi: go116: not implemented: float32")
        case reflect.Float64       : panic("abi: go116: not implemented: float64")
        case reflect.Complex64     : panic("abi: go116: not implemented: complex64")
        case reflect.Complex128    : panic("abi: go116: not implemented: complex128")
        case reflect.Array         : panic("abi: go116: not implemented: arrays")
        case reflect.Chan          : return sp +  8, append(p, mkStack(reflect.TypeOf((chan int)(nil)), sp))
        case reflect.Func          : return sp +  8, append(p, mkStack(reflect.TypeOf((func())(nil)), sp))
        case reflect.Map           : return sp +  8, append(p, mkStack(reflect.TypeOf((map[int]int)(nil)), sp))
        case reflect.Ptr           : return sp +  8, append(p, mkStack(reflect.TypeOf((*int)(nil)), sp))
        case reflect.UnsafePointer : return sp +  8, append(p, mkStack(ptrType, sp))
        case reflect.Interface     : return sp + 16, append(p, mkStack(ptrType, sp), mkStack(ptrType, sp + 8))
        case reflect.Slice         : return sp + 24, append(p, mkStack(ptrType, sp), mkStack(intType, sp + 8), mkStack(intType, sp + 16))
        case reflect.String        : return sp + 16, append(p, mkStack(ptrType, sp), mkStack(intType, sp + 8))
        case reflect.Struct        : panic("abi: go116: not implemented: structs")
        default                    : panic("abi: invalid value type")
    }
}

func (self *AMD64ABI) Reserved() map[x86_64.Register64]int32 {
    return nil
}

func (self *AMD64ABI) LayoutFunc(id int, ft reflect.Type) *FunctionLayout {
    var sp uintptr
    var fn FunctionLayout

    /* allocate the receiver if any (interface call always uses pointer) */
    if id >= 0 {
        sp, fn.Args = salloc(fn.Args, sp, ptrType)
    }

    /* assign every arguments */
    for i := 0; i < ft.NumIn(); i++ {
        sp, fn.Args = salloc(fn.Args, sp, ft.In(i))
    }

    /* assign every return value */
    for i := 0; i < ft.NumOut(); i++ {
        sp, fn.Rets = salloc(fn.Rets, sp, ft.Out(i))
    }

    /* update function ID and stack pointer */
    fn.Id = id
    fn.Sp = sp
    return &fn
}
