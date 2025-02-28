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
	"github.com/cloudwego/kitex/pkg/generic/thrift"
)

var testServiceContent = `
		namespace go kitex.test.server
		
		enum FOO {
		A = 1;
		}
		
		struct ExampleReq {
		1: required string Msg (go.tag = "json:\"message\""),
		2: FOO Foo,
		4: optional i8 I8 = 8,
		5: optional i16 I16,
		6: optional i32 I32,
		7: optional i64 I64,
		8: optional double Double,
		}
		struct ExampleResp {
		1: required string Msg (go.tag = "json:\"message\""),
		2: string required_field,
		3: optional i64 num (api.js_conv="true"),
		4: optional i8 I8,
		5: optional i16 I16,
		6: optional i32 I32,
		7: optional i64 I64,
		8: optional double Double,
		}

		exception Exception {
			1: i32 code
			255: string msg  (go.tag = "json:\"excMsg\""),
		}
		
		service ExampleService {
		ExampleResp ExampleMethod(1: ExampleReq req),
		}
		
		service Example2Service {
		ExampleResp Example2Method(1: ExampleReq req)throws(1: Exception err)
		}
	`

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

	treeTest := func(t *testing.T, p *ThriftContentWithAbsIncludePathProvider, svcName string, withDynamicgo, shouldClose bool) {
		tree := <-p.Provide()
		test.Assert(t, tree != nil)
		test.Assert(t, tree.Name == svcName)
		if !withDynamicgo {
			test.Assert(t, tree.DynamicGoDsc == nil)
		} else {
			test.Assert(t, tree.DynamicGoDsc != nil)
			test.Assert(t, tree.DynamicGoDsc.Name() == svcName)
		}
		if shouldClose {
			p.Close()
		}
	}

	p, err := NewThriftContentWithAbsIncludePathProvider(path, includes)
	test.Assert(t, err == nil)
	test.Assert(t, !p.opts.DynamicGoEnabled)
	treeTest(t, p, "InboxService", false, false)
	// update idl
	includes[path] = newContent
	err = p.UpdateIDL(path, includes)
	test.Assert(t, err == nil)
	treeTest(t, p, "UpdateService", false, true)

	includes[path] = content
	p, err = NewThriftContentWithAbsIncludePathProviderWithDynamicGo(path, includes)
	test.Assert(t, err == nil)
	test.Assert(t, p.opts.DynamicGoEnabled)
	treeTest(t, p, "InboxService", true, false)
	// update idl
	includes[path] = newContent
	err = p.UpdateIDL(path, includes)
	test.Assert(t, err == nil)
	treeTest(t, p, "UpdateService", true, true)

	// with absolute path
	delete(includes, "a/b/x.thrift")
	includes["x.thrift"] = "namespace go kitex.test.server"

	p, err = NewThriftContentWithAbsIncludePathProvider(path, includes)
	test.Assert(t, err == nil)
	treeTest(t, p, "UpdateService", false, true)

	p, err = NewThriftContentWithAbsIncludePathProviderWithDynamicGo(path, includes)
	test.Assert(t, err == nil)
	test.Assert(t, p.opts.DynamicGoEnabled)
	treeTest(t, p, "UpdateService", true, true)
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

func TestDisableGoTag(t *testing.T) {
	path := "http_test/idl/http_annotation.thrift"
	p, err := NewThriftFileProvider(path)
	test.Assert(t, err == nil)
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Functions["BizMethod1"].Request.Struct.FieldsByName["req"].Type.Struct.FieldsByID[5].Type.Struct.FieldsByID[1].FieldName() == "MyID")
	p.Close()

	p, err = NewThriftFileProviderWithOption(path, []ThriftIDLProviderOption{WithGoTagDisabled(true)})
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Functions["BizMethod1"].Request.Struct.FieldsByName["req"].Type.Struct.FieldsByID[5].Type.Struct.FieldsByID[1].FieldName() == "id")
	p.Close()

	p, err = NewThriftFileProviderWithOption(path, []ThriftIDLProviderOption{WithGoTagDisabled(false)})
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Functions["BizMethod1"].Request.Struct.FieldsByName["req"].Type.Struct.FieldsByID[5].Type.Struct.FieldsByID[1].FieldName() == "MyID")
	p.Close()

	p, err = NewThriftContentProvider(testServiceContent, nil)
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Functions["Example2Method"].Request.Struct.FieldsByName["req"].Type.Struct.FieldsByID[1].FieldName() == "message")
	p.Close()

	p, err = NewThriftContentProvider(testServiceContent, nil, WithGoTagDisabled(true))
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Functions["Example2Method"].Request.Struct.FieldsByName["req"].Type.Struct.FieldsByID[1].FieldName() == "Msg")
	p.Close()

	path = "json_test/idl/example_multi_service.thrift"
	includes := map[string]string{path: testServiceContent}
	p, err = NewThriftContentWithAbsIncludePathProvider(path, includes)
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Functions["Example2Method"].Response.Struct.FieldsByID[0].Type.Struct.FieldsByID[1].FieldName() == "message")
	test.Assert(t, tree.Functions["Example2Method"].Response.Struct.FieldsByID[1].Type.Struct.FieldsByID[255].FieldName() == "excMsg")
	p.Close()

	p, err = NewThriftContentWithAbsIncludePathProvider(path, includes, WithGoTagDisabled(true))
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Functions["Example2Method"].Response.Struct.FieldsByID[0].Type.Struct.FieldsByID[1].FieldName() == "Msg")
	test.Assert(t, tree.Functions["Example2Method"].Response.Struct.FieldsByID[1].Type.Struct.FieldsByID[255].FieldName() == "msg")
	p.Close()
}

