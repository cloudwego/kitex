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

package rt

import (
    `unsafe`
)

//go:noescape
//go:linkname mapclear runtime.mapclear
//goland:noinspection GoUnusedParameter
func mapclear(t *GoType, h unsafe.Pointer)

//go:noescape
//go:linkname growslice runtime.growslice
//goland:noinspection GoUnusedParameter
func growslice(et *GoType, old GoSlice, cap int) GoSlice

//go:noescape
//go:linkname mapiternext runtime.mapiternext
//goland:noinspection GoUnusedParameter
func mapiternext(it *GoMapIterator)

//go:noescape
//go:linkname mapiterinit runtime.mapiterinit
//goland:noinspection GoUnusedParameter
func mapiterinit(t *GoType, h unsafe.Pointer, it *GoMapIterator)

//go:nosplit
func MapIter(m interface{}) *GoMapIterator {
    v := UnpackEface(m)
    r := new(GoMapIterator)
    mapiterinit(v.Type, v.Value, r)
    return r
}

//go:nosplit
func MapClear(m interface{}) {
    v := UnpackEface(m)
    mapclear(v.Type, v.Value)
}

//go:nosplit
func GrowSlice(s interface{}, cap int) {
    v := UnpackEface(s)
    *(*GoSlice)(v.Value) = growslice(SliceElem(PtrElem(v.Type)), *(*GoSlice)(v.Value), cap)
}
