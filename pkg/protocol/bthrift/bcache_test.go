/*
 * Copyright 2024 CloudWeGo Authors
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

package bthrift

import (
	"fmt"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestBCache(t *testing.T) {
	bc := newBCache(cachedSpanSize)

	for i := 0; i < 1024; i++ {
		buf := []byte("123")
		test.DeepEqual(t, bc.Copy(buf), buf)

		buf = make([]byte, 32)
		test.DeepEqual(t, len(bc.Copy(buf)), len(buf))
		buf = make([]byte, 63)
		test.DeepEqual(t, len(bc.Copy(buf)), len(buf))
		buf = make([]byte, 64)
		test.DeepEqual(t, len(bc.Copy(buf)), len(buf))
		buf = make([]byte, 1024)
		test.DeepEqual(t, len(bc.Copy(buf)), len(buf))
		buf = make([]byte, 32*1024)
		test.DeepEqual(t, len(bc.Copy(buf)), len(buf))
		buf = make([]byte, 64*1024)
		test.DeepEqual(t, len(bc.Copy(buf)), len(buf))
		buf = make([]byte, 128*1024)
		test.DeepEqual(t, len(bc.Copy(buf)), len(buf))
	}
}

func TestSpanClass(t *testing.T) {
	test.DeepEqual(t, spanClass(0), 0)

	test.DeepEqual(t, spanClass(1), 1)

	test.DeepEqual(t, spanClass(2), 2)
	test.DeepEqual(t, spanClass(3), 2)

	test.DeepEqual(t, spanClass(4), 3)
	test.DeepEqual(t, spanClass(5), 3)
	test.DeepEqual(t, spanClass(6), 3)
	test.DeepEqual(t, spanClass(7), 3)

	test.DeepEqual(t, spanClass(8), 4)
	test.DeepEqual(t, spanClass(9), 4)

	test.DeepEqual(t, spanClass(32), 6)
	test.DeepEqual(t, spanClass(33), 6)
	test.DeepEqual(t, spanClass(63), 6)

	test.DeepEqual(t, spanClass(64), 7)
	test.DeepEqual(t, spanClass(65), 7)
	test.DeepEqual(t, spanClass(127), 7)
	test.DeepEqual(t, spanClass(128), 8)
	test.DeepEqual(t, spanClass(129), 8)
}

var benchStringSizes = []int{
	15, 31, 127, 511,
	1023, 4095, 16383,
}

func BenchmarkBCacheCopy(b *testing.B) {
	for _, sz := range benchStringSizes {
		b.Run(fmt.Sprintf("size=%d", sz), func(b *testing.B) {
			var buffer []byte
			from := make([]byte, sz)
			bc := newBCache(cachedSpanSize)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buffer = bc.Copy(from[:sz])
				buffer[0] = 'a'
				buffer[sz-1] = 'z'
			}
		})
	}
}

func BenchmarkBSpanCopy(b *testing.B) {
	for _, sz := range benchStringSizes {
		b.Run(fmt.Sprintf("size=%d", sz), func(b *testing.B) {
			var buffer []byte
			from := make([]byte, sz)
			bs := newSpan(cachedSpanSize)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buffer = bs.Make(sz)
				copy(buffer, from)
				buffer[0] = 'a'
				buffer[sz-1] = 'z'
			}
		})
	}
}

func BenchmarkMakeAndCopy(b *testing.B) {
	for _, sz := range benchStringSizes {
		b.Run(fmt.Sprintf("size=%d", sz), func(b *testing.B) {
			var buffer []byte
			from := make([]byte, sz)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buffer = make([]byte, sz)
				copy(buffer, from)
				buffer[0] = 'a'
				buffer[sz-1] = 'z'
			}
		})
	}
}
