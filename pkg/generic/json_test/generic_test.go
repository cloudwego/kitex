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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/dynamicgo/conv"
	"github.com/tidwall/gjson"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/genericclient"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/transport"
)

func TestRun(t *testing.T) {
	t.Run("TestThrift", testThrift)
	t.Run("TestThriftWithDynamicGo", testThriftWithDynamicGo)
	t.Run("TestThriftPingMethod", testThriftPingMethod)
	t.Run("TestThriftPingMethodWithDynamicGo", testThriftPingMethodWithDynamicGo)
	t.Run("TestThriftError", testThriftError)
	t.Run("TestThriftOnewayMethod", testThriftOnewayMethod)
	t.Run("TestThriftOnewayMethodWithDynamicGo", testThriftOnewayMethodWithDynamicGo)
	t.Run("TestThriftVoidMethod", testThriftVoidMethod)
	t.Run("TestThriftVoidMethodWithDynamicGo", testThriftVoidMethodWithDynamicGo)
	t.Run("TestThrift2NormalServer", testThrift2NormalServer)
	t.Run("TestThriftException", testThriftException)
	t.Run("TestJSONThriftGenericClientClose", testJSONThriftGenericClientClose)
	t.Run("TestThriftRawBinaryEcho", testThriftRawBinaryEcho)
	t.Run("TestThriftBase64BinaryEcho", testThriftBase64BinaryEcho)
	t.Run("TestRegression", testRegression)
	t.Run("TestJSONThriftGenericClientFinalizer", testJSONThriftGenericClientFinalizer)
}

func testThrift(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8122", new(GenericServiceImpl), "./idl/example.thrift", nil, nil, false)
	time.Sleep(500 * time.Millisecond)

	// normal way
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8122", "./idl/example.thrift", nil, nil, false)
	resp, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(respStr, "Msg").String(), "world"), "world")

	// extend method
	resp, err = cli.GenericCall(context.Background(), "ExtendMethod", reqExtendMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok = resp.(string)
	test.Assert(t, ok)
	test.Assert(t, respStr == reqExtendMsg)

	svr.Stop()
}

func testThriftWithDynamicGo(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8123", new(GenericServiceImpl), "./idl/example.thrift", nil, nil, true)
	time.Sleep(500 * time.Millisecond)

	// write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	// read: dynamicgo
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8123", "./idl/example.thrift", nil, nil, true)
	resp, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(respStr, "Msg").String(), "world"), "world")

	// client without dynamicgo

	// server side:
	//  write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	//  read: dynamicgo
	cli = initThriftClient(transport.TTHeader, t, "127.0.0.1:8123", "./idl/example.thrift", nil, nil, false)
	resp, err = cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok = resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(respStr, "Msg").String(), "world"), "world")

	// server side:
	//  write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	//  read: fallback
	cli = initThriftClient(transport.PurePayload, t, "127.0.0.1:8123", "./idl/example.thrift", nil, nil, false)
	resp, err = cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok = resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(respStr, "Msg").String(), "world"), "world")

	svr.Stop()
}

func BenchmarkCompareDynamicGoAndOriginal_Small(b *testing.B) {
	// small data
	sobj := getSimpleValue()
	sout, err := json.Marshal(sobj)
	if err != nil {
		panic(err)
	}
	simpleJSON := string(sout)
	fmt.Println("small data size: ", len(simpleJSON))

	t := testing.T{}

	b.Run("thrift_small_dynamicgo", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initThriftServer(&t, ":8124", new(GenericServiceBenchmarkImpl), "./idl/baseline.thrift", nil, nil, true)
		time.Sleep(500 * time.Millisecond)
		cli := initThriftClient(transport.TTHeader, &t, "127.0.0.1:8124", "./idl/baseline.thrift", nil, nil, true)

		resp, err := cli.GenericCall(context.Background(), "SimpleMethod", simpleJSON, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		respStr, ok := resp.(string)
		test.Assert(&t, ok)
		test.Assert(&t, gjson.Get(respStr, "I64Field").Int() == math.MaxInt64, math.MaxInt64)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "SimpleMethod", simpleJSON, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})

	b.Run("thrift_small_original", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initThriftServer(&t, ":8125", new(GenericServiceBenchmarkImpl), "./idl/baseline.thrift", nil, nil, false)
		time.Sleep(500 * time.Millisecond)
		cli := initThriftClient(transport.TTHeader, &t, "127.0.0.1:8125", "./idl/baseline.thrift", nil, nil, false)

		resp, err := cli.GenericCall(context.Background(), "SimpleMethod", simpleJSON, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		respStr, ok := resp.(string)
		test.Assert(&t, ok)
		test.Assert(&t, gjson.Get(respStr, "I64Field").Int() == math.MaxInt64, math.MaxInt64)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "SimpleMethod", simpleJSON, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})
}

