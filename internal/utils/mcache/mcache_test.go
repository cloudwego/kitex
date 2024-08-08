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

package mcache

import "testing"

func TestMalloc(t *testing.T) {
	buf := Malloc(4096)
	t.Log(cap(buf))
}

func BenchmarkNormal4096(b *testing.B) {
	var buf []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = make([]byte, 0, 4096)
	}
	_ = buf
}

func BenchmarkMCache4096(b *testing.B) {
	var buf []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = Malloc(4096)
		Free(buf)
	}
	_ = buf
}

func BenchmarkNormal10M(b *testing.B) {
	var buf []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = make([]byte, 0, 1024*1024*10)
	}
	_ = buf
}

func BenchmarkMCache10M(b *testing.B) {
	var buf []byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = Malloc(1024 * 1024 * 10)
		Free(buf)
	}
	_ = buf
}

func BenchmarkNormal4096Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				buf = make([]byte, 0, 4096)
			}
		}
		_ = buf
	})
}

func BenchmarkMCache4096Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				buf = Malloc(4096)
				Free(buf)
			}
		}
		_ = buf
	})
}

func BenchmarkNormal10MParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				buf = make([]byte, 0, 1024*1024*10)
			}
		}
		_ = buf
	})
}

func BenchmarkMCache10MParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		for pb.Next() {
			for i := 0; i < b.N; i++ {
				buf = Malloc(1024 * 1024 * 10)
				Free(buf)
			}
		}
		_ = buf
	})
}
