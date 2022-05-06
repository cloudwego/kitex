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

package parser

import "strings"

func searchCircle(cur *Thrift, nodes []string) string {
	if cur == nil {
		return ""
	}
	for _, node := range nodes {
		if node == cur.Filename {
			return strings.Join(append(nodes, node), " -> ")
		}
	}
	nodes = append(nodes, cur.Filename)
	for _, inc := range cur.Includes {
		if path := searchCircle(inc.Reference, nodes); path != "" {
			return path
		}
	}
	return ""
}

// CircleDetect detects whether there is an include circle and return a string
// representing the loop. When no include circle is found, it returns an empty string.
func CircleDetect(ast *Thrift) string {
	return searchCircle(ast, []string{})
}
