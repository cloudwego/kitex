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

package skipmap

import (
	_ "unsafe" // for linkname

	"github.com/bytedance/gopkg/internal/wyhash"
	"github.com/bytedance/gopkg/lang/fastrand"
)

const (
	maxLevel            = 16
	p                   = 0.25
	defaultHighestLevel = 3
)

func hash(s string) uint64 {
	return wyhash.Sum64String(s)
}

//go:linkname cmpstring runtime.cmpstring
func cmpstring(a, b string) int

func randomLevel() int {
	level := 1
	for fastrand.Uint32n(1/p) == 0 {
		level++
	}
	if level > maxLevel {
		return maxLevel
	}
	return level
}
