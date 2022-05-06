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
    `unsafe`

    `github.com/cloudwego/frugal/internal/rt`
    `github.com/cloudwego/frugal/internal/utils`
    `github.com/cloudwego/frugal/iov`
)

type Encoder func (
    buf unsafe.Pointer,
    len int,
    mem iov.BufferWriter,
    p   unsafe.Pointer,
    rs  *RuntimeState,
    st  int,
) (int, error)

var (
    programCache = utils.CreateProgramCache()
)

func encode(vt *rt.GoType, buf unsafe.Pointer, len int, mem iov.BufferWriter, p unsafe.Pointer, rs *RuntimeState, st int) (int, error) {
    if enc, err := resolve(vt); err != nil {
        return -1, err
    } else {
        return enc(buf, len, mem, p, rs, st)
    }
}

func resolve(vt *rt.GoType) (Encoder, error) {
    if val := programCache.Get(vt); val != nil {
        return val.(Encoder), nil
    } else if ret, err := programCache.Compute(vt, compile); err == nil {
        return ret.(Encoder), nil
    } else {
        return nil, err
    }
}

func compile(vt *rt.GoType) (interface{}, error) {
    if pp, err := CreateCompiler().CompileAndFree(vt.Pack()); err != nil {
        return nil, err
    } else {
        return Link(Translate(pp)), nil
    }
}

func EncodedSize(val interface{}) int {
    if ret, err := EncodeObject(nil, nil, val); err != nil {
        panic(fmt.Errorf("frugal: cannot measure encoded size: %w", err))
    } else {
        return ret
    }
}

func EncodeObject(buf []byte, mem iov.BufferWriter, val interface{}) (ret int, err error) {
    rst := newRuntimeState()
    efv := rt.UnpackEface(val)
    out := (*rt.GoSlice)(unsafe.Pointer(&buf))

    /* check for indirect types */
    if efv.Type.IsIndirect() {
        ret, err = encode(efv.Type, out.Ptr, out.Len, mem, efv.Value, rst, 0)
    } else {
        ret, err = encode(efv.Type, out.Ptr, out.Len, mem, rt.NoEscape(unsafe.Pointer(&efv.Value)), rst, 0)
    }

    /* return the state into pool */
    freeRuntimeState(rst)
    return
}
