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

package test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/dynamicgo/conv"
	"github.com/tidwall/gjson"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	jtest "github.com/cloudwego/kitex/pkg/generic/json_test"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/transport"
)

func TestRun(t *testing.T) {
	t.Run("TestThriftNormalBinaryEcho", testThriftNormalBinaryEcho)
	t.Run("TestThriftDynamicgo", testThriftDynamicgo)
	t.Run("TestRegression", testRegression)
	t.Run("TestThriftBase64BinaryEcho", testThriftBase64BinaryEcho)
	t.Run("TestDynamicgoThriftBase64BinaryEcho", testDynamicgoThriftBase64BinaryEcho)
}

// TODO: exception test for normal
// TODO: exception test for dynamicgo

func initThriftClientByIDL(t *testing.T, addr, idl string, base64Binary bool) genericclient.Client {
	p, err := generic.NewThriftFileProvider(idl)
	test.Assert(t, err == nil)
	g, err := generic.HTTPThriftGeneric(p)
	test.Assert(t, err == nil)
	err = generic.SetBinaryWithBase64(g, base64Binary)
	test.Assert(t, err == nil)
	cli := newGenericClient("destServiceName", g, addr)
	test.Assert(t, err == nil)
	return cli
}

func initThriftServer(t *testing.T, address string, handler generic.Service) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewThriftFileProvider("./idl/binary_echo.thrift")
	test.Assert(t, err == nil)
	g, err := generic.MapThriftGeneric(p)
	test.Assert(t, err == nil)
	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func initOriginalThriftServer(t *testing.T, address string, handler generic.Service, idlPath string) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewThriftFileProvider(idlPath)
	test.Assert(t, err == nil)
	g, err := generic.MapThriftGeneric(p)
	test.Assert(t, err == nil)
	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func initDynamicgoThriftClientByIDL(tp transport.Protocol, t *testing.T, addr, idl string, opts generic.Options) genericclient.Client {
	p, err := generic.NewThriftFileProvider(idl)
	test.Assert(t, err == nil)
	g, err := generic.HTTPThriftGeneric(p, opts)
	test.Assert(t, err == nil)
	cli := newGenericDynamicgoClient(tp, "destServiceName", g, addr)
	test.Assert(t, err == nil)
	return cli
}

