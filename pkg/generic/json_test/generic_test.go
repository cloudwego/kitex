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
	"net"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/tidwall/gjson"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/genericclient"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/server"
)

func TestRun(t *testing.T) {
	t.Run("TestThrift", testThrift)
	t.Run("TestThriftPingMethod", testThriftPingMethod)
	t.Run("TestThriftError", testThriftError)
	t.Run("TestThriftOnewayMethod", testThriftOnewayMethod)
	t.Run("TestThriftVoidMethod", testThriftVoidMethod)
	t.Run("TestThriftReadRequiredField", testThriftReadRequiredField)
	t.Run("TestThriftWriteRequiredField", testThriftWriteRequiredField)
	t.Run("TestThrift2NormalServer", testThrift2NormalServer)
	t.Run("TestJSONThriftGenericClientClose", testJSONThriftGenericClientClose)
	t.Run("TestThriftRawBinaryEcho", testThriftRawBinaryEcho)
	t.Run("TestThriftBase64BinaryEcho", testThriftBase64BinaryEcho)
	t.Run("TestJSONThriftGenericClientFinalizer", testJSONThriftGenericClientFinalizer)
}

func testThrift(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9121", new(GenericServiceImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9121")

	resp, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(respStr, "Msg").String(), "world"), "world")
	svr.Stop()
}

func testThriftPingMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9122", new(GenericServicePingImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9122")

	resp, err := cli.GenericCall(context.Background(), "Ping", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respMap, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(respMap, "hello"), respMap)
	svr.Stop()
}

func testThriftError(t *testing.T) {
	time.Sleep(2 * time.Second)
	svr := initThriftServer(t, ":9123", new(GenericServiceErrorImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9123")

	_, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errResp), err.Error())
	svr.Stop()
}

func testThriftOnewayMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9124", new(GenericServiceOnewayImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9124")

	resp, err := cli.GenericCall(context.Background(), "Oneway", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == nil)
	svr.Stop()
}

func testThriftVoidMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9125", new(GenericServiceVoidImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9125")

	resp, err := cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})
	svr.Stop()
}

func testThriftReadRequiredField(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9126", new(GenericServiceReadRequiredFiledImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:9126", "./idl/example_check_read_required.thrift", nil)

	_, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil, err)
	test.Assert(t, strings.Contains(err.Error(), "required field (2/required_field) missing"), err.Error())
	svr.Stop()
}

func testThriftWriteRequiredField(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9127", new(GenericServiceReadRequiredFiledImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:9127", "./idl/example_check_write_required.thrift", nil)

	_, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil, err)
	test.Assert(t, strings.Contains(err.Error(), "required field (2/Foo) missing"), err.Error())
	svr.Stop()
}

