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
)

func emu_gcall_newFieldBitmap(ctx hir.CallContext) {
    if !ctx.Verify("", "*") {
        panic("invalid newFieldBitmap call")
    } else {
        ctx.Rp(0, unsafe.Pointer(newFieldBitmap()))
    }
}

func emu_gcall_FieldBitmap_Free(ctx hir.CallContext) {
    if !ctx.Verify("*", "") {
        panic("invalid (*FieldBitmap).Free call")
    } else {
        (*FieldBitmap)(ctx.Ap(0)).Free()
    }
}
