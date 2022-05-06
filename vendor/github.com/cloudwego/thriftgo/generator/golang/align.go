// Copyright 2021 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package golang

import (
	"sort"
	"unsafe"

	"github.com/cloudwego/thriftgo/parser"
)

// assuming the generated codes run on the same architecture as thriftgo.
const pointerSize = int(unsafe.Sizeof((*int)(nil)))

var sizeof = (func() map[parser.Category]int {
	return map[parser.Category]int{
		parser.Category_Bool:   1,
		parser.Category_Byte:   1,
		parser.Category_I16:    2,
		parser.Category_I32:    4,
		parser.Category_I64:    8,
		parser.Category_Double: 8, // float64
		parser.Category_String: pointerSize * 2,
		parser.Category_Binary: pointerSize * 3,
		parser.Category_Set:    pointerSize * 3,
		parser.Category_List:   pointerSize * 3,
		parser.Category_Map:    pointerSize,
		parser.Category_Struct: pointerSize, // as pointer
		parser.Category_Enum:   8,           // as int64
	}
})()

// align implements the structure padding algorithm of golang.
type align struct {
	unit int
	size int
}

func (a *align) add(size int) {
	unit := size
	if unit >= pointerSize {
		unit = pointerSize
	}
	if unit > a.unit {
		a.unit = unit
	}

	if pad := a.padded(); pad-a.size < size {
		a.size = pad
	}
	a.size += size
}

func (a *align) padded() int {
	return (a.size + a.unit - 1) / a.unit * a.unit
}

type sizeDiff struct {
	original int
	arranged int
}

func (d *sizeDiff) percent() float64 {
	return float64(d.arranged-d.original) / float64(d.original) * 100
}

func reorderFields(s *parser.StructLike) *sizeDiff {
	if len(s.Fields) == 0 {
		return nil
	}

	fs := make([]*parser.Field, len(s.Fields))
	copy(fs, s.Fields)

	var diff sizeDiff
	var a1, a2 align
	sizes := make(map[*parser.Field]int, len(fs))
	for _, f := range fs {
		if NeedRedirect(f) {
			sizes[f] = pointerSize
		} else {
			sizes[f] = sizeof[f.Type.Category]
		}
		a1.add(sizes[f])
	}

	sort.Slice(fs, func(i, j int) bool {
		return sizes[fs[i]] >= sizes[fs[j]]
	})

	for _, f := range fs {
		a2.add(sizes[f])
	}

	diff.original = a1.padded()
	diff.arranged = a2.padded()
	if diff.original != diff.arranged {
		s.Fields = fs
	}
	return &diff
}
