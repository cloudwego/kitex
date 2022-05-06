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

// Apache ports functions adapted from the apache thrift go generator.
// See https://git-wip-us.apache.org/repos/asf?p=thrift.git;a=blob;f=compiler/cpp/src/thrift/generate/t_go_generator.cc
type Apache struct {
	ignoreInitialisms bool
}

// Name implements NamingStyle.
func (a *Apache) Name() string {
	return "apache"
}

// Identify implements NamingStyle.
func (a *Apache) Identify(name string) (string, error) {
	return a.publicize(name, "", false), nil
}

// UseInitialisms implements NamingStyle.
func (a *Apache) UseInitialisms(enable bool) {
	a.ignoreInitialisms = !enable
}

// publicize implements the publicize function of the apache thrift go generator.
func (a *Apache) publicize(name, service string, isArgsOrResult bool) string { // nolint
	prefix, value := "", name
	if idx := strings.LastIndexByte(name, '.'); idx != -1 {
		prefix, value = name[:idx], name[idx+1:]
	}
	value = common.UpperFirstRune(value)
	value = a.camelcase(value)

	// if strings.HasPrefix(value, "New") {
	//     value = value + "_"
	// }

	if isArgsOrResult {
		prefix += a.publicize(service, "", false)
	} else {
		// IDL identifiers may end with "Args"/"Result" which interferes with the
		// implicit service function structs.
		// Adding another extra underscore to all those identifiers solves this.
		// Suppress this check for the actual helper struct names.
		if strings.HasSuffix(value, "Args") || strings.HasSuffix(value, "Result") {
			value += "_"
		}
	}

	return prefix + value
}

func (a *Apache) camelcase(name string) string {
	// Fix common initialism in first word
	words := strings.Split(name, "_")

	var ws []string
	for i, w := range words {
		if len(w) == 0 {
			ws = append(ws, "_")
			continue
		}
		if i == 0 {
			w = a.fixCommonInitialism(w)
			ws = append(ws, w)
			continue
		}
		if unicode.IsLower([]rune(w)[0]) {
			w = common.UpperFirstRune(w)
			w = a.fixCommonInitialism(w)
			ws = append(ws, w)
		} else {
			ws = append(ws, "_", w)
		}
	}

	return strings.Join(ws, "")
}

func (a *Apache) fixCommonInitialism(s string) string {
	if !a.ignoreInitialisms {
		u := strings.ToUpper(s)
		if common.IsCommonInitialisms[u] {
			return u
		}
	}
	return s
}
