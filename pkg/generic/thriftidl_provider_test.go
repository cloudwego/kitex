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
	"strings"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
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
	defer p.Close()
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	err = p.UpdateIDL(path, includes)
	test.Assert(t, err == nil)
}

func TestCircularDependency(t *testing.T) {
	main := "a.thrift"
	content := `
	include "b.thrift"
	include "c.thrift" 

	service a {}
	`
	includes := map[string]string{
		main: content,
		"b.thrift": `
		include "c.thrift"
		include "d.thrift"
		`,
		"c.thrift": `
		include "d.thrift"
		`,
		"d.thrift": `include "a.thrift"`,
	}
	p, err := NewThriftContentWithAbsIncludePathProvider(main, includes)
	test.Assert(t, err != nil && p == nil)
	test.Assert(t, strings.HasPrefix(err.Error(), "IDL circular dependency:"))
}

func TestThriftContentProvider(t *testing.T) {
	p, err := NewThriftContentProvider("test", nil)
	test.Assert(t, err != nil)
	test.Assert(t, p == nil)
}
