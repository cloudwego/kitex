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

package pgen

import (
    `math`

    `github.com/chenzhuoyu/iasm/x86_64`
)

const (
    AL    = x86_64.AL
    AX    = x86_64.AX
    EAX   = x86_64.EAX
    RAX   = x86_64.RAX
    RCX   = x86_64.RCX
    RDX   = x86_64.RDX
    RBX   = x86_64.RBX
    RSP   = x86_64.RSP
    RBP   = x86_64.RBP
    RSI   = x86_64.RSI
    RDI   = x86_64.RDI
    R8    = x86_64.R8
    R9    = x86_64.R9
    R10   = x86_64.R10
    R11   = x86_64.R11
    R12   = x86_64.R12
    R13   = x86_64.R13
    R14   = x86_64.R14
    R15   = x86_64.R15
    XMM15 = x86_64.XMM15
)

var allocationOrder = [11]x86_64.Register64 {
    R12, R13, R14, R15,         // reserved registers first (except for RBX)
    R10, R11,                   // then scratch registers
    R9, R8, RCX, RDX,           // then argument registers in reverse order (RDI, RSI and RAX are always free)
    RBX,                        // finally the RBX, we put RBX here to reduce collision with Go register ABI
}

func Ptr(base x86_64.Register, disp int32) *x86_64.MemoryOperand {
    return x86_64.Ptr(base, disp)
}

func Sib(base x86_64.Register, index x86_64.Register64, scale uint8, disp int32) *x86_64.MemoryOperand {
    return x86_64.Sib(base, index, scale, disp)
}

func isPow2(v int64) bool {
    return v & (v - 1) == 0
}

func isInt32(v int64) bool {
    return v >= math.MinInt32 && v <= math.MaxInt32
}

func isReg64(v x86_64.Register) (ok bool) {
    _, ok = v.(x86_64.Register64)
    return
}

func toAddress(p *x86_64.Label) int {
    if v, err := p.Evaluate(); err != nil {
        panic(err)
    } else {
        return int(v)
    }
}

func isSimpleMem(v *x86_64.MemoryOperand) bool {
    return !v.Masked                        &&
            v.Broadcast == 0                &&
            v.Addr.Type == x86_64.Memory    &&
            v.Addr.Offset == 0              &&
            v.Addr.Reference == nil         &&
            v.Addr.Memory.Scale <= 1        &&
            v.Addr.Memory.Index == nil      &&
            v.Addr.Memory.Displacement == 0 &&
            isReg64(v.Addr.Memory.Base)
}