func BenchmarkCompareDynamicGoAndOriginal_Medium(b *testing.B) {
	// medium data
	nobj := getNestingValue()
	nout, err := json.Marshal(nobj)
	if err != nil {
		panic(err)
	}
	nestingJSON := string(nout)
	fmt.Println("medium data size: ", len(nestingJSON))

	t := testing.T{}

	b.Run("thrift_medium_dynamicgo", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initThriftServer(&t, ":8126", new(GenericServiceBenchmarkImpl), "./idl/baseline.thrift", nil, nil, true)
		time.Sleep(500 * time.Millisecond)
		cli := initThriftClient(transport.TTHeader, &t, "127.0.0.1:8126", "./idl/baseline.thrift", nil, nil, true)

		resp, err := cli.GenericCall(context.Background(), "NestingMethod", nestingJSON, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		respStr, ok := resp.(string)
		test.Assert(&t, ok)
		test.Assert(&t, gjson.Get(respStr, "Double").Float() == math.MaxFloat64, math.MaxFloat64)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "NestingMethod", nestingJSON, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})

	b.Run("thrift_medium_original", func(b *testing.B) {
		time.Sleep(1 * time.Second)
		svr := initThriftServer(&t, ":8127", new(GenericServiceBenchmarkImpl), "./idl/baseline.thrift", nil, nil, false)
		time.Sleep(500 * time.Millisecond)
		cli := initThriftClient(transport.TTHeader, &t, "127.0.0.1:8127", "./idl/baseline.thrift", nil, nil, false)

		resp, err := cli.GenericCall(context.Background(), "NestingMethod", nestingJSON, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(&t, err == nil, err)
		respStr, ok := resp.(string)
		test.Assert(&t, ok)
		test.Assert(&t, gjson.Get(respStr, "Double").Float() == math.MaxFloat64, math.MaxFloat64)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cli.GenericCall(context.Background(), "NestingMethod", nestingJSON, callopt.WithRPCTimeout(100*time.Second))
		}
		svr.Stop()
	})
}

func testThriftPingMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8128", new(GenericServicePingImpl), "./idl/example.thrift", nil, nil, false)
	time.Sleep(500 * time.Millisecond)

	// normal way
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8128", "./idl/example.thrift", nil, nil, false)
	resp, err := cli.GenericCall(context.Background(), "Ping", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respMap, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(respMap, "hello"), respMap)

	svr.Stop()
}

func testThriftPingMethodWithDynamicGo(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8129", new(GenericServicePingImpl), "./idl/example.thrift", nil, nil, true)
	time.Sleep(500 * time.Millisecond)

	// write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	// read: dynamicgo
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8129", "./idl/example.thrift", nil, nil, true)
	resp, err := cli.GenericCall(context.Background(), "Ping", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respMap, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(respMap, "hello"), respMap)

	// client without dynamicgo

	// server side:
	//  write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	//  read: dynamicgo
	cli = initThriftClient(transport.TTHeader, t, "127.0.0.1:8129", "./idl/example.thrift", nil, nil, false)
	resp, err = cli.GenericCall(context.Background(), "Ping", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respMap, ok = resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(respMap, "hello"), respMap)

	// server side:
	//  write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	//  read: fallback
	cli = initThriftClient(transport.PurePayload, t, "127.0.0.1:8129", "./idl/example.thrift", nil, nil, false)
	resp, err = cli.GenericCall(context.Background(), "Ping", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respMap, ok = resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(respMap, "hello"), respMap)

	svr.Stop()
}

