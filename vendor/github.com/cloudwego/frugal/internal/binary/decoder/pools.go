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

package decoder

import (
    `sync`

    `github.com/cloudwego/frugal/internal/rt`
)

var (
    programPool      sync.Pool
    compilerPool     sync.Pool
    runtimeStatePool sync.Pool
)

func newProgram() Program {
    if v := programPool.Get(); v != nil {
        return v.(Program)[:0]
    } else {
        return make(Program, 0, 16)
    }
}

func freeProgram(p Program) {
    programPool.Put(p)
}

func newCompiler() Compiler {
    if v := compilerPool.Get(); v == nil {
        return make(Compiler)
    } else {
        return resetCompiler(v.(Compiler))
    }
}

func freeCompiler(p Compiler) {
    compilerPool.Put(p)
}

func resetCompiler(p Compiler) Compiler {
    rt.MapClear(p)
    return p
}

func newRuntimeState() *RuntimeState {
    if v := runtimeStatePool.Get(); v != nil {
        return v.(*RuntimeState)
    } else {
        return new(RuntimeState)
    }
}

func freeRuntimeState(p *RuntimeState) {
    runtimeStatePool.Put(p)
}
