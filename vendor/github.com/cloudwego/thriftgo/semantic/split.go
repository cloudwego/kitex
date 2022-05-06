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

package semantic

import (
	"path/filepath"
	"strings"
)

// SplitType tries to split the given ID into two part: the name of an included
// IDL without file extension and the type name in that IDL. If ID is a type name
// containing no dot, then the it is returned unchanged.
func SplitType(id string) []string {
	if id == "" {
		return []string{}
	}
	idx := strings.LastIndex(id, ".")
	if idx == -1 {
		return []string{id}
	}
	return []string{id[:idx], id[idx+1:]}
}

// SplitValue tries to find any possible resolution of the given ID assuming it
// is a name of a constant value or an enum value.
// When ID contains no dot, it is returned unchanged. Otherwise, this function
// tries to explain it as 'IDL.Value' or as 'IDL.EnumType.Value' and to return
// all possible divisions.
func SplitValue(id string) (sss [][]string) {
	if id == "" {
		return
	}
	idx := strings.LastIndex(id, ".")
	if idx == -1 {
		sss = append(sss, []string{id})
		return
	}

	i, v := id[:idx], id[idx+1:]
	sss = append(sss, []string{i, v})

	if idx = strings.LastIndex(i, "."); idx != -1 {
		i, e := i[:idx], i[idx+1:]
		sss = append(sss, []string{i, e, v})
	}
	return
}

// IDLPrefix returns the file name without extension.
func IDLPrefix(filename string) string {
	ref := filepath.Base(filename)
	return strings.TrimSuffix(ref, filepath.Ext(ref))
}