func testThriftError(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8130", new(GenericServiceErrorImpl), "./idl/example.thrift", nil, nil, false)
	time.Sleep(500 * time.Millisecond)

	// normal way
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8130", "./idl/example.thrift", nil, nil, false)
	_, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errResp), err.Error())

	// with dynamicgo
	cli = initThriftClient(transport.TTHeader, t, "127.0.0.1:8130", "./idl/example.thrift", nil, nil, true)
	_, err = cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errResp), err.Error())

	svr.Stop()

	// server with dynamicgo
	time.Sleep(1 * time.Second)
	svr = initThriftServer(t, ":8131", new(GenericServiceErrorImpl), "./idl/example.thrift", nil, nil, true)
	time.Sleep(500 * time.Millisecond)

	// normal way
	cli = initThriftClient(transport.TTHeader, t, "127.0.0.1:8131", "./idl/example.thrift", nil, nil, false)
	_, err = cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errResp), err.Error())

	// with dynamicgo
	cli = initThriftClient(transport.TTHeader, t, "127.0.0.1:8131", "./idl/example.thrift", nil, nil, true)
	_, err = cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errResp), err.Error())

	svr.Stop()
}

func testThriftOnewayMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8132", new(GenericServiceOnewayImpl), "./idl/example.thrift", nil, nil, false)
	time.Sleep(500 * time.Millisecond)

	// normal way
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8132", "./idl/example.thrift", nil, nil, false)
	resp, err := cli.GenericCall(context.Background(), "Oneway", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == nil)

	svr.Stop()
}

func testThriftOnewayMethodWithDynamicGo(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8133", new(GenericServiceOnewayImpl), "./idl/example.thrift", nil, nil, true)
	time.Sleep(500 * time.Millisecond)

	// write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	// read: dynamicgo
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8133", "./idl/example.thrift", nil, nil, true)
	resp, err := cli.GenericCall(context.Background(), "Oneway", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == nil)

	// client without dynamicgo

	// server side:
	//  write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	//  read: dynamicgo
	cli = initThriftClient(transport.TTHeader, t, "127.0.0.1:8133", "./idl/example.thrift", nil, nil, false)
	resp, err = cli.GenericCall(context.Background(), "Oneway", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == nil)

	// server side:
	//  write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	//  read: fallback
	cli = initThriftClient(transport.PurePayload, t, "127.0.0.1:8133", "./idl/example.thrift", nil, nil, false)
	resp, err = cli.GenericCall(context.Background(), "Oneway", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == nil)

	svr.Stop()
}

func testThriftVoidMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8134", new(GenericServiceVoidImpl), "./idl/example.thrift", nil, nil, false)
	time.Sleep(500 * time.Millisecond)

	// normal way
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8134", "./idl/example.thrift", nil, nil, false)
	resp, err := cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})

	svr.Stop()

	time.Sleep(1 * time.Second)
	svr = initThriftServer(t, ":8134", new(GenericServiceVoidWithStringImpl), "./idl/example.thrift", nil, nil, false)
	time.Sleep(500 * time.Millisecond)
	resp, err = cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})

	svr.Stop()
}

func testThriftVoidMethodWithDynamicGo(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8135", new(GenericServiceVoidImpl), "./idl/example.thrift", nil, nil, true)
	time.Sleep(500 * time.Millisecond)

	// write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	// read: dynamicgo
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8135", "./idl/example.thrift", nil, nil, true)
	resp, err := cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})

	// client without dynamicgo

	// server side:
	//  write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	//  read: dynamicgo
	cli = initThriftClient(transport.TTHeader, t, "127.0.0.1:8135", "./idl/example.thrift", nil, nil, false)
	resp, err = cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})

	// server side:
	//  write: dynamicgo (amd64 && go1.16), fallback (arm || !go1.16)
	//  read: fallback
	cli = initThriftClient(transport.PurePayload, t, "127.0.0.1:8135", "./idl/example.thrift", nil, nil, false)
	resp, err = cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})

	svr.Stop()

	time.Sleep(1 * time.Second)
	svr = initThriftServer(t, ":8135", new(GenericServiceVoidWithStringImpl), "./idl/example.thrift", nil, nil, true)
	time.Sleep(500 * time.Millisecond)
	resp, err = cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})

	svr.Stop()
}

