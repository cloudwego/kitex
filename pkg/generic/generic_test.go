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
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/dynamicgo/meta"
	dproto "github.com/cloudwego/dynamicgo/proto"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/thrift"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestBinaryThriftGeneric(t *testing.T) {
	g := BinaryThriftGeneric()
	defer g.Close()

	test.Assert(t, g.GetExtra(BinaryThriftGenericV1PayloadCodecKey).(remote.PayloadCodec).Name() == "RawThriftBinary")
	test.Assert(t, g.PayloadCodecType() == serviceinfo.Thrift)

	test.Assert(t, g.GenericMethod()(context.Background(), "Test") != nil)
}

func TestMapThriftGeneric(t *testing.T) {
	p, err := NewThriftFileProvider("./map_test/idl/mock.thrift")
	test.Assert(t, err == nil)

	// new
	g, err := MapThriftGeneric(p)
	test.Assert(t, err == nil)
	defer g.Close()

	mg, ok := g.(*mapThriftGeneric)
	test.Assert(t, ok)

	test.Assert(t, g.IDLServiceName() == "Mock")

	isCombineService, _ := g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, !isCombineService)

	err = SetBinaryWithBase64(g, true)
	test.Assert(t, err == nil)
	test.Assert(t, mg.codec.binaryWithBase64)

	err = SetBinaryWithByteSlice(g, true)
	test.Assert(t, err == nil)
	test.Assert(t, mg.codec.binaryWithByteSlice)

	test.Assert(t, g.PayloadCodecType() == serviceinfo.Thrift)

	test.Assert(t, g.GenericMethod()(context.Background(), "Test") != nil)
}

func TestMapThriftGenericForJSON(t *testing.T) {
	p, err := NewThriftFileProvider("./map_test/idl/mock.thrift")
	test.Assert(t, err == nil)

	// new
	g, err := MapThriftGenericForJSON(p)
	test.Assert(t, err == nil)
	defer g.Close()

	mg, ok := g.(*mapThriftGeneric)
	test.Assert(t, ok)

	test.Assert(t, g.IDLServiceName() == "Mock")

	isCombineService, _ := g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, !isCombineService)

	err = SetBinaryWithBase64(g, true)
	test.Assert(t, err == nil)
	test.Assert(t, mg.codec.binaryWithBase64)

	test.Assert(t, g.PayloadCodecType() == serviceinfo.Thrift)

	test.Assert(t, g.GenericMethod()(context.Background(), "Test") != nil)
}

func TestHTTPThriftGeneric(t *testing.T) {
	p, err := NewThriftFileProvider("./http_test/idl/binary_echo.thrift")
	test.Assert(t, err == nil)

	var opts []Option
	opts = append(opts, UseRawBodyForHTTPResp(true))
	g, err := HTTPThriftGeneric(p, opts...)
	test.Assert(t, err == nil)
	defer g.Close()

	hg, ok := g.(*httpThriftGeneric)
	test.Assert(t, ok)

	test.Assert(t, g.IDLServiceName() == "ExampleService")

	isCombineService, _ := g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, !isCombineService)

	test.Assert(t, !hg.codec.dynamicgoEnabled)
	test.Assert(t, hg.codec.useRawBodyForHTTPResp)
	test.Assert(t, !hg.codec.binaryWithBase64)
	test.Assert(t, !hg.codec.convOpts.NoBase64Binary)
	test.Assert(t, !hg.codec.convOptsWithThriftBase.NoBase64Binary)

	err = SetBinaryWithBase64(g, true)
	test.Assert(t, err == nil)
	test.Assert(t, hg.codec.binaryWithBase64)
	test.Assert(t, !hg.codec.convOpts.NoBase64Binary)
	test.Assert(t, !hg.codec.convOptsWithThriftBase.NoBase64Binary)

	test.Assert(t, g.PayloadCodecType() == serviceinfo.Thrift)

	url := "https://example.com/BinaryEcho"
	body := map[string]interface{}{
		"msg": "Test",
	}
	data, err := customJson.Marshal(body)
	test.Assert(t, err == nil)
	// NewRequest
	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	test.Assert(t, err == nil)
	customReq, err := FromHTTPRequest(req)
	test.Assert(t, err == nil)

	method, _ := g.GetExtra(GetMethodNameByRequestFuncKey).(GetMethodNameByRequestFunc)(customReq)
	test.Assert(t, method == "BinaryEcho")

	test.Assert(t, g.GenericMethod()(context.Background(), method) != nil)
}