func testThriftNormalBinaryEcho(t *testing.T) {
	svr := initThriftServer(t, ":8126", new(GenericServiceBinaryEchoImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:8126", "./idl/binary_echo.thrift", false)
	url := "http://example.com/BinaryEcho"

	// []byte value for binary field
	body := map[string]interface{}{
		"msg":        []byte(mockMyMsg),
		"got_base64": true,
		"num":        "",
	}
	data, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err := generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	gr, ok := resp.(*generic.HTTPResponse)
	test.Assert(t, ok)
	test.Assert(t, gr.Body["msg"] == base64.StdEncoding.EncodeToString([]byte(mockMyMsg)))
	test.Assert(t, gr.Body["num"] == "0")

	// string value for binary field which should fail
	body = map[string]interface{}{
		"msg":        string(mockMyMsg),
		"got_base64": false,
		"num":        0,
	}
	data, err = json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err = http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err = generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	gr, ok = resp.(*generic.HTTPResponse)
	test.Assert(t, ok)
	test.Assert(t, gr.Body["msg"] == mockMyMsg)

	// []byte value for binary field
	body = map[string]interface{}{
		"msg":        []byte(mockMyMsg),
		"got_base64": true,
	}
	data, err = json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err = http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err = generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	gr, ok = resp.(*generic.HTTPResponse)
	test.Assert(t, ok)
	test.Assert(t, gr.Body["msg"] == base64.StdEncoding.EncodeToString([]byte(mockMyMsg)))
	test.Assert(t, gr.Body["num"] == "0")

	body = map[string]interface{}{
		"msg":        []byte(mockMyMsg),
		"got_base64": true,
		"num":        "123",
	}
	data, err = json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err = http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err = generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err.Error() == "remote or network error[remote]: biz error: call failed, incorrect num")

	svr.Stop()
}

func testThriftDynamicgo(t *testing.T) {
	svr := initThriftServer(t, ":8126", new(GenericServiceBinaryEchoImpl))
	time.Sleep(500 * time.Millisecond)

	convOpts := conv.Options{EnableValueMapping: true, NoBase64Binary: true}
	dOpts := generic.Options{DynamicgoConvOpts: convOpts, EnableDynamicgoHTTPResp: true}

	cli := initDynamicgoThriftClientByIDL(transport.TTHeader, t, "127.0.0.1:8126", "./idl/binary_echo.thrift", dOpts)
	url := "http://example.com/BinaryEcho"

	// []byte value for binary field
	body := map[string]interface{}{
		"msg":        []byte(mockMyMsg),
		"got_base64": true,
		"num":        "",
	}
	data, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err := generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	// write: dynamicgo (amd64), fallback (arm)
	// read: dynamicgo
	resp, err := cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	gr, ok := resp.(*generic.HTTPResponse)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(string(gr.GeneralBody.([]byte)), "msg").String(), base64.StdEncoding.EncodeToString([]byte(mockMyMsg))), gjson.Get(string(gr.GeneralBody.([]byte)), "msg").String())

	// write: dynamicgo (amd64), fallback (arm)
	// read: fallback
	cli = initDynamicgoThriftClientByIDL(transport.PurePayload, t, "127.0.0.1:8126", "./idl/binary_echo.thrift", dOpts)
	resp, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	gr, ok = resp.(*generic.HTTPResponse)
	test.Assert(t, ok)
	test.Assert(t, gr.Body["msg"] == base64.StdEncoding.EncodeToString([]byte(mockMyMsg)))

	// write: dynamicgo (amd64), fallback (arm)
	// read: fallback
	dOpts = generic.Options{DynamicgoConvOpts: convOpts}
	cli = initDynamicgoThriftClientByIDL(transport.TTHeader, t, "127.0.0.1:8126", "./idl/binary_echo.thrift", dOpts)
	resp, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	gr, ok = resp.(*generic.HTTPResponse)
	test.Assert(t, ok)
	test.Assert(t, gr.Body["msg"] == base64.StdEncoding.EncodeToString([]byte(mockMyMsg)))

	body = map[string]interface{}{
		"msg":        []byte(mockMyMsg),
		"got_base64": true,
		"num":        "123",
	}
	data, err = json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err = http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err = generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	dOpts = generic.Options{DynamicgoConvOpts: convOpts, EnableDynamicgoHTTPResp: true}
	cli = initDynamicgoThriftClientByIDL(transport.TTHeader, t, "127.0.0.1:8126", "./idl/binary_echo.thrift", dOpts)
	_, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err.Error() == "remote or network error[remote]: biz error: call failed, incorrect num", err.Error())

	svr.Stop()
}

func BenchmarkCompareKitexAndDynamicgo_Small(b *testing.B) {
	// small data
	sobj := jtest.GetSimpleValue()
	data, err := json.Marshal(sobj)
	if err != nil {
		panic(err)
	}
	fmt.Println("small data size: ", len(string(data)))
	url := "http://example.com/simple"

	t := testing.T{}

	b.Run("thrift_small", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initOriginalThriftServer(&t, ":8121", new(GenericServiceBenchmarkImpl), "../json_test/idl/baseline.thrift")
		time.Sleep(500 * time.Millisecond)
		cli := initThriftClientByIDL(&t, "127.0.0.1:8121", "../json_test/idl/baseline.thrift", false)

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
		if err != nil {
			panic(err)
		}
		customReq, err := generic.FromHTTPRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		gr, ok := resp.(*generic.HTTPResponse)
		test.Assert(&t, ok)
		test.Assert(&t, reflect.DeepEqual(gr.Body["I64Field"].(string), strconv.Itoa(math.MaxInt64)), gr.Body["I64Field"].(string))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})

	b.Run("dynaimcgoThrift_small", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initOriginalThriftServer(&t, ":8128", new(GenericServiceBenchmarkImpl), "../json_test/idl/baseline.thrift")
		time.Sleep(500 * time.Millisecond)
		dOpts := generic.Options{EnableBasicDynamicgoConvOpts: true, EnableDynamicgoHTTPResp: true}
		cli := initDynamicgoThriftClientByIDL(transport.TTHeader, &t, "127.0.0.1:8128", "../json_test/idl/baseline.thrift", dOpts)

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
		if err != nil {
			panic(err)
		}
		customReq, err := generic.FromHTTPRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		gr, ok := resp.(*generic.HTTPResponse)
		test.Assert(&t, ok)
		test.Assert(&t, reflect.DeepEqual(gjson.Get(string(gr.GeneralBody.([]byte)), "I64Field").String(), strconv.Itoa(math.MaxInt64)), gjson.Get(string(gr.GeneralBody.([]byte)), "I64Field").String())

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})
}

