/*
 * Copyright 2021 CloudWeGo Authors
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

package generic

import (
	"fmt"
	"testing"

	"github.com/jackedelic/kitex/internal/test"
)

func TestAbsPath(t *testing.T) {
	t.Run("relative include path", func(t *testing.T) {
		path := "/a/b/c.thrift"
		includePath := "../d.thrift"
		result := "/a/d.thrift"
		p := absPath(path, includePath)
		test.Assert(t, result == p)
	})
	t.Run("abs include path", func(t *testing.T) {
		path := "/a/b/c.thrift"
		includePath := "/a/d.thrift"
		result := "/a/d.thrift"
		p := absPath(path, includePath)
		test.Assert(t, result == p)
	})
}

func TestThriftContentWithAbsIncludePathProvider(t *testing.T) {
	path := "a/b/main.thrift"
	content := `
	namespace go kitex.test.server
	include "x.thrift"
	include "../y.thrift" 

	service InboxService {}
	`
	includes := map[string]string{
		path:           content,
		"a/b/x.thrift": "namespace go kitex.test.server",
		"a/y.thrift": `
		namespace go kitex.test.server
		include "z.thrift"
		`,
		"a/z.thrift": "namespace go kitex.test.server",
	}
	p, err := NewThriftContentWithAbsIncludePathProvider(path, includes)
	test.Assert(t, err == nil)
	tree := <-p.Provide()
	fmt.Printf("%#v\n", tree)
}
