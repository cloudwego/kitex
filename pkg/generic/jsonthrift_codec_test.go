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
	"context"
	"testing"
	"time"

	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/gopkg/bufiox"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestJsonThriftCodec(t *testing.T) {
	// without dynamicgo
	p, err := NewThriftFileProvider("./json_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	jtc := newJsonThriftCodec(p, nil)
	test.Assert(t, !jtc.dynamicgoEnabled)
	test.DeepEqual(t, jtc.convOpts, conv.Options{})
	test.DeepEqual(t, jtc.convOptsWithThriftBase, conv.Options{})
	test.DeepEqual(t, jtc.convOptsWithException, conv.Options{})
	defer jtc.Close()
	test.Assert(t, jtc.Name() == "JSONThrift")

	method, err := jtc.getMethod("Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)
	test.Assert(t, jtc.svcName.Load().(string) == "Mock")
	isCombineService, _ := jtc.combineService.Load().(bool)
	test.Assert(t, !isCombineService)

	rw := jtc.getMessageReaderWriter()
	_, ok := rw.(*thrift.JSONReaderWriter)
	test.Assert(t, ok)
}

func TestJsonThriftCodecWithDynamicGo(t *testing.T) {
	// with dynamicgo
	p, err := NewThriftFileProviderWithDynamicGo("./json_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	g, err := JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	jtc := g.(*jsonThriftGeneric).codec
	test.Assert(t, jtc.dynamicgoEnabled)
	assertDynamicgoOptions(t, DefaultJSONDynamicGoConvOpts, jtc.convOpts)
	test.Assert(t, jtc.convOptsWithException.ConvertException)
	assertDynamicgoOptions(t, DefaultJSONDynamicGoConvOpts, jtc.convOptsWithException)
	test.Assert(t, jtc.convOptsWithThriftBase.EnableThriftBase)
	test.Assert(t, jtc.convOptsWithThriftBase.MergeBaseFunc != nil)
	assertDynamicgoOptions(t, DefaultJSONDynamicGoConvOpts, jtc.convOptsWithThriftBase)
	defer jtc.Close()
	test.Assert(t, jtc.Name() == "JSONThrift")

	method, err := jtc.getMethod("Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)
	isCombineService, _ := jtc.combineService.Load().(bool)
	test.Assert(t, !isCombineService)

	rw := jtc.getMessageReaderWriter()
	_, ok := rw.(*thrift.JSONReaderWriter)
	test.Assert(t, ok)
}

func assertDynamicgoOptions(t *testing.T, exp, act conv.Options) {
	test.Assert(t, exp.ByteAsUint8 == act.ByteAsUint8, "ByteAsUint8")
	test.Assert(t, exp.DisallowUnknownField == act.DisallowUnknownField, "DisallowUnknownField")
	test.Assert(t, exp.EnableValueMapping == act.EnableValueMapping, "EnableValueMapping")
	test.Assert(t, exp.EnableHttpMapping == act.EnableHttpMapping, "EnableHttpMapping")
	test.Assert(t, exp.String2Int64 == act.String2Int64, "String2Int64")
	test.Assert(t, exp.Int642String == act.Int642String, "Int642String")
	test.Assert(t, exp.NoBase64Binary == act.NoBase64Binary, "NoBase64Binary")
	test.Assert(t, exp.WriteOptionalField == act.WriteOptionalField, "WriteOptionalField")
	test.Assert(t, exp.WriteDefaultField == act.WriteDefaultField, "WriteDefaultField")
	test.Assert(t, exp.WriteRequireField == act.WriteRequireField, "WriteRequireField")
	test.Assert(t, exp.ReadHttpValueFallback == act.ReadHttpValueFallback, "ReadHttpValueFallback")
	test.Assert(t, exp.WriteHttpValueFallback == act.WriteHttpValueFallback, "WriteHttpValueFallback")
	test.Assert(t, exp.OmitHttpMappingErrors == act.OmitHttpMappingErrors, "OmitHttpMappingErrors")
	test.Assert(t, exp.NoCopyString == act.NoCopyString, "NoCopyString")
	test.Assert(t, exp.UseNativeSkip == act.UseNativeSkip, "UseNativeSkip")
	test.Assert(t, exp.UseKitexHttpEncoding == act.UseKitexHttpEncoding, "UseKitexHttpEncoding")
}

func TestJsonThriftCodec_SelfRef(t *testing.T) {
	t.Run("without_dynamicgo", func(t *testing.T) {
		p, err := NewThriftFileProvider("./json_test/idl/self_ref.thrift")
		test.Assert(t, err == nil)
		jtc := newJsonThriftCodec(p, nil)
		defer jtc.Close()
		test.Assert(t, jtc.Name() == "JSONThrift")

		method, err := jtc.getMethod("Test")
		test.Assert(t, err == nil)
		test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)

		rw := jtc.getMessageReaderWriter()
		_, ok := rw.(thrift.MessageWriter)
		test.Assert(t, ok)
		_, ok = rw.(thrift.MessageReader)
		test.Assert(t, ok)
	})

	t.Run("with_dynamicgo", func(t *testing.T) {
		p, err := NewThriftFileProviderWithDynamicGo("./json_test/idl/self_ref.thrift")
		test.Assert(t, err == nil)
		gOpts := &Options{dynamicgoConvOpts: DefaultJSONDynamicGoConvOpts}
		jtc := newJsonThriftCodec(p, gOpts)
		defer jtc.Close()
		test.Assert(t, jtc.Name() == "JSONThrift")

		method, err := jtc.getMethod("Test")
		test.Assert(t, err == nil)
		test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)
		test.Assert(t, jtc.svcName.Load().(string) == "Mock")

		rw := jtc.getMessageReaderWriter()
		_, ok := rw.(thrift.MessageWriter)
		test.Assert(t, ok)
		_, ok = rw.(thrift.MessageReader)
		test.Assert(t, ok)
	})
}

func TestJsonThriftGenericUpdateMethods(t *testing.T) {
	content := `
	namespace go kitex.test.server

	service InboxService {
		string InBox(string msg)
	}`
	newContent := `
	namespace go kitex.test.server
	
	service UpdateService {
		string Update(string msg)
	}`
	p, err := NewThriftContentWithAbsIncludePathProvider("main.thrift", map[string]string{"main.thrift": content})
	test.Assert(t, err == nil)
	g, err := JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	genericMethod := g.GenericMethod()
	mi := genericMethod(context.Background(), "InBox")
	test.Assert(t, mi != nil)

	// update idl
	err = p.UpdateIDL("main.thrift", map[string]string{"main.thrift": newContent})
	test.Assert(t, err == nil)

	time.Sleep(100 * time.Millisecond)

	// test new method
	mi = genericMethod(context.Background(), "InBox")
	test.Assert(t, mi == nil)
	mi = genericMethod(context.Background(), "Update")
	test.Assert(t, mi != nil)
}

func TestJsonThriftGenericUpdateStructs(t *testing.T) {
	content := `
	namespace go kitex.test.server

	struct Test {
		1: optional string aaa
	}

	service InboxService {
		Test InBox(Test msg)
	}`
	newContent := `
	namespace go kitex.test.server

	struct Test {
		1: optional string aaa
		2: optional i64 bbb
	}

	service InboxService {
		Test InBox(Test msg)
	}`
	p, err := NewThriftContentWithAbsIncludePathProvider("main.thrift", map[string]string{"main.thrift": content})
	test.Assert(t, err == nil)
	g, err := JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	genericMethod := g.GenericMethod()
	mi := genericMethod(context.Background(), "InBox")
	test.Assert(t, mi != nil)

	jsonStr := `{"aaa":"123","bbb":123}`
	reversedJsonStr := `{"bbb":123,"aaa":"123"}`
	shortJsonStr := `{"aaa":"123"}`

	{
		var writeBuf []byte
		arg := mi.NewArgs().(*Args)
		arg.Request = jsonStr
		wbuf := bufiox.NewBytesWriter(&writeBuf)
		err = arg.Write(context.Background(), "InBox", wbuf)
		test.Assert(t, err == nil)
		err = wbuf.Flush()
		test.Assert(t, err == nil)

		arg = mi.NewArgs().(*Args)
		rbuf := bufiox.NewBytesReader(writeBuf)
		err = arg.Read(context.Background(), "InBox", len(writeBuf), rbuf)
		test.Assert(t, err == nil)
		test.Assert(t, arg.Request.(string) == shortJsonStr)
	}

	// update idl
	err = p.UpdateIDL("main.thrift", map[string]string{"main.thrift": newContent})
	test.Assert(t, err == nil)

	time.Sleep(100 * time.Millisecond)

	{
		var writeBuf []byte
		arg := mi.NewArgs().(*Args)
		arg.Request = jsonStr
		wbuf := bufiox.NewBytesWriter(&writeBuf)
		err = arg.Write(context.Background(), "InBox", wbuf)
		test.Assert(t, err == nil)
		err = wbuf.Flush()
		test.Assert(t, err == nil)

		arg = mi.NewArgs().(*Args)
		rbuf := bufiox.NewBytesReader(writeBuf)
		err = arg.Read(context.Background(), "InBox", len(writeBuf), rbuf)
		test.Assert(t, err == nil)
		test.Assert(t, arg.Request.(string) == jsonStr || arg.Request.(string) == reversedJsonStr)
	}
}
