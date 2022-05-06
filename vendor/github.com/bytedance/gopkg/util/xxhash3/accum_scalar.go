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

package xxhash3

import (
	"github.com/bytedance/gopkg/internal/runtimex"
	"unsafe"
)

func accumScalar(xacc *[8]uint64, xinput, xsecret unsafe.Pointer, l uintptr) {
	j := uintptr(0)

	// Loops over block and process 16*8*8=1024 bytes of data each iteration
	for ; j < (l-1)/1024; j++ {
		k := xsecret
		for i := 0; i < 16; i++ {
			dataVec0 := runtimex.ReadUnaligned64(xinput)

			keyVec := dataVec0 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*0))
			xacc[1] += dataVec0
			xacc[0] += (keyVec & 0xffffffff) * (keyVec >> 32)

			dataVec1 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*1))
			keyVec1 := dataVec1 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*1))
			xacc[0] += dataVec1
			xacc[1] += (keyVec1 & 0xffffffff) * (keyVec1 >> 32)

			dataVec2 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*2))
			keyVec2 := dataVec2 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*2))
			xacc[3] += dataVec2
			xacc[2] += (keyVec2 & 0xffffffff) * (keyVec2 >> 32)

			dataVec3 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*3))
			keyVec3 := dataVec3 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*3))
			xacc[2] += dataVec3
			xacc[3] += (keyVec3 & 0xffffffff) * (keyVec3 >> 32)

			dataVec4 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*4))
			keyVec4 := dataVec4 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*4))
			xacc[5] += dataVec4
			xacc[4] += (keyVec4 & 0xffffffff) * (keyVec4 >> 32)

			dataVec5 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*5))
			keyVec5 := dataVec5 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*5))
			xacc[4] += dataVec5
			xacc[5] += (keyVec5 & 0xffffffff) * (keyVec5 >> 32)

			dataVec6 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*6))
			keyVec6 := dataVec6 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*6))
			xacc[7] += dataVec6
			xacc[6] += (keyVec6 & 0xffffffff) * (keyVec6 >> 32)

			dataVec7 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*7))
			keyVec7 := dataVec7 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*7))
			xacc[6] += dataVec7
			xacc[7] += (keyVec7 & 0xffffffff) * (keyVec7 >> 32)

			xinput, k = unsafe.Pointer(uintptr(xinput)+_stripe), unsafe.Pointer(uintptr(k)+8)
		}

		// scramble xacc
		xacc[0] ^= xacc[0] >> 47
		xacc[0] ^= xsecret_128
		xacc[0] *= prime32_1

		xacc[1] ^= xacc[1] >> 47
		xacc[1] ^= xsecret_136
		xacc[1] *= prime32_1

		xacc[2] ^= xacc[2] >> 47
		xacc[2] ^= xsecret_144
		xacc[2] *= prime32_1

		xacc[3] ^= xacc[3] >> 47
		xacc[3] ^= xsecret_152
		xacc[3] *= prime32_1

		xacc[4] ^= xacc[4] >> 47
		xacc[4] ^= xsecret_160
		xacc[4] *= prime32_1

		xacc[5] ^= xacc[5] >> 47
		xacc[5] ^= xsecret_168
		xacc[5] *= prime32_1

		xacc[6] ^= xacc[6] >> 47
		xacc[6] ^= xsecret_176
		xacc[6] *= prime32_1

		xacc[7] ^= xacc[7] >> 47
		xacc[7] ^= xsecret_184
		xacc[7] *= prime32_1

	}
	l -= _block * j

	// last partial block (at most 1024 bytes)
	if l > 0 {
		k := xsecret
		i := uintptr(0)
		for ; i < (l-1)/_stripe; i++ {
			dataVec := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*0))
			keyVec := dataVec ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*0))
			xacc[1] += dataVec
			xacc[0] += (keyVec & 0xffffffff) * (keyVec >> 32)

			dataVec1 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*1))
			keyVec1 := dataVec1 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*1))
			xacc[0] += dataVec1
			xacc[1] += (keyVec1 & 0xffffffff) * (keyVec1 >> 32)

			dataVec2 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*2))
			keyVec2 := dataVec2 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*2))
			xacc[3] += dataVec2
			xacc[2] += (keyVec2 & 0xffffffff) * (keyVec2 >> 32)

			dataVec3 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*3))
			keyVec3 := dataVec3 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*3))
			xacc[2] += dataVec3
			xacc[3] += (keyVec3 & 0xffffffff) * (keyVec3 >> 32)

			dataVec4 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*4))
			keyVec4 := dataVec4 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*4))
			xacc[5] += dataVec4
			xacc[4] += (keyVec4 & 0xffffffff) * (keyVec4 >> 32)

			dataVec5 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*5))
			keyVec5 := dataVec5 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*5))
			xacc[4] += dataVec5
			xacc[5] += (keyVec5 & 0xffffffff) * (keyVec5 >> 32)

			dataVec6 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*6))
			keyVec6 := dataVec6 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*6))
			xacc[7] += dataVec6
			xacc[6] += (keyVec6 & 0xffffffff) * (keyVec6 >> 32)

			dataVec7 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*7))
			keyVec7 := dataVec7 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*7))
			xacc[6] += dataVec7
			xacc[7] += (keyVec7 & 0xffffffff) * (keyVec7 >> 32)

			xinput, k = unsafe.Pointer(uintptr(xinput)+_stripe), unsafe.Pointer(uintptr(k)+8)
		}
		l -= _stripe * i

		// last stripe (align to last 64 bytes)
		if l > 0 {
			xinput = unsafe.Pointer(uintptr(xinput) - uintptr(_stripe-l))
			k = unsafe.Pointer(uintptr(xsecret) + 121)

			dataVec := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*0))
			keyVec := dataVec ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*0))
			xacc[1] += dataVec
			xacc[0] += (keyVec & 0xffffffff) * (keyVec >> 32)

			dataVec1 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*1))
			keyVec1 := dataVec1 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*1))
			xacc[0] += dataVec1
			xacc[1] += (keyVec1 & 0xffffffff) * (keyVec1 >> 32)

			dataVec2 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*2))
			keyVec2 := dataVec2 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*2))
			xacc[3] += dataVec2
			xacc[2] += (keyVec2 & 0xffffffff) * (keyVec2 >> 32)

			dataVec3 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*3))
			keyVec3 := dataVec3 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*3))
			xacc[2] += dataVec3
			xacc[3] += (keyVec3 & 0xffffffff) * (keyVec3 >> 32)

			dataVec4 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*4))
			keyVec4 := dataVec4 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*4))
			xacc[5] += dataVec4
			xacc[4] += (keyVec4 & 0xffffffff) * (keyVec4 >> 32)

			dataVec5 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*5))
			keyVec5 := dataVec5 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*5))
			xacc[4] += dataVec5
			xacc[5] += (keyVec5 & 0xffffffff) * (keyVec5 >> 32)

			dataVec6 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*6))
			keyVec6 := dataVec6 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*6))
			xacc[7] += dataVec6
			xacc[6] += (keyVec6 & 0xffffffff) * (keyVec6 >> 32)

			dataVec7 := runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(xinput) + 8*7))
			keyVec7 := dataVec7 ^ runtimex.ReadUnaligned64(unsafe.Pointer(uintptr(k)+8*7))
			xacc[6] += dataVec7
			xacc[7] += (keyVec7 & 0xffffffff) * (keyVec7 >> 32)
		}
	}
}