func testThrift2NormalServer(t *testing.T) {
	time.Sleep(4 * time.Second)
	svr := initMockServer(t, new(mockImpl))
	time.Sleep(500 * time.Millisecond)

	// client without dynamicgo
	cli := initThriftMockClient(t, transport.TTHeader, false)
	_, err := cli.GenericCall(context.Background(), "Test", mockReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)

	// client with dynamicgo
	cli = initThriftMockClient(t, transport.TTHeader, true)
	_, err = cli.GenericCall(context.Background(), "Test", mockReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)

	svr.Stop()
}

func testThriftException(t *testing.T) {
	time.Sleep(4 * time.Second)
	svr := initMockServer(t, new(mockImpl))
	time.Sleep(500 * time.Millisecond)

	// client without dynamicgo
	cli := initThriftMockClient(t, transport.TTHeader, false)

	_, err := cli.GenericCall(context.Background(), "ExceptionTest", mockReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil, err)
	test.DeepEqual(t, err.Error(), `remote or network error[remote]: map[string]interface {}{"code":400, "msg":"this is an exception"}`)

	// client with dynaimcgo
	cli = initThriftMockClient(t, transport.TTHeader, true)

	_, err = cli.GenericCall(context.Background(), "ExceptionTest", mockReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil, err)
	test.DeepEqual(t, err.Error(), `remote or network error[remote]: {"code":400,"msg":"this is an exception"}`)

	svr.Stop()
}

func testThriftRawBinaryEcho(t *testing.T) {
	var opts []generic.Option
	opts = append(opts, generic.WithCustomDynamicGoConvOpts(&conv.Options{WriteRequireField: true, WriteDefaultField: true, NoBase64Binary: true}))
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8136", new(GenericServiceBinaryEchoImpl), "./idl/binary_echo.thrift", opts, &(&struct{ x bool }{false}).x, false)
	time.Sleep(500 * time.Millisecond)

	// client without dynamicgo
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8136", "./idl/binary_echo.thrift", opts, &(&struct{ x bool }{false}).x, false)

	req := "{\"msg\":\"" + mockMyMsg + "\", \"got_base64\":false}"
	resp, err := cli.GenericCall(context.Background(), "BinaryEcho", req, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	sr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, strings.Contains(sr, mockMyMsg))

	// client with dynamicgo
	cli = initThriftClient(transport.PurePayload, t, "127.0.0.1:8136", "./idl/binary_echo.thrift", opts, &(&struct{ x bool }{false}).x, true)

	req = "{\"msg\":\"" + mockMyMsg + "\", \"got_base64\":false}"
	resp, err = cli.GenericCall(context.Background(), "BinaryEcho", req, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	sr, ok = resp.(string)
	test.Assert(t, ok)
	test.Assert(t, strings.Contains(sr, mockMyMsg))

	svr.Stop()
}

func testThriftBase64BinaryEcho(t *testing.T) {
	var opts []generic.Option
	opts = append(opts, generic.WithCustomDynamicGoConvOpts(&conv.Options{WriteRequireField: true, WriteDefaultField: true, NoBase64Binary: false}))
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8137", new(GenericServiceBinaryEchoImpl), "./idl/binary_echo.thrift", opts, nil, false)
	time.Sleep(500 * time.Millisecond)

	// client without dynamicgo
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8137", "./idl/binary_echo.thrift", opts, nil, false)

	base64MockMyMsg := base64.StdEncoding.EncodeToString([]byte(mockMyMsg))
	req := "{\"msg\":\"" + base64MockMyMsg + "\", \"got_base64\":true}"
	resp, err := cli.GenericCall(context.Background(), "BinaryEcho", req, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	sr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, strings.Contains(sr, base64MockMyMsg))

	// client with dynamicgo
	cli = initThriftClient(transport.PurePayload, t, "127.0.0.1:8137", "./idl/binary_echo.thrift", opts, nil, true)

	base64MockMyMsg = base64.StdEncoding.EncodeToString([]byte(mockMyMsg))
	req = "{\"msg\":\"" + base64MockMyMsg + "\", \"got_base64\":true}"
	resp, err = cli.GenericCall(context.Background(), "BinaryEcho", req, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	sr, ok = resp.(string)
	test.Assert(t, ok)
	test.Assert(t, strings.Contains(sr, base64MockMyMsg))

	svr.Stop()
}

func testRegression(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8138", new(GenericRegressionImpl), "./idl/example.thrift", nil, nil, false)
	time.Sleep(500 * time.Millisecond)

	// normal way
	cli := initThriftClient(transport.TTHeader, t, "127.0.0.1:8138", "./idl/example.thrift", nil, nil, false)
	resp, err := cli.GenericCall(context.Background(), "ExampleMethod", reqRegression, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, gjson.Get(respStr, "num").Type == gjson.String)
	test.Assert(t, gjson.Get(respStr, "num").String() == "64")
	test.Assert(t, gjson.Get(respStr, "I8").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "I8").Int() == int64(8))
	test.Assert(t, gjson.Get(respStr, "I16").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "I32").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "I64").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "Double").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "Double").Float() == 12.3)

	svr.Stop()

	time.Sleep(1 * time.Second)
	svr = initThriftServer(t, ":8139", new(GenericRegressionImpl), "./idl/example.thrift", nil, nil, true)

	// dynamicgo way
	cli = initThriftClient(transport.TTHeader, t, "127.0.0.1:8139", "./idl/example.thrift", nil, nil, true)
	resp, err = cli.GenericCall(context.Background(), "ExampleMethod", reqRegression, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok = resp.(string)
	test.Assert(t, ok)
	test.Assert(t, gjson.Get(respStr, "num").Type == gjson.String)
	test.Assert(t, gjson.Get(respStr, "num").String() == "64")
	test.Assert(t, gjson.Get(respStr, "I8").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "I8").Int() == int64(8))
	test.Assert(t, gjson.Get(respStr, "I16").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "I32").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "I64").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "Double").Type == gjson.Number)
	test.Assert(t, gjson.Get(respStr, "Double").Float() == 12.3)

	svr.Stop()
}

