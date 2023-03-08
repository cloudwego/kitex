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
	"net"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/client/genericclient"
	kt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/server"
)

func TestThrift(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9021", new(GenericServiceImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9021")

	cases := []struct {
		reqMsg   interface{}
		wantResp interface{}
	}{
		{
			reqMsg: map[string]interface{}{
				"Msg": "hello",
				"TestList": []interface{}{
					map[string]interface{}{
						"Bar": "foo",
					},
				},
				"TestMap": map[interface{}]interface{}{
					"l1": map[string]interface{}{
						"Bar": "foo",
					},
				},
				"StrList": []interface{}{
					"123", "456",
				},
				"I64List": []interface{}{
					int64(123), int64(456),
				},
				"B": true,
			},
			wantResp: map[string]interface{}{
				"Msg": "hello",
				"Foo": int32(0),
				"TestList": []interface{}{
					map[string]interface{}{
						"Bar": "foo",
					},
				},
				"TestMap": map[interface{}]interface{}{
					"l1": map[string]interface{}{
						"Bar": "foo",
					},
				},
				"StrList": []interface{}{
					"123", "456",
				},
				"I64List": []interface{}{
					int64(123), int64(456),
				},
				"B": true,
				"Base": map[string]interface{}{
					"LogID":  "",
					"Caller": "",
					"Addr":   "",
					"Client": "",
				},
			},
		},
		{
			reqMsg: map[string]interface{}{
				"Msg": "hello",
				"TestList": []interface{}{
					map[string]interface{}{
						"Bar": "foo",
					},
					nil,
				},
				"TestMap": map[interface{}]interface{}{
					"l1": map[string]interface{}{
						"Bar": "foo",
					},
					"l2": nil,
				},
				"StrList": []interface{}{
					"123", nil,
				},
				"I64List": []interface{}{
					int64(123), nil,
				},
				"B": nil,
			},
			wantResp: map[string]interface{}{
				"Msg": "hello",
				"Foo": int32(0),
				"TestList": []interface{}{
					map[string]interface{}{
						"Bar": "foo",
					},
					map[string]interface{}{
						"Bar": "",
					},
				},
				"TestMap": map[interface{}]interface{}{
					"l1": map[string]interface{}{
						"Bar": "foo",
					},
					"l2": map[string]interface{}{
						"Bar": "",
					},
				},
				"StrList": []interface{}{
					"123", "",
				},
				"I64List": []interface{}{
					int64(123), int64(0),
				},
				"B": false,
				"Base": map[string]interface{}{
					"LogID":  "",
					"Caller": "",
					"Addr":   "",
					"Client": "",
				},
			},
		},
		{
			reqMsg: map[string]interface{}{
				"Msg": "hello",
				"TestList": []interface{}{
					nil,
				},
				"TestMap": map[interface{}]interface{}{
					"l2": nil,
				},
				"StrList": []interface{}{
					nil,
				},
				"I64List": []interface{}{
					nil,
				},
				"B": nil,
			},
			wantResp: map[string]interface{}{
				"Msg": "hello",
				"Foo": int32(0),
				"TestList": []interface{}{
					map[string]interface{}{
						"Bar": "",
					},
				},
				"TestMap": map[interface{}]interface{}{
					"l2": map[string]interface{}{
						"Bar": "",
					},
				},
				"StrList": []interface{}{
					"",
				},
				"I64List": []interface{}{
					int64(0),
				},
				"B": false,
				"Base": map[string]interface{}{
					"LogID":  "",
					"Caller": "",
					"Addr":   "",
					"Client": "",
				},
			},
		},
		{
			reqMsg: map[string]interface{}{
				"Msg": "hello",
				"TestList": []interface{}{
					map[string]interface{}{
						"Bar": "foo",
					},
					nil,
				},
				"TestMap": map[interface{}]interface{}{
					"l1": map[string]interface{}{
						"Bar": "foo",
					},
					"l2": nil,
				},
				"StrList": []interface{}{
					"123", nil,
				},
				"I64List": []interface{}{
					float64(123), nil, float64(456),
				},
				"B": nil,
			},
			wantResp: map[string]interface{}{
				"Msg": "hello",
				"Foo": int32(0),
				"TestList": []interface{}{
					map[string]interface{}{
						"Bar": "foo",
					},
					map[string]interface{}{
						"Bar": "",
					},
				},
				"TestMap": map[interface{}]interface{}{
					"l1": map[string]interface{}{
						"Bar": "foo",
					},
					"l2": map[string]interface{}{
						"Bar": "",
					},
				},
				"StrList": []interface{}{
					"123", "",
				},
				"I64List": []interface{}{
					int64(123), int64(0), int64(456),
				},
				"B": false,
				"Base": map[string]interface{}{
					"LogID":  "",
					"Caller": "",
					"Addr":   "",
					"Client": "",
				},
			},
		},
	}
	for _, tcase := range cases {
		resp, err := cli.GenericCall(context.Background(), "ExampleMethod", tcase.reqMsg, callopt.WithRPCTimeout(100*time.Second))
		test.Assert(t, err == nil, err)
		respMap, ok := resp.(map[string]interface{})
		test.Assert(t, ok)
		test.Assert(t, reflect.DeepEqual(respMap, tcase.wantResp), respMap)
	}
	svr.Stop()
}

func TestThriftPingMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9022", new(GenericServicePingImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9022")

	resp, err := cli.GenericCall(context.Background(), "Ping", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	respMap, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(respMap, "hello"), respMap)
	svr.Stop()
}

func TestThriftError(t *testing.T) {
	time.Sleep(2 * time.Second)
	svr := initThriftServer(t, ":9023", new(GenericServiceErrorImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9023")

	_, err := cli.GenericCall(context.Background(), "ExampleMethod", reqMsg, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), errResp), err.Error())
	svr.Stop()
}

func TestThriftOnewayMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9024", new(GenericServiceOnewayImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9024")

	resp, err := cli.GenericCall(context.Background(), "Oneway", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == nil)
	svr.Stop()
}

func TestThriftVoidMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initThriftServer(t, ":9025", new(GenericServiceVoidImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftClient(t, "127.0.0.1:9025")

	resp, err := cli.GenericCall(context.Background(), "Void", "hello", callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	test.Assert(t, resp == descriptor.Void{})
	svr.Stop()
}

func TestThrift2NormalServer(t *testing.T) {
	time.Sleep(4 * time.Second)
	svr := initMockServer(t, new(mockImpl))
	time.Sleep(500 * time.Millisecond)

	cli := initThriftMockClient(t)

	_, err := cli.GenericCall(context.Background(), "Test", mockReq, callopt.WithRPCTimeout(100*time.Second))
	test.Assert(t, err == nil, err)
	svr.Stop()
}

func initThriftMockClient(t *testing.T) genericclient.Client {
	p, err := generic.NewThriftFileProvider("./idl/mock.thrift")
	test.Assert(t, err == nil)
	g, err := generic.MapThriftGeneric(p)
	test.Assert(t, err == nil)
	cli := newGenericClient("destServiceName", g, "127.0.0.1:9019")
	test.Assert(t, err == nil)
	return cli
}

func initThriftClient(t *testing.T, addr string) genericclient.Client {
	return initThriftClientByIDL(t, addr, "./idl/example.thrift")
}

func initThriftClientByIDL(t *testing.T, addr, idl string) genericclient.Client {
	p, err := generic.NewThriftFileProvider(idl)
	test.Assert(t, err == nil)
	g, err := generic.MapThriftGeneric(p)
	test.Assert(t, err == nil)
	cli := newGenericClient("destServiceName", g, addr)
	test.Assert(t, err == nil)
	return cli
}

func initThriftServer(t *testing.T, address string, handler generic.Service) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewThriftFileProvider("./idl/example.thrift")
	test.Assert(t, err == nil)
	g, err := generic.MapThriftGeneric(p)
	test.Assert(t, err == nil)
	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func initMockServer(t *testing.T, handler kt.Mock) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", ":9019")
	svr := newMockServer(handler, addr)
	return svr
}

func TestMapThriftGenericClientClose(t *testing.T) {
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
		g, err := generic.MapThriftGeneric(p)
		test.Assert(t, err == nil, "generic MapThriftGeneric failed, err=%v", err)
		clis[i] = newGenericClient("destServiceName", g, "127.0.0.1:9020")
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

func TestMapThriftGenericClientFinalizer(t *testing.T) {
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
		g, err := generic.MapThriftGeneric(p)
		test.Assert(t, err == nil, "generic MapThriftGeneric failed, err=%v", err)
		clis[i] = newGenericClient("destServiceName", g, "127.0.0.1:9021")
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
	// test.Assert(t, secondGCHeapAlloc < firstGCHeapAlloc && secondGCHeapObjects < firstGCHeapObjects)

	// Trigger the finalizer of kClient be executed
	time.Sleep(200 * time.Millisecond) // ensure the finalizer be executed
	runtime.GC()
	runtime.ReadMemStats(&ms)
	thirdGCHeapAlloc, thirdGCHeapObjects := mb(ms.HeapAlloc), ms.HeapObjects
	t.Logf("After third GC, allocation: %f Mb, Number of allocation: %d\n", thirdGCHeapAlloc, thirdGCHeapObjects)
	test.Assert(t, thirdGCHeapAlloc < secondGCHeapAlloc/2 && thirdGCHeapObjects < secondGCHeapObjects/2)
}

func mb(byteSize uint64) float32 {
	return float32(byteSize) / float32(1024*1024)
}
