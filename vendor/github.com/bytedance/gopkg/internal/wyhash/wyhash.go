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

// Package wyhash implements https://github.com/wangyi-fudan/wyhash
package wyhash

import (
	"math/bits"
	"unsafe"

	"github.com/bytedance/gopkg/internal/hack"
	"github.com/bytedance/gopkg/internal/runtimex"
)

const (
	DefaultSeed = 0xa0761d6478bd642f // s0
	s1          = 0xe7037ed1a0b428db
	s2          = 0x8ebc6af09c88c6e3
	s3          = 0x589965cc75374cc3
	s4          = 0x1d8e4e27c47d124f
)

func _wymix(a, b uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	return hi ^ lo
}

//go:nosplit
func add(p unsafe.Pointer, x uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}

func Sum64(data []byte) uint64 {
	return Sum64WithSeed(data, DefaultSeed)
}

func Sum64String(data string) uint64 {
	return Sum64StringWithSeed(data, DefaultSeed)
}

func Sum64WithSeed(data []byte, seed uint64) uint64 {
	return Sum64StringWithSeed(hack.BytesToString(data), seed)
}

func Sum64StringWithSeed(data string, seed uint64) uint64 {
	var (
		a, b uint64
	)

	length := len(data)
	i := uintptr(len(data))
	paddr := *(*unsafe.Pointer)(unsafe.Pointer(&data))

	if i > 64 {
		var see1 = seed
		for i > 64 {
			seed = _wymix(runtimex.ReadUnaligned64(paddr)^s1, runtimex.ReadUnaligned64(add(paddr, 8))^seed) ^ _wymix(runtimex.ReadUnaligned64(add(paddr, 16))^s2, runtimex.ReadUnaligned64(add(paddr, 24))^seed)
			see1 = _wymix(runtimex.ReadUnaligned64(add(paddr, 32))^s3, runtimex.ReadUnaligned64(add(paddr, 40))^see1) ^ _wymix(runtimex.ReadUnaligned64(add(paddr, 48))^s4, runtimex.ReadUnaligned64(add(paddr, 56))^see1)
			paddr = add(paddr, 64)
			i -= 64
		}
		seed ^= see1
	}

	for i > 16 {
		seed = _wymix(runtimex.ReadUnaligned64(paddr)^s1, runtimex.ReadUnaligned64(add(paddr, 8))^seed)
		paddr = add(paddr, 16)
		i -= 16
	}

	// i <= 16
	switch {
	case i == 0:
		return _wymix(s1, _wymix(s1, seed))
	case i < 4:
		a = uint64(*(*byte)(paddr))<<16 | uint64(*(*byte)(add(paddr, uintptr(i>>1))))<<8 | uint64(*(*byte)(add(paddr, uintptr(i-1))))
		// b = 0
		return _wymix(s1^uint64(length), _wymix(a^s1, seed))
	case i == 4:
		a = runtimex.ReadUnaligned32(paddr)
		// b = 0
		return _wymix(s1^uint64(length), _wymix(a^s1, seed))
	case i < 8:
		a = runtimex.ReadUnaligned32(paddr)
		b = runtimex.ReadUnaligned32(add(paddr, i-4))
		return _wymix(s1^uint64(length), _wymix(a^s1, b^seed))
	case i == 8:
		a = runtimex.ReadUnaligned64(paddr)
		// b = 0
		return _wymix(s1^uint64(length), _wymix(a^s1, seed))
	default: // 8 < i <= 16
		a = runtimex.ReadUnaligned64(paddr)
		b = runtimex.ReadUnaligned64(add(paddr, i-8))
		return _wymix(s1^uint64(length), _wymix(a^s1, b^seed))
	}
}