func TestHTTPThriftGenericWithDynamicGo(t *testing.T) {
	p, err := NewThriftFileProviderWithDynamicGo("./http_test/idl/binary_echo.thrift")
	test.Assert(t, err == nil)

	g, err := HTTPThriftGeneric(p)
	test.Assert(t, err == nil)
	defer g.Close()

	hg, ok := g.(*httpThriftGeneric)
	test.Assert(t, ok)

	test.Assert(t, g.IDLServiceName() == "ExampleService")

	isCombineService, _ := g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, !isCombineService)

	test.Assert(t, hg.codec.dynamicgoEnabled)
	test.Assert(t, !hg.codec.useRawBodyForHTTPResp)
	test.Assert(t, !hg.codec.binaryWithBase64)
	test.Assert(t, hg.codec.convOpts.NoBase64Binary)
	test.Assert(t, hg.codec.convOptsWithThriftBase.NoBase64Binary)

	err = SetBinaryWithBase64(g, true)
	test.Assert(t, err == nil)
	test.Assert(t, hg.codec.binaryWithBase64)
	test.Assert(t, !hg.codec.convOpts.NoBase64Binary)
	test.Assert(t, !hg.codec.convOptsWithThriftBase.NoBase64Binary)

	test.Assert(t, g.PayloadCodecType() == serviceinfo.Thrift)

	url := "https://example.com/BinaryEcho"
	body := map[string]interface{}{
		"msg": "Test",
	}
	data, err := customJson.Marshal(body)
	test.Assert(t, err == nil)
	// NewRequest
	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	test.Assert(t, err == nil)
	customReq, err := FromHTTPRequest(req)
	test.Assert(t, err == nil)

	method, _ := g.GetExtra(GetMethodNameByRequestFuncKey).(GetMethodNameByRequestFunc)(customReq)
	test.Assert(t, method == "BinaryEcho")

	test.Assert(t, g.GenericMethod()(context.Background(), method) != nil)
}

func TestJSONThriftGeneric(t *testing.T) {
	p, err := NewThriftFileProvider("./json_test/idl/mock.thrift")
	test.Assert(t, err == nil)

	g, err := JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	defer g.Close()

	jg, ok := g.(*jsonThriftGeneric)
	test.Assert(t, ok)

	test.Assert(t, g.IDLServiceName() == "Mock")

	isCombineService, _ := g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, !isCombineService)

	test.Assert(t, !jg.codec.dynamicgoEnabled)
	test.Assert(t, jg.codec.binaryWithBase64)
	test.Assert(t, !jg.codec.convOpts.NoBase64Binary)
	test.Assert(t, !jg.codec.convOptsWithThriftBase.NoBase64Binary)

	err = SetBinaryWithBase64(g, false)
	test.Assert(t, err == nil)
	test.Assert(t, !jg.codec.binaryWithBase64)
	test.Assert(t, !jg.codec.convOpts.NoBase64Binary)
	test.Assert(t, !jg.codec.convOptsWithThriftBase.NoBase64Binary)

	test.Assert(t, g.PayloadCodecType() == serviceinfo.Thrift)

	test.Assert(t, g.GenericMethod()(context.Background(), "Test") != nil)
}

