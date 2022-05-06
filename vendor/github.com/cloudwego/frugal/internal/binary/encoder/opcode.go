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
    `fmt`
)

type OpCode uint8

const (
    OP_size_check OpCode = iota
    OP_size_const
    OP_size_dyn
    OP_size_map
    OP_size_defer
    OP_byte
    OP_word
    OP_long
    OP_quad
    OP_sint
    OP_length
    OP_memcpy_be
    OP_seek
    OP_deref
    OP_defer
    OP_map_len
    OP_map_key
    OP_map_next
    OP_map_value
    OP_map_begin
    OP_map_if_next
    OP_map_if_empty
    OP_list_decr
    OP_list_begin
    OP_list_if_next
    OP_list_if_empty
    OP_unique
    OP_goto
    OP_if_nil
    OP_if_hasbuf
    OP_make_state
    OP_drop_state
    OP_halt
)

var _OpNames = [256]string {
    OP_size_check    : "size_check",
    OP_size_const    : "size_const",
    OP_size_dyn      : "size_dyn",
    OP_size_map      : "size_map",
    OP_size_defer    : "size_defer",
    OP_byte          : "byte",
    OP_word          : "word",
    OP_long          : "long",
    OP_quad          : "quad",
    OP_sint          : "sint",
    OP_length        : "length",
    OP_memcpy_be     : "memcpy_be",
    OP_seek          : "seek",
    OP_deref         : "deref",
    OP_defer         : "defer",
    OP_map_len       : "map_len",
    OP_map_key       : "map_key",
    OP_map_next      : "map_next",
    OP_map_value     : "map_value",
    OP_map_begin     : "map_begin",
    OP_map_if_next   : "map_if_next",
    OP_map_if_empty  : "map_if_empty",
    OP_list_decr     : "list_decr",
    OP_list_begin    : "list_begin",
    OP_list_if_next  : "list_if_next",
    OP_list_if_empty : "list_if_empty",
    OP_unique        : "unique",
    OP_goto          : "goto",
    OP_if_nil        : "if_nil",
    OP_if_hasbuf     : "if_hasbuf",
    OP_make_state    : "make_state",
    OP_drop_state    : "drop_state",
    OP_halt          : "halt",
}

var _OpBranches = [256]bool {
    OP_map_if_next   : true,
    OP_map_if_empty  : true,
    OP_list_if_next  : true,
    OP_list_if_empty : true,
    OP_goto          : true,
    OP_if_nil        : true,
    OP_if_hasbuf     : true,
}

func (self OpCode) String() string {
    if _OpNames[self] != "" {
        return _OpNames[self]
    } else {
        return fmt.Sprintf("OpCode(%d)", self)
    }
}