func TestDisableGoTagForDynamicGo(t *testing.T) {
	path := "http_test/idl/binary_echo.thrift"
	p, err := NewThriftFileProviderWithDynamicGo(path)
	test.Assert(t, err == nil)
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["BinaryEcho"].Request().Struct().FieldByKey("req").Type().Struct().FieldById(4).Alias() == "STR")
	p.Close()

	p, err = NewThriftFileProviderWithDynamicgoWithOption(path, []ThriftIDLProviderOption{WithGoTagDisabled(true)})
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["BinaryEcho"].Request().Struct().FieldByKey("req").Type().Struct().FieldById(4).Alias() == "str")
	p.Close()

	p, err = NewThriftFileProviderWithDynamicgoWithOption(path, []ThriftIDLProviderOption{WithGoTagDisabled(false)})
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["BinaryEcho"].Request().Struct().FieldByKey("req").Type().Struct().FieldById(4).Alias() == "STR")
	p.Close()

	p, err = NewThriftContentProviderWithDynamicGo(testServiceContent, nil, WithGoTagDisabled(true))
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["Example2Method"].Request().Struct().FieldByKey("req").Type().Struct().FieldById(1).Alias() == "Msg")
	p.Close()

	p, err = NewThriftContentProviderWithDynamicGo(testServiceContent, nil)
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["Example2Method"].Request().Struct().FieldByKey("req").Type().Struct().FieldById(1).Alias() == "message")
	p.Close()

	p, err = NewThriftContentProviderWithDynamicGo(testServiceContent, nil, WithGoTagDisabled(true))
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["Example2Method"].Request().Struct().FieldByKey("req").Type().Struct().FieldById(1).Alias() == "Msg")
	p.Close()

	path = "json_test/idl/example_multi_service.thrift"
	includes := map[string]string{path: testServiceContent}
	p, err = NewThriftContentWithAbsIncludePathProviderWithDynamicGo(path, includes)
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["Example2Method"].Response().Struct().FieldById(0).Type().Struct().FieldById(1).Alias() == "message")
	test.Assert(t, tree.DynamicGoDsc.Functions()["Example2Method"].Response().Struct().FieldById(1).Type().Struct().FieldById(255).Alias() == "excMsg")
	p.Close()

	p, err = NewThriftContentWithAbsIncludePathProviderWithDynamicGo(path, includes, WithGoTagDisabled(true))
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Functions()["Example2Method"].Response().Struct().FieldById(0).Type().Struct().FieldById(1).Alias() == "Msg")
	test.Assert(t, tree.DynamicGoDsc.Functions()["Example2Method"].Response().Struct().FieldById(1).Type().Struct().FieldById(255).Alias() == "msg")
	p.Close()
}

func TestFileProviderParseMode(t *testing.T) {
	path := "json_test/idl/example_multi_service.thrift"
	thrift.SetDefaultParseMode(thrift.LastServiceOnly)
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	p, err := NewThriftFileProvider(path)
	test.Assert(t, err == nil)
	defer p.Close()
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "Example2Service")

	opts := []ThriftIDLProviderOption{WithParseMode(thrift.CombineServices)}
	p, err = NewThriftFileProviderWithOption(path, opts)
	// global var isn't affected by the option
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "CombinedServices")
}

