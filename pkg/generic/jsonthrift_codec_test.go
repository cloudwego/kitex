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
	"testing"

	"github.com/cloudwego/dynamicgo/conv"

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

	method, err := jtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")
	test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)
	test.Assert(t, jtc.svcName == "Mock")
	test.Assert(t, jtc.extra[CombineServiceKey] == "false")

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

	method, err := jtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")
	test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)
	test.Assert(t, jtc.extra[CombineServiceKey] == "false")

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
	// typos:ignore
	test.Assert(t, exp.TracebackRequredOrRootFields == act.TracebackRequredOrRootFields, "TracebackRequredOrRootFields")
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

		method, err := jtc.getMethod(nil, "Test")
		test.Assert(t, err == nil)
		test.Assert(t, method.Name == "Test")
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

		method, err := jtc.getMethod(nil, "Test")
		test.Assert(t, err == nil)
		test.Assert(t, method.Name == "Test")
		test.Assert(t, method.StreamingMode == serviceinfo.StreamingNone)
		test.Assert(t, jtc.svcName == "Mock")

		rw := jtc.getMessageReaderWriter()
		_, ok := rw.(thrift.MessageWriter)
		test.Assert(t, ok)
		_, ok = rw.(thrift.MessageReader)
		test.Assert(t, ok)
	})
}
