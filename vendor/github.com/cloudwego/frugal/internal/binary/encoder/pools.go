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
    `sync`

    `github.com/cloudwego/frugal/internal/rt`
)

var (
    programPool        sync.Pool
    compilerPool       sync.Pool
    basicBlockPool     sync.Pool
    graphBuilderPool   sync.Pool
    runtimeStatePool   sync.Pool
    optimizerStatePool sync.Pool
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
    if v := compilerPool.Get(); v != nil {
        return v.(Compiler)
    } else {
        return make(Compiler)
    }
}

func freeCompiler(p Compiler) {
    compilerPool.Put(p)
}

func resetCompiler(p Compiler) Compiler {
    rt.MapClear(p)
    return p
}

func newBasicBlock() *BasicBlock {
    if v := basicBlockPool.Get(); v != nil {
        return v.(*BasicBlock)
    } else {
        return new(BasicBlock)
    }
}

func freeBasicBlock(p *BasicBlock) {
    basicBlockPool.Put(p)
}

func newGraphBuilder() *GraphBuilder {
    if v := graphBuilderPool.Get(); v == nil {
        return allocGraphBuilder()
    } else {
        return resetGraphBuilder(v.(*GraphBuilder))
    }
}

func freeGraphBuilder(p *GraphBuilder) {
    graphBuilderPool.Put(p)
}

func allocGraphBuilder() *GraphBuilder {
    return &GraphBuilder {
        Pin   : make(map[int]bool),
        Graph : make(map[int]*BasicBlock),
    }
}

func resetGraphBuilder(p *GraphBuilder) *GraphBuilder {
    rt.MapClear(p.Pin)
    rt.MapClear(p.Graph)
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

func newOptimizerState() *_OptimizerState {
    if v := optimizerStatePool.Get(); v == nil {
        return allocOptimizerState()
    } else {
        return resetOptimizerState(v.(*_OptimizerState))
    }
}

func freeOptimizerState(p *_OptimizerState) {
    optimizerStatePool.Put(p)
}

func allocOptimizerState() *_OptimizerState {
    return &_OptimizerState {
        buf  : make([]*BasicBlock, 0, 16),
        refs : make(map[int]int),
        mask : make(map[*BasicBlock]bool),
    }
}

func resetOptimizerState(p *_OptimizerState) *_OptimizerState {
    p.buf = p.buf[:0]
    rt.MapClear(p.refs)
    rt.MapClear(p.mask)
    return p
}