func TestFileProviderParseModeWithDynamicGo(t *testing.T) {
	path := "json_test/idl/example_multi_service.thrift"
	thrift.SetDefaultParseMode(thrift.LastServiceOnly)
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	p, err := NewThriftFileProviderWithDynamicGo(path)
	test.Assert(t, err == nil)
	defer p.Close()
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "Example2Service")
	test.Assert(t, tree.DynamicGoDsc.Name() == "Example2Service")

	opts := []ThriftIDLProviderOption{WithParseMode(thrift.FirstServiceOnly)}
	p, err = NewThriftFileProviderWithDynamicgoWithOption(path, opts)
	// global var isn't affected by the option
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "ExampleService")
	test.Assert(t, tree.DynamicGoDsc.Name() == "ExampleService")
}

func TestContentProviderParseMode(t *testing.T) {
	thrift.SetDefaultParseMode(thrift.LastServiceOnly)
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	p, err := NewThriftContentProvider(testServiceContent, nil)
	test.Assert(t, err == nil)
	defer p.Close()
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "Example2Service")

	p, err = NewThriftContentProvider(testServiceContent, nil, WithParseMode(thrift.CombineServices))
	test.Assert(t, err == nil)
	// global var isn't affected by the option
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "CombinedServices")
}

func TestContentProviderParseModeWithDynamicGo(t *testing.T) {
	thrift.SetDefaultParseMode(thrift.LastServiceOnly)
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	p, err := NewThriftContentProviderWithDynamicGo(testServiceContent, nil)
	test.Assert(t, err == nil)
	defer p.Close()
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "Example2Service")
	test.Assert(t, tree.DynamicGoDsc.Name() == "Example2Service")

	p, err = NewThriftContentProviderWithDynamicGo(testServiceContent, nil, WithParseMode(thrift.CombineServices))
	test.Assert(t, err == nil)
	// global var isn't affected by the option
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "CombinedServices")
	test.Assert(t, tree.DynamicGoDsc.Name() == "CombinedServices")
}

func TestContentWithAbsIncludePathProviderParseMode(t *testing.T) {
	path := "json_test/idl/example_multi_service.thrift"
	includes := map[string]string{path: testServiceContent}
	thrift.SetDefaultParseMode(thrift.LastServiceOnly)
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	p, err := NewThriftContentWithAbsIncludePathProvider(path, includes)
	test.Assert(t, err == nil, err)
	defer p.Close()
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "Example2Service")

	p, err = NewThriftContentWithAbsIncludePathProvider(path, includes, WithParseMode(thrift.FirstServiceOnly))
	test.Assert(t, err == nil)
	// global var isn't affected by the option
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "ExampleService")
}

func TestContentWithAbsIncludePathProviderParseModeWithDynamicGo(t *testing.T) {
	path := "json_test/idl/example_multi_service.thrift"
	includes := map[string]string{path: testServiceContent}
	thrift.SetDefaultParseMode(thrift.LastServiceOnly)
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	p, err := NewThriftContentWithAbsIncludePathProviderWithDynamicGo(path, includes)
	test.Assert(t, err == nil, err)
	defer p.Close()
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "Example2Service")
	test.Assert(t, tree.DynamicGoDsc.Name() == "Example2Service")

	p, err = NewThriftContentWithAbsIncludePathProviderWithDynamicGo(path, includes, WithParseMode(thrift.CombineServices))
	test.Assert(t, err == nil)
	// global var isn't affected by the option
	test.Assert(t, thrift.DefaultParseMode() == thrift.LastServiceOnly)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "CombinedServices")
	test.Assert(t, tree.DynamicGoDsc.Name() == "CombinedServices")
}