func initThriftMockClient(t *testing.T, tp transport.Protocol, enableDynamicGo bool) genericclient.Client {
	var p generic.DescriptorProvider
	var err error
	if enableDynamicGo {
		p, err = generic.NewThriftFileProviderWithDynamicGo("./idl/mock.thrift")
	} else {
		p, err = generic.NewThriftFileProvider("./idl/mock.thrift")
	}
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	cli := newGenericClient(tp, "destServiceName", g, "127.0.0.1:8128")
	test.Assert(t, err == nil)
	return cli
}

func initThriftClient(tp transport.Protocol, t *testing.T, addr, idl string, opts []generic.Option, base64Binary *bool, enableDynamicGo bool) genericclient.Client {
	var p generic.DescriptorProvider
	var err error
	if enableDynamicGo {
		p, err = generic.NewThriftFileProviderWithDynamicGo(idl)
	} else {
		p, err = generic.NewThriftFileProvider(idl)
	}
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p, opts...)
	test.Assert(t, err == nil)
	if base64Binary != nil {
		generic.SetBinaryWithBase64(g, *base64Binary)
	}
	cli := newGenericClient(tp, "destServiceName", g, addr)
	test.Assert(t, err == nil)
	return cli
}

func initThriftServer(t *testing.T, address string, handler generic.Service, idlPath string, opts []generic.Option, base64Binary *bool, enableDynamicGo bool) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	var p generic.DescriptorProvider
	var err error
	if enableDynamicGo {
		p, err = generic.NewThriftFileProviderWithDynamicGo(idlPath)
	} else {
		p, err = generic.NewThriftFileProvider(idlPath)
	}
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p, opts...)
	test.Assert(t, err == nil)
	if base64Binary != nil {
		generic.SetBinaryWithBase64(g, *base64Binary)
	}
	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func initMockServer(t *testing.T, handler kt.Mock) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", ":8128")
	svr := newMockServer(handler, addr)
	return svr
}

