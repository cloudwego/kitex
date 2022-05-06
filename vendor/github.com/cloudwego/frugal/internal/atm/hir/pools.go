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

package hir

import (
    `sync`

    `github.com/cloudwego/frugal/internal/rt`
)

var (
    instrPool   sync.Pool
    builderPool sync.Pool
)

func newInstr(op OpCode) *Ir {
    if v := instrPool.Get(); v == nil {
        return allocInstr(op)
    } else {
        return resetInstr(op, v.(*Ir))
    }
}

func freeInstr(p *Ir) {
    instrPool.Put(p)
}

func allocInstr(op OpCode) (p *Ir) {
    p = new(Ir)
    p.Op = op
    return
}

func resetInstr(op OpCode, p *Ir) *Ir {
    *p = Ir{Op: op}
    return p
}

func newBuilder() *Builder {
    if v := builderPool.Get(); v == nil {
        return allocBuilder()
    } else {
        return resetBuilder(v.(*Builder))
    }
}

func freeBuilder(p *Builder) {
    builderPool.Put(p)
}

func allocBuilder() (p *Builder) {
    p       = new(Builder)
    p.refs  = make(map[string]*Ir, 64)
    p.pends = make(map[string][]**Ir, 64)
    return
}

func resetBuilder(p *Builder) *Builder {
    p.i    = 0
    p.head = nil
    p.tail = nil
    rt.MapClear(p.refs)
    rt.MapClear(p.pends)
    return p
}