func testThrift2NormalServer(t *testing.T) {
	time.Sleep(4 * time.Second)
	svr := initMockServer(t, new(mockImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftMockClient(t)

	_, err := cli.GenericCall(context.Background(), "Test", mockReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	svr.Stop()
}

func testThriftRawBinaryEcho(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServerByIDL(t, ":9126", new(GenericServiceBinaryEchoImpl), "./idl/binary_echo.thrift", &(&struct{ x bool }{false}).x)
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:9126", "./idl/binary_echo.thrift", &(&struct{ x bool }{false}).x)

	req := "{\"msg\":\"" + mockMyMsg + "\", \"got_base64\":\"false\"}"
	resp, err := cli.GenericCall(context.Background(), "BinaryEcho", req, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	sr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, strings.Contains(sr, mockMyMsg))

	svr.Stop()
}

func testThriftBase64BinaryEcho(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServerByIDL(t, ":9126", new(GenericServiceBinaryEchoImpl), "./idl/binary_echo.thrift", nil)
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:9126", "./idl/binary_echo.thrift", nil)

	base64MockMyMsg := base64.StdEncoding.EncodeToString([]byte(mockMyMsg))
	req := "{\"msg\":\"" + base64MockMyMsg + "\", \"got_base64\":\"true\"}"
	resp, err := cli.GenericCall(context.Background(), "BinaryEcho", req, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	sr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, strings.Contains(sr, base64MockMyMsg))

	svr.Stop()
}

func initThriftMockClient(t *testing.T) genericclient.Client {
	p, err := generic.NewThriftFileProvider("./idl/mock.thrift")
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	cli := newGenericClient("destServiceName", g, "127.0.0.1:9128")
	test.Assert(t, err == nil)
	return cli
}

func initThriftClient(t *testing.T, addr string) genericclient.Client {
	return initThriftClientByIDL(t, addr, "./idl/example.thrift", nil)
}

func initThriftClientByIDL(t *testing.T, addr, idl string, base64binary *bool) genericclient.Client {
	p, err := generic.NewThriftFileProvider(idl)
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	if base64binary != nil {
		generic.SetBinaryWithBase64(g, *base64binary)
	}
	cli := newGenericClient("destServiceName", g, addr)
	test.Assert(t, err == nil)
	return cli
}

func initThriftServer(t *testing.T, address string, handler generic.Service) server.Server {
	return initThriftServerByIDL(t, address, handler, "./idl/example.thrift", nil)
}

func initThriftServerByIDL(t *testing.T, address string, handler generic.Service, idlPath string, base64Binary *bool) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewThriftFileProvider(idlPath)
	test.Assert(t, err == nil)
	g, err := generic.JSONThriftGeneric(p)
	test.Assert(t, err == nil)
	if base64Binary != nil {
		generic.SetBinaryWithBase64(g, *base64Binary)
	}
	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func initMockServer(t *testing.T, handler kt.Mock) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", ":9128")
	svr := newMockServer(handler, addr)
	return svr
}

func testJSONThriftGenericClientClose(t *testing.T) {
	debug.SetGCPercent(-1)
	defer debug.SetGCPercent(100)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	clis := make([]genericclient.Client, 1000)
	for i := 0; i < 1000; i++ {
		p, err := generic.NewThriftFileProvider("./idl/mock.thrift")
		test.Assert(t, err == nil)
		g, err := generic.JSONThriftGeneric(p)
		test.Assert(t, err == nil)
		clis[i] = newGenericClient("destServiceName", g, "127.0.0.1:9129")
	}

	runtime.ReadMemStats(&ms)
	preHeepAlloc, preHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", preHeepAlloc, preHeapObjects)

	for _, cli := range clis {
		_ = cli.Close()
	}
	runtime.GC()
	runtime.ReadMemStats(&ms)
	aferGCHeepAlloc, afterGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", aferGCHeepAlloc, afterGCHeapObjects)
	test.Assert(t, aferGCHeepAlloc < preHeepAlloc && afterGCHeapObjects < preHeapObjects)
}

func testJSONThriftGenericClientFinalizer(t *testing.T) {
	debug.SetGCPercent(-1)
	defer debug.SetGCPercent(100)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	clis := make([]genericclient.Client, 1000)
	for i := 0; i < 1000; i++ {
		p, err := generic.NewThriftFileProvider("./idl/mock.thrift")
		test.Assert(t, err == nil)
		g, err := generic.JSONThriftGeneric(p)
		test.Assert(t, err == nil)
		clis[i] = newGenericClient("destServiceName", g, "127.0.0.1:9130")
	}

	runtime.ReadMemStats(&ms)
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", mb(ms.HeapAlloc), ms.HeapObjects)

	runtime.GC()
	runtime.ReadMemStats(&ms)
	firstGCHeepAlloc, firstGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", firstGCHeepAlloc, firstGCHeapObjects)

	runtime.GC()
	runtime.ReadMemStats(&ms)
	secondGCHeepAlloc, secondGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("Allocation: %f Mb, Number of allocation: %d\n", secondGCHeepAlloc, secondGCHeapObjects)
	test.Assert(t, secondGCHeepAlloc < firstGCHeepAlloc && secondGCHeapObjects < firstGCHeapObjects)
}

func mb(byteSize uint64) float32 {
	return float32(byteSize) / float32(1024*1024)
}
