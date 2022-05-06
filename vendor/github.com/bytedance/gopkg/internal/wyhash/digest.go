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

package wyhash

import (
	"reflect"
	"unsafe"

	"github.com/bytedance/gopkg/internal/runtimex"
)

type Digest struct {
	initseed uint64
	seed     uint64
	see1     uint64
	data     [64]byte
	length   int
	total    int
}

// New creates a new Digest that computes the 64-bit wyhash algorithm.
func New(seed uint64) *Digest {
	return &Digest{
		seed:     seed,
		initseed: seed,
		see1:     seed,
	}
}

func NewDefault() *Digest {
	return New(DefaultSeed)
}

// Size always returns 8 bytes.
func (d *Digest) Size() int { return 8 }

// BlockSize always returns 64 bytes.
func (d *Digest) BlockSize() int { return 64 }

// Reset the digest, and the seed will be reset to initseed.
// The initseed is the seed when digest has been created.
func (d *Digest) Reset() {
	d.seed = d.initseed
	d.see1 = d.initseed
	d.total = 0
	d.length = 0
}

func (d *Digest) SetSeed(seed uint64)     { d.seed = seed }
func (d *Digest) Seed() uint64            { return d.seed }
func (d *Digest) SetInitSeed(seed uint64) { d.initseed = seed }
func (d *Digest) InitSeed() uint64        { return d.initseed }

// Write (via the embedded io.Writer interface) adds more data to the running hash.
// It never returns an error.
func (d *Digest) Write(input []byte) (int, error) {
	ilen := len(input)
	d.total += ilen

	if d.length+ilen <= 64 {
		copy(d.data[d.length:d.length+ilen], input)
		d.length += ilen
		return ilen, nil
	}

	// Consume first 64 bytes if possible.
	if d.length != 0 {
		inputpre := 64 - d.length // the data from input to d.data
		copy(d.data[d.length:], input[:inputpre])
		input = input[inputpre:] // free preceding data

		paddr := unsafe.Pointer(&d.data)
		d.seed = _wymix(runtimex.ReadUnaligned64(paddr)^s1, runtimex.ReadUnaligned64(add(paddr, 8))^d.seed) ^ _wymix(runtimex.ReadUnaligned64(add(paddr, 16))^s2, runtimex.ReadUnaligned64(add(paddr, 24))^d.seed)
		d.see1 = _wymix(runtimex.ReadUnaligned64(add(paddr, 32))^s3, runtimex.ReadUnaligned64(add(paddr, 40))^d.see1) ^ _wymix(runtimex.ReadUnaligned64(add(paddr, 48))^s4, runtimex.ReadUnaligned64(add(paddr, 56))^d.see1)

		d.length = 0 // free d.data, since it has been consumed
	}

	// If remain input still greater than 64.
	seed, see1 := d.seed, d.see1
	for len(input) > 64 {
		paddr := *(*unsafe.Pointer)(unsafe.Pointer(&input))
		seed = _wymix(runtimex.ReadUnaligned64(paddr)^s1, runtimex.ReadUnaligned64(add(paddr, 8))^seed) ^ _wymix(runtimex.ReadUnaligned64(add(paddr, 16))^s2, runtimex.ReadUnaligned64(add(paddr, 24))^seed)
		see1 = _wymix(runtimex.ReadUnaligned64(add(paddr, 32))^s3, runtimex.ReadUnaligned64(add(paddr, 40))^see1) ^ _wymix(runtimex.ReadUnaligned64(add(paddr, 48))^s4, runtimex.ReadUnaligned64(add(paddr, 56))^see1)
		input = input[64:]
	}
	d.seed, d.see1 = seed, see1

	// Store remain data(< 64 bytes), d.length == 0 for now.
	if len(input) > 0 {
		copy(d.data[:len(input)], input)
		d.length = len(input)
	}

	return ilen, nil
}

// Sum64 returns the current hash.
func (d *Digest) Sum64() uint64 {
	if d.total <= 64 {
		return Sum64WithSeed(byteToSlice(d.data, d.length), d.seed)
	}

	var (
		i      = uintptr(d.length)
		paddr  = unsafe.Pointer(&d.data)
		seed   = d.seed ^ d.see1
		length = d.total
		a, b   uint64
	)

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

// Sum appends the current hash to b and returns the resulting slice.
func (d *Digest) Sum(b []byte) []byte {
	s := d.Sum64()
	return append(
		b,
		byte(s>>56),
		byte(s>>48),
		byte(s>>40),
		byte(s>>32),
		byte(s>>24),
		byte(s>>16),
		byte(s>>8),
		byte(s),
	)
}

func byteToSlice(b [64]byte, length int) []byte {
	var res []byte
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&res))
	bh.Data = uintptr(unsafe.Pointer(&b))
	bh.Len = length
	bh.Cap = length
	return res
}