func TestJSONThriftGenericWithDynamicGo(t *testing.T) {
	p, err := NewThriftFileProviderWithDynamicGo("./json_test/idl/mock.thrift")
	test.Assert(t, err == nil)

	g, err := JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	defer g.Close()

	jg, ok := g.(*jsonThriftGeneric)
	test.Assert(t, ok)

	test.Assert(t, g.IDLServiceName() == "Mock")

	isCombineService, _ := g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, !isCombineService)

	test.Assert(t, jg.codec.dynamicgoEnabled)
	test.Assert(t, jg.codec.binaryWithBase64)
	test.Assert(t, !jg.codec.convOpts.NoBase64Binary)
	test.Assert(t, !jg.codec.convOptsWithThriftBase.NoBase64Binary)

	err = SetBinaryWithBase64(g, false)
	test.Assert(t, err == nil)
	test.Assert(t, !jg.codec.binaryWithBase64)
	test.Assert(t, jg.codec.convOpts.NoBase64Binary)
	test.Assert(t, jg.codec.convOptsWithThriftBase.NoBase64Binary)

	test.Assert(t, g.PayloadCodecType() == serviceinfo.Thrift)

	test.Assert(t, g.GenericMethod()(context.Background(), "Test") != nil)
}

func TestJSONPbGeneric(t *testing.T) {
	path := "./jsonpb_test/idl/echo.proto"

	// initialise dynamicgo proto.ServiceDescriptor
	opts := dproto.Options{}
	p, err := NewPbFileProviderWithDynamicGo(path, context.Background(), opts)
	if err != nil {
		panic(err)
	}

	g, err := JSONPbGeneric(p)
	test.Assert(t, err == nil)
	defer g.Close()

	test.Assert(t, g.IDLServiceName() == "Echo")

	isCombineService, _ := g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, !isCombineService)

	test.Assert(t, g.PayloadCodecType() == serviceinfo.Protobuf)

	jg, ok := g.(*jsonPbGeneric)
	test.Assert(t, ok)

	test.Assert(t, jg.codec.dynamicgoEnabled == true)
	test.Assert(t, !jg.codec.convOpts.NoBase64Binary)

	test.Assert(t, err == nil)

	test.Assert(t, g.GenericMethod()(context.Background(), "Echo") != nil)
}

func TestIsCombinedServices(t *testing.T) {
	path := "./json_test/idl/example_multi_service.thrift"

	// normal thrift
	opts := []ThriftIDLProviderOption{WithParseMode(thrift.CombineServices)}
	p, err := NewThriftFileProviderWithOption(path, opts)
	test.Assert(t, err == nil)

	g, err := JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	g.Close()

	isCombineService, _ := g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, isCombineService)

	// thrift with dynamicgo
	p, err = NewThriftFileProviderWithDynamicgoWithOption(path, opts)
	test.Assert(t, err == nil)

	g, err = JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	g.Close()

	isCombineService, _ = g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, isCombineService)

	// pb with dynamicgo
	pbp, err := NewPbFileProviderWithDynamicGo(
		"./grpcjsonpb_test/idl/pbapi_multi_service.proto",
		context.Background(),
		dproto.Options{ParseServiceMode: meta.CombineServices},
	)
	test.Assert(t, err == nil)

	g, err = JSONPbGeneric(pbp)
	test.Assert(t, err == nil)
	g.Close()

	isCombineService, _ = g.GetExtra(serviceinfo.CombineServiceKey).(bool)
	test.Assert(t, isCombineService)

	// TODO: test pb after supporting parse mode
}

func TestBinaryPbGeneric(t *testing.T) {
	svcName := "TestService"
	packageName := "test.package"
	g := BinaryPbGeneric(svcName, packageName)
	defer g.Close()

	test.Assert(t, g.PayloadCodecType() == serviceinfo.Protobuf)
	test.Assert(t, g.IDLServiceName() == svcName)

	_packageName, _ := g.GetExtra(serviceinfo.PackageName).(string)
	test.Assert(t, _packageName == packageName)

	test.Assert(t, g.GenericMethod()(context.Background(), "Test") != nil)
}

func TestBinaryThriftGenericV2(t *testing.T) {
	svcName := "TestService"
	g := BinaryThriftGenericV2(svcName)
	defer g.Close()

	test.Assert(t, g.PayloadCodecType() == serviceinfo.Thrift)
	test.Assert(t, g.IDLServiceName() == svcName)

	test.Assert(t, g.GenericMethod()(context.Background(), "Test") != nil)
}