func BenchmarkCompareKitexAndDynamicgo_Medium(b *testing.B) {
	// medium data
	nobj := jtest.GetNestingValue()
	data, err := json.Marshal(nobj)
	if err != nil {
		panic(err)
	}
	fmt.Println("medium data size: ", len(string(data)))
	url := "http://example.com/nesting/100"

	t := testing.T{}

	b.Run("thrift_medium", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initOriginalThriftServer(&t, ":8121", new(GenericServiceBenchmarkImpl), "../json_test/idl/baseline.thrift")
		time.Sleep(500 * time.Millisecond)
		cli := initThriftClientByIDL(&t, "127.0.0.1:8121", "../json_test/idl/baseline.thrift", false)

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
		if err != nil {
			panic(err)
		}
		customReq, err := generic.FromHTTPRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		gr, ok := resp.(*generic.HTTPResponse)
		test.Assert(&t, ok)
		test.Assert(&t, gr.Body["I32"].(int32) == math.MaxInt32, gr.Body["I32"].(int32))

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})

	b.Run("dynaimcgoThrift_medium", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initOriginalThriftServer(&t, ":8128", new(GenericServiceBenchmarkImpl), "../json_test/idl/baseline.thrift")
		time.Sleep(500 * time.Millisecond)
		dOpts := generic.Options{EnableBasicDynamicgoConvOpts: true, EnableDynamicgoHTTPResp: true}
		cli := initDynamicgoThriftClientByIDL(transport.TTHeader, &t, "127.0.0.1:8128", "../json_test/idl/baseline.thrift", dOpts)

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
		if err != nil {
			panic(err)
		}
		customReq, err := generic.FromHTTPRequest(req)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		gr, ok := resp.(*generic.HTTPResponse)
		test.Assert(&t, ok)
		test.Assert(&t, reflect.DeepEqual(gjson.Get(string(gr.GeneralBody.([]byte)), "I32").String(), strconv.Itoa(math.MaxInt32)), gjson.Get(string(gr.GeneralBody.([]byte)), "I32").String())

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})
}

func testRegression(t *testing.T) {
	nobj := jtest.GetNestingValue()
	data, err := json.Marshal(nobj)
	if err != nil {
		panic(err)
	}
	url := "http://example.com/nesting/100"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err := generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	svr := initOriginalThriftServer(t, ":8121", new(GenericServiceBenchmarkImpl), "../json_test/idl/baseline.thrift")
	time.Sleep(500 * time.Millisecond)

	// dynamicgo
	dOpts := generic.Options{EnableBasicDynamicgoConvOpts: true, EnableDynamicgoHTTPResp: true}
	cli := initDynamicgoThriftClientByIDL(transport.TTHeader, t, "127.0.0.1:8121", "../json_test/idl/baseline.thrift", dOpts)
	resp, err := cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil)
	dgr, ok := resp.(*generic.HTTPResponse)
	test.Assert(t, ok)

	// fallback: only resp
	dOpts = generic.Options{EnableBasicDynamicgoConvOpts: true}
	cli = initDynamicgoThriftClientByIDL(transport.TTHeader, t, "127.0.0.1:8121", "../json_test/idl/baseline.thrift", dOpts)
	resp, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil)
	fgr, ok := resp.(*generic.HTTPResponse)
	test.Assert(t, ok)

	test.Assert(t, reflect.DeepEqual(dgr.Header, fgr.Header))
	test.Assert(t, reflect.DeepEqual(dgr.StatusCode, fgr.StatusCode))
	test.Assert(t, reflect.DeepEqual(dgr.ContentType, fgr.ContentType))

	var dMapBody map[string]interface{}
	customJson := sonic.Config{
		EscapeHTML: true,
		UseNumber:  true,
	}.Froze()
	err = customJson.Unmarshal(dgr.GeneralBody.([]byte), &dMapBody)
	test.Assert(t, err == nil)
	fBytes, err := customJson.Marshal(fgr.Body)
	test.Assert(t, err == nil)
	err = customJson.Unmarshal(fBytes, &fgr.Body)
	test.Assert(t, err == nil)
	test.Assert(t, isEqual(dMapBody, fgr.Body))

	// fallback: both req and resp
	cli = initThriftClientByIDL(t, "127.0.0.1:8121", "../json_test/idl/baseline.thrift", false)
	resp, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil)
	fgr, ok = resp.(*generic.HTTPResponse)
	test.Assert(t, ok)

	test.Assert(t, reflect.DeepEqual(dgr.Header, fgr.Header))
	test.Assert(t, reflect.DeepEqual(dgr.StatusCode, fgr.StatusCode))
	test.Assert(t, reflect.DeepEqual(dgr.ContentType, fgr.ContentType))

	err = customJson.Unmarshal(dgr.GeneralBody.([]byte), &dMapBody)
	test.Assert(t, err == nil)
	fBytes, err = customJson.Marshal(fgr.Body)
	test.Assert(t, err == nil)
	err = customJson.Unmarshal(fBytes, &fgr.Body)
	test.Assert(t, err == nil)
	test.Assert(t, isEqual(dMapBody, fgr.Body))

	svr.Stop()
}

