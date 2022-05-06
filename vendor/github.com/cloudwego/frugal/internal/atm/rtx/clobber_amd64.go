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

package rtx

import (
    `unsafe`

    `github.com/chenzhuoyu/iasm/x86_64`
    `github.com/cloudwego/frugal/internal/rt`
    `github.com/oleiade/lane`
    `golang.org/x/arch/x86/x86asm`
)

var branchTable = map[x86asm.Op]bool {
    x86asm.JA    : true,
    x86asm.JAE   : true,
    x86asm.JB    : true,
    x86asm.JBE   : true,
    x86asm.JCXZ  : true,
    x86asm.JE    : true,
    x86asm.JECXZ : true,
    x86asm.JG    : true,
    x86asm.JGE   : true,
    x86asm.JL    : true,
    x86asm.JLE   : true,
    x86asm.JMP   : true,
    x86asm.JNE   : true,
    x86asm.JNO   : true,
    x86asm.JNP   : true,
    x86asm.JNS   : true,
    x86asm.JO    : true,
    x86asm.JP    : true,
    x86asm.JRCXZ : true,
    x86asm.JS    : true,
}

var registerTable = map[x86asm.Reg]x86_64.Register64 {
    x86asm.AL   : x86_64.RAX,
    x86asm.CL   : x86_64.RCX,
    x86asm.DL   : x86_64.RDX,
    x86asm.BL   : x86_64.RBX,
    x86asm.AH   : x86_64.RAX,
    x86asm.CH   : x86_64.RCX,
    x86asm.DH   : x86_64.RDX,
    x86asm.BH   : x86_64.RBX,
    x86asm.SPB  : x86_64.RSP,
    x86asm.BPB  : x86_64.RBP,
    x86asm.SIB  : x86_64.RSI,
    x86asm.DIB  : x86_64.RDI,
    x86asm.R8B  : x86_64.R8,
    x86asm.R9B  : x86_64.R9,
    x86asm.R10B : x86_64.R10,
    x86asm.R11B : x86_64.R11,
    x86asm.R12B : x86_64.R12,
    x86asm.R13B : x86_64.R13,
    x86asm.R14B : x86_64.R14,
    x86asm.R15B : x86_64.R15,
    x86asm.AX   : x86_64.RAX,
    x86asm.CX   : x86_64.RCX,
    x86asm.DX   : x86_64.RDX,
    x86asm.BX   : x86_64.RBX,
    x86asm.SP   : x86_64.RSP,
    x86asm.BP   : x86_64.RBP,
    x86asm.SI   : x86_64.RSI,
    x86asm.DI   : x86_64.RDI,
    x86asm.R8W  : x86_64.R8,
    x86asm.R9W  : x86_64.R9,
    x86asm.R10W : x86_64.R10,
    x86asm.R11W : x86_64.R11,
    x86asm.R12W : x86_64.R12,
    x86asm.R13W : x86_64.R13,
    x86asm.R14W : x86_64.R14,
    x86asm.R15W : x86_64.R15,
    x86asm.EAX  : x86_64.RAX,
    x86asm.ECX  : x86_64.RCX,
    x86asm.EDX  : x86_64.RDX,
    x86asm.EBX  : x86_64.RBX,
    x86asm.ESP  : x86_64.RSP,
    x86asm.EBP  : x86_64.RBP,
    x86asm.ESI  : x86_64.RSI,
    x86asm.EDI  : x86_64.RDI,
    x86asm.R8L  : x86_64.R8,
    x86asm.R9L  : x86_64.R9,
    x86asm.R10L : x86_64.R10,
    x86asm.R11L : x86_64.R11,
    x86asm.R12L : x86_64.R12,
    x86asm.R13L : x86_64.R13,
    x86asm.R14L : x86_64.R14,
    x86asm.R15L : x86_64.R15,
    x86asm.RAX  : x86_64.RAX,
    x86asm.RCX  : x86_64.RCX,
    x86asm.RDX  : x86_64.RDX,
    x86asm.RBX  : x86_64.RBX,
    x86asm.RSP  : x86_64.RSP,
    x86asm.RBP  : x86_64.RBP,
    x86asm.RSI  : x86_64.RSI,
    x86asm.RDI  : x86_64.RDI,
    x86asm.R8   : x86_64.R8,
    x86asm.R9   : x86_64.R9,
    x86asm.R10  : x86_64.R10,
    x86asm.R11  : x86_64.R11,
    x86asm.R12  : x86_64.R12,
    x86asm.R13  : x86_64.R13,
    x86asm.R14  : x86_64.R14,
    x86asm.R15  : x86_64.R15,
}

var freeRegisters = map[x86_64.Register64]bool {
    x86_64.RAX: true,
    x86_64.RSI: true,
    x86_64.RDI: true,
}

type _InstrBlock struct {
    ret    bool
    size   uintptr
    entry  unsafe.Pointer
    links [2]*_InstrBlock
}

func newInstrBlock(entry unsafe.Pointer) *_InstrBlock {
    return &_InstrBlock{entry: entry}
}

func (self *_InstrBlock) pc() unsafe.Pointer {
    return unsafe.Pointer(uintptr(self.entry) + self.size)
}

func (self *_InstrBlock) code() []byte {
    return rt.BytesFrom(self.pc(), 15, 15)
}

func (self *_InstrBlock) commit(size int) {
    self.size += uintptr(size)
}

func resolveClobberSet(fn interface{}) map[x86_64.Register64]bool {
    buf := lane.NewQueue()
    ret := make(map[x86_64.Register64]bool)
    bmp := make(map[unsafe.Pointer]*_InstrBlock)

    /* build the CFG with BFS */
    for buf.Enqueue(newInstrBlock(rt.FuncAddr(fn))); !buf.Empty(); {
        val := buf.Dequeue()
        cfg := val.(*_InstrBlock)

        /* parse every instruction in the block */
        for !cfg.ret {
            var err error
            var ins x86asm.Inst

            /* decode one instruction */
            if ins, err = x86asm.Decode(cfg.code(), 64); err != nil {
                panic(err)
            } else {
                cfg.commit(ins.Len)
            }

            /* calling to other functions, cannot analyze */
            if ins.Op == x86asm.CALL {
                return nil
            }

            /* simple algorithm: every write to register is treated as clobbering */
            if ins.Op == x86asm.MOV {
                if reg, ok := ins.Args[0].(x86asm.Reg); ok {
                    if rr, rok := registerTable[reg]; rok && !freeRegisters[rr] {
                        ret[rr] = true
                    }
                }
            }

            /* check for returns */
            if ins.Op == x86asm.RET {
                cfg.ret = true
                break
            }

            /* check for branches */
            if !branchTable[ins.Op] {
                continue
            }

            /* calculate branch address */
            links := [2]unsafe.Pointer {
                cfg.pc(),
                unsafe.Pointer(uintptr(cfg.pc()) + uintptr(ins.Args[0].(x86asm.Rel))),
            }

            /* link the next blocks */
            for i := 0; i < 2; i++ {
                if cfg.links[i] = bmp[links[i]]; cfg.links[i] == nil {
                    cfg.links[i] = newInstrBlock(links[i])
                    bmp[links[i]] = cfg.links[i]
                }
            }

            /* add the branches if not returned, if either one returns, mark the block returned */
            for i := 0; i < 2; i++ {
                if cfg.links[i].ret {
                    cfg.ret = true
                } else {
                    buf.Enqueue(cfg.links[i])
                }
            }
        }
    }

    /* all done */
    return ret
}
