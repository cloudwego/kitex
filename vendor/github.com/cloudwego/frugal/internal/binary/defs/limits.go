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

package defs

import (
    `os`
    `strconv`
)

const (
    _DefaultMaxNesting  = 5         // cutoff at 5 levels of nesting types
    _DefaultMaxILBuffer = 50000     // cutoff at 50k of IL instructions
)

var (
    MaxNesting  = parseOrDefault("FRUGAL_MAX_NESTING", _DefaultMaxNesting, 1)
    MaxILBuffer = parseOrDefault("FRUGAL_MAX_IL_BUFFER", _DefaultMaxILBuffer, 256)
)

func parseOrDefault(key string, def int, min int) int {
    if env := os.Getenv(key); env == "" {
        return def
    } else if val, err := strconv.ParseUint(env, 0, 64); err != nil {
        panic("frugal: invalid value for " + key)
    } else if ret := int(val); ret <= min {
        panic("frugal: value too small for " + key)
    } else {
        return ret
    }
}
