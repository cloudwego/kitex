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
    `reflect`

    `github.com/cloudwego/frugal/internal/utils`
)

const (
    IntSize   = 4 << (^uint(0) >> 63)
    StackSize = 65536
)

func GetSize(vt reflect.Type) int {
    switch vt.Kind() {
        case reflect.Bool    : return 1
        case reflect.Int     : return IntSize
        case reflect.Int8    : return 1
        case reflect.Int16   : return 2
        case reflect.Int32   : return 4
        case reflect.Int64   : return measureInt64(vt)
        case reflect.Uint    : panic(utils.ENotSupp(vt, "int"))
        case reflect.Uint8   : panic(utils.ENotSupp(vt, "int8"))
        case reflect.Uint16  : panic(utils.ENotSupp(vt, "int16"))
        case reflect.Uint32  : panic(utils.ENotSupp(vt, "int32"))
        case reflect.Uint64  : panic(utils.ENotSupp(vt, "int64"))
        case reflect.Float32 : panic(utils.ENotSupp(vt, "float64"))
        case reflect.Float64 : return 8
        case reflect.Array   : panic(utils.ENotSupp(vt, "[]" + vt.Elem().String()))
        case reflect.Map     : return -1
        case reflect.Ptr     : return -1
        case reflect.Slice   : return -1
        case reflect.String  : return -1
        case reflect.Struct  : return measureStruct(vt)
        default              : panic("invalid type tag")
    }
}

func measureInt64(vt reflect.Type) int {
    if vt == i64type {
        return 8
    } else {
        return 4
    }
}

func measureStruct(vt reflect.Type) int {
    var fs int
    var rs int

    /* measure each field, plus the 3-byte field header */
    for i := 0; i < vt.NumField(); i++ {
        if fs = GetSize(vt.Field(i).Type); fs > 0 {
            rs += fs + 3
        } else {
            return -1
        }
    }

    /* all fields have fixed size, plus the STOP field */
    return rs + 1
}