func TestDefaultValue(t *testing.T) {
	// file provider
	path := "http_test/idl/binary_echo.thrift"
	p, err := NewThriftFileProviderWithDynamicGo(path)
	test.Assert(t, err == nil)
	pd, ok := p.(GetProviderOption)
	test.Assert(t, ok)
	test.Assert(t, pd.Option().DynamicGoEnabled)
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.Functions["BinaryEcho"].Request.Struct.FieldsByID[1].Type.Struct.FieldsByID[4].DefaultValue == "echo")
	test.Assert(t, tree.DynamicGoDsc.Functions()["BinaryEcho"].Request().Struct().FieldById(1).Type().Struct().FieldById(4).DefaultValue().JSONValue() == `"echo"`)
	p.Close()

	// content provider
	p, err = NewThriftContentProviderWithDynamicGo(testServiceContent, nil)
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "Example2Service")
	test.Assert(t, tree.Functions["Example2Method"].Request.Struct.FieldsByID[1].Type.Struct.FieldsByID[4].DefaultValue == uint8(8))
	test.Assert(t, tree.DynamicGoDsc.Name() == "Example2Service")
	test.Assert(t, tree.DynamicGoDsc.Functions()["Example2Method"].Request().Struct().FieldById(1).Type().Struct().FieldById(4).DefaultValue().JSONValue() == "8")
	p.Close()

	// content with abs include path provider
	path = "json_test/idl/example_multi_service.thrift"
	includes := map[string]string{path: testServiceContent}
	p, err = NewThriftContentWithAbsIncludePathProviderWithDynamicGo(path, includes)
	test.Assert(t, err == nil, err)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "Example2Service")
	test.Assert(t, tree.DynamicGoDsc.Name() == "Example2Service")
	test.Assert(t, tree.DynamicGoDsc.Name() == "Example2Service")
	test.Assert(t, tree.DynamicGoDsc.Functions()["Example2Method"].Request().Struct().FieldById(1).Type().Struct().FieldById(4).DefaultValue().JSONValue() == "8")
	p.Close()
}

func TestParseWithIDLServiceName(t *testing.T) {
	path := "json_test/idl/example_multi_service.thrift"
	opts := []ThriftIDLProviderOption{WithIDLServiceName("ExampleService")}
	p, err := NewThriftFileProviderWithOption(path, opts)
	test.Assert(t, err == nil)
	tree := <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "ExampleService")
	p.Close()

	p, err = NewThriftFileProviderWithDynamicgoWithOption(path, opts)
	test.Assert(t, err == nil)
	tree = <-p.Provide()
	test.Assert(t, tree != nil)
	test.Assert(t, tree.Name == "ExampleService")
	test.Assert(t, tree.DynamicGoDsc.Name() == "ExampleService")
	p.Close()

	content := `
	namespace go thrift
	
	struct Request {
		1: required string message,
	}
	
	struct Response {
		1: required string message,
	}
	
	service Service1 {
		Response Test(1: Request req)
	}

	service Service2 {
		Response Test(1: Request req)
	}

	service Service3 {
		Response Test(1: Request req)
	}
	`

	updateContent := `
	namespace go thrift
	
	struct Request {
		1: required string message,
	}
	
	struct Response {
		1: required string message,
	}
	
	service Service1 {
		Response Test(1: Request req)
	}

	service Service2 {
		Response Test(1: Request req)
	}
	`
	cp, err := NewThriftContentProvider(content, nil, WithIDLServiceName("Service2"))
	test.Assert(t, err == nil)
	tree = <-cp.Provide()
	test.Assert(t, tree.Name == "Service2")
	cp.Close()

	cp, err = NewThriftContentProvider(content, nil, WithIDLServiceName("UnknownService"))
	test.Assert(t, err != nil)
	test.Assert(t, err.Error() == "the idl service name UnknownService is not in the idl. Please check your idl")
	test.Assert(t, cp == nil)

	cp, err = NewThriftContentProviderWithDynamicGo(content, nil, WithIDLServiceName("Service1"))
	test.Assert(t, err == nil)
	tree = <-cp.Provide()
	test.Assert(t, tree.Name == "Service1")
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Name() == "Service1")

	err = cp.UpdateIDL(updateContent, nil)
	test.Assert(t, err == nil)
	tree = <-cp.Provide()
	test.Assert(t, tree.Name == "Service1")
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Name() == "Service1")
	cp.Close()

	path = "a/b/main.thrift"
	includes := map[string]string{path: content}
	ap, err := NewThriftContentWithAbsIncludePathProvider(path, includes, WithIDLServiceName("Service2"))
	test.Assert(t, err == nil)
	tree = <-ap.Provide()
	test.Assert(t, tree.Name == "Service2")
	ap.Close()

	ap, err = NewThriftContentWithAbsIncludePathProviderWithDynamicGo(path, includes, WithIDLServiceName("Service3"))
	test.Assert(t, err == nil)
	tree = <-ap.Provide()
	test.Assert(t, tree.Name == "Service3")
	test.Assert(t, tree.DynamicGoDsc != nil)
	test.Assert(t, tree.DynamicGoDsc.Name() == "Service3")

	err = ap.UpdateIDL(path, map[string]string{path: updateContent})
	test.Assert(t, err != nil)
	test.Assert(t, err.Error() == "the idl service name Service3 is not in the idl. Please check your idl")
	ap.Close()
}
