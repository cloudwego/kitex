// Copyright 2021 ByteDance Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build 386 || amd64 || arm || arm64 || ppc64le || mips64le || mipsle || riscv64 || wasm
// +build 386 amd64 arm arm64 ppc64le mips64le mipsle riscv64 wasm

//
// from golang-go/src/os/endian_big.go

package runtimex

import (
	"unsafe"
)

func ReadUnaligned64(p unsafe.Pointer) uint64 {
	// Equal to runtime.readUnaligned64, but this function can be inlined
	// compared to  use runtime.readUnaligned64 via go:linkname.
	q := (*[8]byte)(p)
	return uint64(q[0]) | uint64(q[1])<<8 | uint64(q[2])<<16 | uint64(q[3])<<24 | uint64(q[4])<<32 | uint64(q[5])<<40 | uint64(q[6])<<48 | uint64(q[7])<<56
}

func ReadUnaligned32(p unsafe.Pointer) uint64 {
	q := (*[4]byte)(p)
	return uint64(uint32(q[0]) | uint32(q[1])<<8 | uint32(q[2])<<16 | uint32(q[3])<<24)
}

func ReadUnaligned16(p unsafe.Pointer) uint64 {
	q := (*[2]byte)(p)
	return uint64(uint32(q[0]) | uint32(q[1])<<8)
}
