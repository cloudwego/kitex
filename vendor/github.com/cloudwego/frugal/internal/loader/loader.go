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

package loader

import (
    `fmt`
    `os`
    `syscall`
    `unsafe`

    `github.com/cloudwego/frugal/internal/rt`
)

const (
    _AP = syscall.MAP_ANON  | syscall.MAP_PRIVATE
    _RX = syscall.PROT_READ | syscall.PROT_EXEC
    _RW = syscall.PROT_READ | syscall.PROT_WRITE
)

type Loader   []byte
type Function unsafe.Pointer

func mkptr(m uintptr) unsafe.Pointer {
    return *(*unsafe.Pointer)(unsafe.Pointer(&m))
}

func alignUp(n uintptr, a int) uintptr {
    return (n + uintptr(a) - 1) &^ (uintptr(a) - 1)
}

func (self Loader) Load(fn string, frame rt.Frame) (f Function) {
    var mm uintptr
    var er syscall.Errno

    /* align the size to pages */
    nf := uintptr(len(self))
    nb := alignUp(nf, os.Getpagesize())

    /* allocate a block of memory */
    if mm, _, er = syscall.Syscall6(syscall.SYS_MMAP, 0, nb, _RW, _AP, 0, 0); er != 0 {
        panic(er)
    }

    /* copy code into the memory, and register the function */
    copy(rt.BytesFrom(mkptr(mm), len(self), int(nb)), self)
    registerFunction(fmt.Sprintf("(frugal).%s_%x", fn, mm), mm, nf, frame)

    /* make it executable */
    if _, _, err := syscall.Syscall(syscall.SYS_MPROTECT, mm, nb, _RX); err != 0 {
        panic(err)
    } else {
        return Function(&mm)
    }
}