func testThriftBase64BinaryEcho(t *testing.T) {
	svr := initThriftServer(t, ":8126", new(GenericServiceBinaryEchoImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:8126", "./idl/binary_echo.thrift", true)
	url := "http://example.com/BinaryEcho"

	// []byte value for binary field
	body := map[string]interface{}{
		"msg":        []byte(mockMyMsg),
		"got_base64": false,
		"num":        "0",
	}
	data, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err := generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	gr, ok := resp.(*generic.HTTPResponse)
	test.Assert(t, ok)
	test.Assert(t, gr.Body["msg"] == base64.StdEncoding.EncodeToString(body["msg"].([]byte)))

	// string value for binary field which should fail
	body = map[string]interface{}{
		"msg": string(mockMyMsg),
	}
	data, err = json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err = http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err = generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, strings.Contains(err.Error(), "illegal base64 data"))

	svr.Stop()
}

func testDynamicgoThriftBase64BinaryEcho(t *testing.T) {
	svr := initThriftServer(t, ":8126", new(GenericServiceBinaryEchoImpl))
	time.Sleep(500 * time.Millisecond)

	convOpts := conv.Options{EnableValueMapping: true, NoBase64Binary: false}
	dOpts := generic.Options{DynamicgoConvOpts: convOpts, EnableDynamicgoHTTPResp: true}

	cli := initDynamicgoThriftClientByIDL(transport.TTHeader, t, "127.0.0.1:8126", "./idl/binary_echo.thrift", dOpts)
	url := "http://example.com/BinaryEcho"

	// []byte value for binary field
	body := map[string]interface{}{
		"msg":        []byte(mockMyMsg),
		"got_base64": false,
		"num":        "0",
	}
	data, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err := generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	// write: dynamicgo (amd64), fallback (arm)
	// read: dynamicgo
	resp, err := cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	gr, ok := resp.(*generic.HTTPResponse)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(string(gr.GeneralBody.([]byte)), "msg").String(), base64.StdEncoding.EncodeToString([]byte(mockMyMsg))), gjson.Get(string(gr.GeneralBody.([]byte)), "msg").String())

	// string value for binary field which should fail
	body = map[string]interface{}{
		"msg": string(mockMyMsg),
	}
	data, err = json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req, err = http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	customReq, err = generic.FromHTTPRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cli.GenericCall(context.Background(), "", customReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, strings.Contains(err.Error(), "illegal base64 data"))

	svr.Stop()
}

func isEqual(a, b interface{}) bool {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false
	}
	switch a := a.(type) {
	case []interface{}:
		b, ok := b.([]interface{})
		if !ok {
			return false
		}
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if !isEqual(a[i], b[i]) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		b, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		if len(a) != len(b) {
			return false
		}
		for k, v1 := range a {
			v2, ok := b[k]
			if !ok || !isEqual(v1, v2) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}
