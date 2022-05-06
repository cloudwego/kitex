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

package styles

import (
	"strings"
	"unicode"

	"github.com/cloudwego/thriftgo/generator/golang/common"
)

// GoLint implements a naming conversion algorithm that similar to https://github.com/golang/lint.
type GoLint struct {
	nolint bool
}

// Name implements NamingStyle.
func (g *GoLint) Name() string {
	return "golint"
}

// Identify implements NamingStyle.
func (g *GoLint) Identify(name string) (string, error) {
	return g.convertName(name), nil
}

// UseInitialisms implements NamingStyle.
func (g *GoLint) UseInitialisms(enable bool) {
	g.nolint = !enable
}

// convertName convert name to thrift name, but will not add underline.
func (g *GoLint) convertName(name string) string {
	if name == "" {
		return ""
	}
	var result string
	words := strings.Split(name, "_")
	for i, str := range words {
		switch {
		case str == "":
			words[i] = "_"
		case str[0] >= 'A' && str[0] <= 'Z' && i != 0:
			words[i] = "_" + str
		case str[0] >= '0' && str[0] <= '9' && i != 0:
			words[i] = "_" + str
		default:
			words[i] = common.UpperFirstRune(str)
		}
	}
	result = strings.Join(words, "")
	return g.lintName(result)
}

func (g *GoLint) lintName(name string) string {
	if g.nolint || name == "_" {
		return name
	}

	type Kind int
	const (
		undefined Kind = iota
		underscore
		lower
		other
	)

	kindOf := func(r rune) Kind {
		if r == '_' {
			return underscore
		}
		if unicode.IsLower(r) {
			return lower
		}
		return other
	}

	var prev, next Kind
	var runes []rune
	var words []string

	testInitialisms := func(rs []rune) string {
		w := string(rs)
		if u := strings.ToUpper(w); common.IsCommonInitialisms[u] {
			// If the identifier starts with a lower case rune, we should keep consistent case.
			if len(words) == 0 && unicode.IsLower(rs[0]) {
				return strings.ToLower(w)
			}
			return u
		}
		if len(words) > 0 && strings.ToLower(w) == w {
			rs[0] = unicode.ToUpper(rs[0])
			w = string(rs)
		}
		return w
	}

	var digitalEnd bool
	newWord := func() {
		words = append(words, testInitialisms(runes))
		digitalEnd = unicode.IsDigit(runes[len(runes)-1])
		runes = runes[:0]
	}

	for _, r := range name {
		next = kindOf(r)
		switch prev {
		case underscore:
			if next == underscore {
				// Any run of underscores turn into only one
				continue
			}
			if len(words) == 0 || (digitalEnd && unicode.IsDigit(r)) {
				// Only leading underscores or underscores between digitals will be kept
				newWord()
			} else {
				runes = runes[:0]
			}
		case lower:
			if next == underscore || next == other {
				newWord()
			}
		case other:
			if next == underscore {
				newWord()
			}
		}
		runes = append(runes, r)
		prev = next
	}
	if w := testInitialisms(runes); w != "_" && w != "" {
		words = append(words, w)
	}

	return strings.Join(words, "")
}
