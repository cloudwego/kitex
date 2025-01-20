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

package protoc

import (
	"go/token"
	"strings"
	"unicode"

	"github.com/cloudwego/kitex/tool/internal_pkg/util"
)

func goSanitized(s string) string {
	// replace invalid characters with '_'
	s = strings.Map(func(r rune) rune {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return '_'
		}
		return r
	}, s)

	// avoid invalid identifier
	if !token.IsIdentifier(s) {
		return "_" + s
	}
	return s
}

type pathElements struct {
	module string // module name
	prefix string // prefix of import path for generated codes
}

// getImportPath returns the complete import path for the given go_package defined in pb idl
func (p *pathElements) getImportPath(goPackage string) (path string, ok bool) {
	parts := strings.Split(goPackage, "/")
	if len(parts) == 0 {
		// malformed import path
		return "", false
	}

	if strings.HasPrefix(goPackage, p.prefix) {
		return goPackage, true
	}
	if strings.Contains(parts[0], ".") || (p.module != "" && strings.HasPrefix(goPackage, p.module)) {
		// already a complete import path, but outside the target path
		return "", false
	}
	// incomplete import path

	return util.JoinPath(p.prefix, goPackage), true
}

func (p *pathElements) getImportPathAndNamespace(goPackage string) (path string, ns string, ok bool) {
	parts := strings.Split(goPackage, "/")
	if len(parts) == 0 {
		return "", "", false
	}
	// todo: consider complete path
	if strings.HasPrefix(goPackage, p.prefix) {
		return goPackage, strings.TrimPrefix(goPackage, p.prefix), true
	}
	return util.CombineOutputPath(p.prefix, goPackage), goPackage, true
}
