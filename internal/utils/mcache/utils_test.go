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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBsr(t *testing.T) {
	assert.Equal(t, bsr(4), 2)
	assert.Equal(t, bsr(24), 4)
	assert.Equal(t, bsr((1<<10)-1), 9)
	assert.Equal(t, bsr((1<<30)+(1<<19)+(1<<16)+(1<<1)), 30)
}

func BenchmarkBsr(b *testing.B) {
	num := (1 << 30) + (1 << 19) + (1 << 16) + (1 << 1)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bsr(num + i)
	}
}
