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

	dthrift "github.com/cloudwego/dynamicgo/thrift"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
)

func TestAbsPath(t *testing.T) {
	t.Run("relative include path", func(t *testing.T) {
		path := "/a/b/c.thrift"
		includePath := "../d.thrift"
		result := "/a/d.thrift"
		p := absPath(path, includePath)
		test.DeepEqual(t, result, p)
	})
	t.Run("abs include path", func(t *testing.T) {
		path := "/a/b/c.thrift"
		includePath := "/a/d.thrift"
		result := "/a/d.thrift"
		p := absPath(path, includePath)
		test.DeepEqual(t, result, p)
	})
}

func TestThriftFileProvider(t *testing.T) {
	path := "http_test/idl/binary_echo.thrift"
	p, err := NewThriftFileProvider(path)
	test.Assert(t, err == nil)
	defer p.Close()
	pd, ok := p.(GetProviderOption)
	test.Assert(t, ok)
	test.Assert(t, !pd.Option().DynamicGoEnabled)
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc == nil)

	p, err = NewThriftFileProviderWithDynamicGo(path)
	test.Assert(t, err == nil)
	defer p.Close()
	pd, ok = p.(GetProviderOption)
	test.Assert(t, ok)
	test.Assert(t, pd.Option().DynamicGoEnabled)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
}

func TestThriftContentWithAbsIncludePathProvider(t *testing.T) {
	path := "a/b/main.thrift"
	content := `
	namespace go kitex.test.server
	include "x.thrift"
	include "../y.thrift" 

	service InboxService {}
	`
	newContent := `
	namespace go kitex.test.server
	include "x.thrift"
	include "../y.thrift"

	service UpdateService {}
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
	test.DeepEqual(t, nil, err)
	defer p.Close()
	test.Assert(t, !p.opts.DynamicGoEnabled)
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "InboxService")
	test.Assert(t, tree.DynamicGoDsc == nil)
	includes[path] = newContent
	err = p.UpdateIDL(path, includes)
	test.Assert(t, err == nil)
	defer p.Close()
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "UpdateService")
	test.Assert(t, tree.DynamicGoDsc == nil)

	includes[path] = content
	p, err = NewThriftContentWithAbsIncludePathProviderWithDynamicGo(path, includes)
	test.Assert(t, err == nil)
	defer p.Close()
	test.Assert(t, p.opts.DynamicGoEnabled)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "InboxService")
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Name() == "InboxService")
	includes[path] = newContent
	err = p.UpdateIDL(path, includes)
	test.Assert(t, err == nil)
	defer p.Close()
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "UpdateService")
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Name() == "UpdateService")
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

	content := `
	namespace go kitex.test.server

	service InboxService {}
	`
	newContent := `
	namespace go kitex.test.server

	service UpdateService {}
	`

	p, err = NewThriftContentProvider(content, nil)
	test.Assert(t, err == nil, err)
	defer p.Close()
	test.Assert(t, !p.opts.DynamicGoEnabled)
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "InboxService")
	test.Assert(t, tree.DynamicGoDsc == nil)
	err = p.UpdateIDL(newContent, nil)
	test.Assert(t, err == nil)
	defer p.Close()
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "UpdateService")
	test.Assert(t, tree.DynamicGoDsc == nil)

	p, err = NewThriftContentProviderWithDynamicGo(content, nil)
	test.Assert(t, err == nil, err)
	defer p.Close()
	test.Assert(t, p.opts.DynamicGoEnabled)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	err = p.UpdateIDL(newContent, nil)
	test.Assert(t, err == nil)
	defer p.Close()
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "UpdateService")
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Name() == "UpdateService")
}

func TestFallback(t *testing.T) {
	path := "http_test/idl/dynamicgo_go_tag_error.thrift"
	p, err := NewThriftFileProviderWithDynamicGo(path)
	test.Assert(t, err == nil)
	defer p.Close()
	pd, ok := p.(GetProviderOption)
	test.Assert(t, ok)
	test.Assert(t, pd.Option().DynamicGoEnabled)
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Functions["BinaryEcho"].Request.Struct.FieldsByName["req"].Type.Struct.FieldsByID[int32(1)].Alias == "STR")
	test.Assert(t, tree.DynamicGoDsc != nil)
}

func TestDisableGoTagForDynamicGo(t *testing.T) {
	path := "http_test/idl/binary_echo.thrift"
	p, err := NewThriftFileProviderWithDynamicGo(path)
	test.Assert(t, err == nil)
	defer p.Close()
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["BinaryEcho"].Request().Struct().FieldByKey("req").Type().Struct().FieldById(4).Alias() == "STR")

	dthrift.RemoveAnnotationMapper(dthrift.AnnoScopeField, "go.tag")
	p, err = NewThriftFileProviderWithDynamicGo(path)
	test.Assert(t, err == nil)
	defer p.Close()
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["BinaryEcho"].Request().Struct().FieldByKey("req").Type().Struct().FieldById(4).Alias() == "str")
}

func TestParseMode(t *testing.T) {
	path := "json_test/idl/example_multi_service.thrift"
	p, err := NewThriftFileProviderWithDynamicGo(path)
	test.Assert(t, err == nil)
	defer p.Close()
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "Example2Service")
	test.Assert(t, tree.DynamicGoDsc.Name() == "Example2Service")

	thrift.SetDefaultParseMode(thrift.FirstServiceOnly)
	p, err = NewThriftFileProviderWithDynamicGo(path)
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "ExampleService")
	test.Assert(t, tree.DynamicGoDsc.Name() == "ExampleService")
}
