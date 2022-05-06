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

package rt

import (
    `reflect`
)

type Method struct {
    Id int
    Vt *GoType
}

func AsMethod(id int, vt *GoType) Method {
    return Method {
        Id: id,
        Vt: vt,
    }
}

func GetMethod(tp interface{}, name string) Method {
    if tp == nil {
        panic("value must be an interface pointer")
    } else if vt := reflect.TypeOf(tp); vt.Kind() != reflect.Ptr {
        panic("value must be an interface pointer")
    } else if et := vt.Elem(); et.Kind() != reflect.Interface {
        panic("value must be an interface pointer")
    } else if mm, ok := et.MethodByName(name); !ok {
        panic("interface " + vt.Elem().String() + " does not have method " + name)
    } else {
        return AsMethod(mm.Index, UnpackType(et))
    }
}
