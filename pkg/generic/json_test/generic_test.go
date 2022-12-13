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
	t.Run("TestThrift2NormalServer", testThrift2NormalServer)
	t.Run("TestJSONThriftGenericClientClose", TestJSONThriftGenericClientClose)
	t.Run("TestThriftRawBinaryEcho", testThriftRawBinaryEcho)
	t.Run("TestThriftBase64BinaryEcho", testThriftBase64BinaryEcho)
	t.Run("TestJSONThriftGenericClientFinalizer", TestJSONThriftGenericClientFinalizer)
}

func testThrift(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8121", new(GenericServiceImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:8121")

	resp, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respStr, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(respStr, "Msg").String(), "world"), "world")
	svr.Stop()
}

func testThriftPingMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8122", new(GenericServicePingImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:8122")

	resp, err := cli.GenericCall(context.Background(), "Ping", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respMap, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(respMap, "hello"), respMap)
	svr.Stop()
}

func testThriftError(t *testing.T) {
	time.Sleep(2 * time.Second)
	svr := initThriftServer(t, ":8123", new(GenericServiceErrorImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:8123")

	_, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errResp), err.Error())
	svr.Stop()
}

func testThriftOnewayMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8124", new(GenericServiceOnewayImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:8124")

	resp, err := cli.GenericCall(context.Background(), "Oneway", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == nil)
	svr.Stop()
}

func testThriftVoidMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":8125", new(GenericServiceVoidImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:8125")

	resp, err := cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})
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
	svr := initThriftServerByIDL(t, ":8126", new(GenericServiceBinaryEchoImpl), "./idl/binary_echo.thrift", &(&struct{ x bool }{false}).x)
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:8126", "./idl/binary_echo.thrift", &(&struct{ x bool }{false}).x)

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
	svr := initThriftServerByIDL(t, ":8126", new(GenericServiceBinaryEchoImpl), "./idl/binary_echo.thrift", nil)
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClientByIDL(t, "127.0.0.1:8126", "./idl/binary_echo.thrift", nil)

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
	cli := newGenericClient("destServiceName", g, "127.0.0.1:8128")
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
	addr, _ := net.ResolveTCPAddr("tcp", ":8128")
	svr := newMockServer(handler, addr)
	return svr
}

func TestJSONThriftGenericClientClose(t *testing.T) {
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
		clis[i] = newGenericClient("destServiceName", g, "127.0.0.1:8129")
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

func TestJSONThriftGenericClientFinalizer(t *testing.T) {
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
		clis[i] = newGenericClient("destServiceName", g, "127.0.0.1:8130")
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
