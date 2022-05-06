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
    `fmt`
    `math/bits`

    `github.com/cloudwego/frugal/internal/atm/hir`
    `github.com/cloudwego/frugal/internal/rt`
)

//go:nosplit
func error_eof(n int) error {
    return fmt.Errorf("frugal: unexpected EOF: %d bytes short", n)
}

//go:nosplit
func error_skip(e int) error {
    switch e {
        case ETAG   : return fmt.Errorf("frugal: error when skipping fields: -1 (invalid tag)")
        case EEOF   : return fmt.Errorf("frugal: error when skipping fields: -2 (unexpected EOF)")
        case ESTACK : return fmt.Errorf("frugal: error when skipping fields: -3 (value nesting too deep)")
        default     : return fmt.Errorf("frugal: error when skipping fields: %d (unknown error)", e)
    }
}

//go:nosplit
func error_type(e uint8, t uint8) error {
    return fmt.Errorf("frugal: type mismatch: %d expected, got %d", e, t)
}

//go:nosplit
func error_missing(t *rt.GoType, i int, m uint64) error {
    return fmt.Errorf("frugal: missing required field %d for type %s", i * 64 + bits.TrailingZeros64(m), t)
}

var (
    F_error_eof     = hir.RegisterGCall(error_eof, emu_gcall_error_eof)
    F_error_skip    = hir.RegisterGCall(error_skip, emu_gcall_error_skip)
    F_error_type    = hir.RegisterGCall(error_type, emu_gcall_error_type)
    F_error_missing = hir.RegisterGCall(error_missing, emu_gcall_error_missing)
)