func testJSONThriftGenericClientClose(t *testing.T) {
	debug.SetGCPercent(-1)
	defer debug.SetGCPercent(100)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	t.Logf("Before new clients, allocation: %f Mb, Number of allocation: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	clientCnt := 1000
	clis := make([]genericclient.Client, clientCnt)
	for i := 0; i < clientCnt; i++ {
		p, err := generic.NewThriftFileProvider("./idl/mock.thrift")
		test.Assertf(t, err == nil, "generic NewThriftFileProvider failed, err=%v", err)
		g, err := generic.JSONThriftGeneric(p)
		test.Assertf(t, err == nil, "generic JSONThriftGeneric failed, err=%v", err)
		clis[i] = newGenericClient(transport.TTHeader, "destServiceName", g, "127.0.0.1:8129")
	}

	runtime.ReadMemStats(&ms)
	preHeapAlloc, preHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("After new clients, allocation: %f Mb, Number of allocation: %d\n", preHeapAlloc, preHeapObjects)

	for _, cli := range clis {
		_ = cli.Close()
	}
	runtime.GC()
	runtime.ReadMemStats(&ms)
	afterGCHeapAlloc, afterGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("After close clients and GC be executed, allocation: %f Mb, Number of allocation: %d\n", afterGCHeapAlloc, afterGCHeapObjects)
	test.Assert(t, afterGCHeapAlloc < preHeapAlloc && afterGCHeapObjects < preHeapObjects)

	// Trigger the finalizer of kclient be executed
	time.Sleep(200 * time.Millisecond) // ensure the finalizer be executed
	runtime.GC()
	runtime.ReadMemStats(&ms)
	secondGCHeapAlloc, secondGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("After second GC, allocation: %f Mb, Number of allocation: %d\n", secondGCHeapAlloc, secondGCHeapObjects)
	test.Assert(t, secondGCHeapAlloc/2 < afterGCHeapAlloc && secondGCHeapObjects/2 < afterGCHeapObjects)
}

func testJSONThriftGenericClientFinalizer(t *testing.T) {
	debug.SetGCPercent(-1)
	defer debug.SetGCPercent(100)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("Before new clients, allocation: %f Mb, Number of allocation: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	clientCnt := 1000
	clis := make([]genericclient.Client, clientCnt)
	for i := 0; i < clientCnt; i++ {
		p, err := generic.NewThriftFileProvider("./idl/mock.thrift")
		test.Assert(t, err == nil, "generic NewThriftFileProvider failed, err=%v", err)
		g, err := generic.JSONThriftGeneric(p)
		test.Assert(t, err == nil, "generic JSONThriftGeneric failed, err=%v", err)
		clis[i] = newGenericClient(transport.TTHeader, "destServiceName", g, "127.0.0.1:8130")
	}

	runtime.ReadMemStats(&ms)
	t.Logf("After new clients, allocation: %f Mb, Number of allocation: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	runtime.GC()
	runtime.ReadMemStats(&ms)
	firstGCHeapAlloc, firstGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("After first GC, allocation: %f Mb, Number of allocation: %d\n", firstGCHeapAlloc, firstGCHeapObjects)

	// Trigger the finalizer of generic client be executed
	time.Sleep(200 * time.Millisecond) // ensure the finalizer be executed
	runtime.GC()
	runtime.ReadMemStats(&ms)
	secondGCHeapAlloc, secondGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("After second GC, allocation: %f Mb, Number of allocation: %d\n", secondGCHeapAlloc, secondGCHeapObjects)
	test.Assert(t, secondGCHeapAlloc < firstGCHeapAlloc && secondGCHeapObjects < firstGCHeapObjects)

	// Trigger the finalizer of kClient be executed
	time.Sleep(200 * time.Millisecond) // ensure the finalizer be executed
	runtime.GC()
	runtime.ReadMemStats(&ms)
	thirddGCHeapAlloc, thirdGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("After third GC, allocation: %f Mb, Number of allocation: %d\n", thirddGCHeapAlloc, thirdGCHeapObjects)
	test.Assert(t, thirddGCHeapAlloc < secondGCHeapAlloc/2 && thirdGCHeapObjects < secondGCHeapObjects/2)
}

func mb(byteSize uint64) float32 {
	return float32(byteSize) / float32(1024*1024)
}
