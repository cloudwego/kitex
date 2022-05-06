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

// ThriftGo is the default naming style of thrifgo.
type ThriftGo struct {
	noInitialisms bool
}

// Name implements NamingStyle.
func (tg *ThriftGo) Name() string {
	return "thriftgo"
}

// Identify implements NamingStyle.
func (tg *ThriftGo) Identify(name string) (string, error) {
	ws := strings.Split(name, "_")
	for i, w := range ws {
		if len(w) == 0 {
			ws[i] = "_"
			continue
		}
		if unicode.IsLower([]rune(w)[0]) {
			w = common.UpperFirstRune(w)
			w = tg.fixInitialism(w)
			ws[i] = w
		}
	}
	s := strings.Join(ws, "")
	return s, nil
}

// UseInitialisms implements NamingStyle.
func (tg *ThriftGo) UseInitialisms(enable bool) {
	tg.noInitialisms = !enable
}

func (tg *ThriftGo) fixInitialism(s string) string {
	if !tg.noInitialisms {
		u := strings.ToUpper(s)
		if common.IsCommonInitialisms[u] {
			return u
		}
	}
	return s
}
