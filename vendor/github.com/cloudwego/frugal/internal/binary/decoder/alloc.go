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
    `unsafe`

    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/rt`
)

//go:noescape
//go:linkname makemap runtime.makemap
//goland:noinspection GoUnusedParameter
func makemap(t *rt.GoMapType, hint int, h *rt.GoMap) *rt.GoMap

//go:noescape
//go:linkname mallocgc runtime.mallocgc
//goland:noinspection GoUnusedParameter
func mallocgc(size uintptr, typ *rt.GoType, needzero bool) unsafe.Pointer

var (
    F_makemap  = hir.RegisterGCall(makemap, emu_gcall_makemap)
    F_mallocgc = hir.RegisterGCall(mallocgc, emu_gcall_mallocgc)
)
