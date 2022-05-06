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
    `unsafe`

    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/atm/pgen`
    `github.com/cloudwego/frugal/internal/loader`
)

type (
	LinkerAMD64 struct{}
)

func init() {
    SetLinker(new(LinkerAMD64))
}

func (LinkerAMD64) Link(p hir.Program) Encoder {
    fn := pgen.CreateCodeGen((Encoder)(nil)).Generate(p, 0)
    fp := loader.Loader(fn.Code).Load("encoder", fn.Frame)
    return *(*Encoder)(unsafe.Pointer(&fp))
}
