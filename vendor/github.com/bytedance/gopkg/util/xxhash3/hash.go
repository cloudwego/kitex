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

// Package xxhash3 implements https://github.com/Cyan4973/xxHash/blob/dev/xxhash.h
package xxhash3

import (
	"math/bits"
	"unsafe"

	"github.com/bytedance/gopkg/internal/hack"
	"github.com/bytedance/gopkg/internal/runtimex"
)

// Hash returns the hash value of the byte slice in 64bits.
func Hash(data []byte) uint64 {
	funcPointer := hashSmall

	if len(data) > 16 {
		funcPointer = hashLarge
	}
	return hashfunc[funcPointer](*(*unsafe.Pointer)(unsafe.Pointer(&data)), len(data))

}

// HashString returns the hash value of the string in 64bits.
func HashString(s string) uint64 {
	return Hash(hack.StringToBytes(s))
}

func xxh3HashSmall(xinput unsafe.Pointer, length int) uint64 {

	if length > 8 {
		inputlo := runtimex.ReadUnaligned64(xinput) ^ xsecret_024 ^ xsecret_032
		inputhi := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-8))) ^ xsecret_040 ^ xsecret_048
		return xxh3Avalanche(uint64(length) + bits.ReverseBytes64(inputlo) + inputhi + mix(inputlo, inputhi))
	} else if length >= 4 {
		input1 := runtimex.ReadUnaligned32(xinput)
		input2 := runtimex.ReadUnaligned32(unsafe.Pointer(uintptr(xinput) + uintptr(length-4)))
		input64 := input2 + input1<<32
		keyed := input64 ^ xsecret_008 ^ xsecret_016
		return xxh3RRMXMX(keyed, uint64(length))
	} else if length == 3 {
		c12 := runtimex.ReadUnaligned16(xinput)
		c3 := uint64(*(*uint8)(unsafe.Pointer(uintptr(xinput) + 2)))
		acc := c12<<16 + c3 + 3<<8
		acc ^= uint64(xsecret32_000 ^ xsecret32_004)
		return xxh64Avalanche(acc)
	} else if length == 2 {
		c12 := runtimex.ReadUnaligned16(xinput)
		acc := c12*(1<<24+1)>>8 + 2<<8
		acc ^= uint64(xsecret32_000 ^ xsecret32_004)
		return xxh64Avalanche(acc)
	} else if length == 1 {
		c1 := uint64(*(*uint8)(xinput))
		acc := c1*(1<<24+1<<16+1) + 1<<8
		acc ^= uint64(xsecret32_000 ^ xsecret32_004)
		return xxh64Avalanche(acc)
	}
	return 0x2d06800538d394c2
}

func xxh3HashLarge(xinput unsafe.Pointer, l int) (acc uint64) {
	length := uintptr(l)

	if length <= 128 {
		acc := uint64(length * prime64_1)
		if length > 32 {
			if length > 64 {
				if length > 96 {
					acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+48))^xsecret_096, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+56))^xsecret_104)
					acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-64)))^xsecret_112, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-56)))^xsecret_120)
				}
				acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+32))^xsecret_064, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+40))^xsecret_072)
				acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-48)))^xsecret_080, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-40)))^xsecret_088)
			}
			acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16))^xsecret_032, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+24))^xsecret_040)
			acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-32)))^xsecret_048, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-24)))^xsecret_056)
		}
		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+0))^xsecret_000, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+8))^xsecret_008)
		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-16)))^xsecret_016, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-8)))^xsecret_024)

		return xxh3Avalanche(acc)

	} else if length <= 240 {
		acc := uint64(length * prime64_1)

		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*0))^xsecret_000, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*0+8))^xsecret_008)
		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*1))^xsecret_016, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*1+8))^xsecret_024)
		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*2))^xsecret_032, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*2+8))^xsecret_040)
		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*3))^xsecret_048, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*3+8))^xsecret_056)
		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*4))^xsecret_064, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*4+8))^xsecret_072)
		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*5))^xsecret_080, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*5+8))^xsecret_088)
		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*6))^xsecret_096, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*6+8))^xsecret_104)
		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*7))^xsecret_112, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+16*7+8))^xsecret_120)

		acc = xxh3Avalanche(acc)
		nbRounds := uint64(length >> 4 << 4)

		for i := uint64(8 * 16); i < nbRounds; i += 16 {
			acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(i)))^runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xsecret)+uintptr(i-125))), runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(i+8)))^runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xsecret)+uintptr(i-117))))
		}

		acc += mix(runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-16)))^xsecret_119, runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput)+uintptr(length-8)))^xsecret_127)

		return xxh3Avalanche(acc)
	}

	xacc = [8]uint64{
		prime32_3, prime64_1, prime64_2, prime64_3,
		prime64_4, prime32_2, prime64_5, prime32_1}

	acc = uint64(length * prime64_1)

	if avx2 {
		accumAVX2(&xacc, xinput, xsecret, length)
	} else if sse2 {
		accumSSE2(&xacc, xinput, xsecret, length)
	} else {
		accumScalar(&xacc, xinput, xsecret, length)
	}
	//merge xacc
	acc += mix(xacc[0]^xsecret_011, xacc[1]^xsecret_019)
	acc += mix(xacc[2]^xsecret_027, xacc[3]^xsecret_035)
	acc += mix(xacc[4]^xsecret_043, xacc[5]^xsecret_051)
	acc += mix(xacc[6]^xsecret_059, xacc[7]^xsecret_067)

	return xxh3Avalanche(acc)
}
