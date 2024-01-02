/*
 * Copyright 2023 CloudWeGo Authors
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
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/cloudwego/dynamicgo/proto"

	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/server"
)

func TestRun(t *testing.T) {
	t.Run("TestEcho", TestEcho)
	t.Run("TestExampleMethod", TestExampleMethod)
	t.Run("TestVoid", TestVoid)
	t.Run("TestExampleMethod2", TestExampleMethod2)
	t.Run("TestInt2FloatMethod", TestInt2FloatMethod)
	t.Run("TestInt2FloatMethod2", TestInt2FloatMethod2)
}

func initPbServerByIDLDynamicGo(t *testing.T, address string, handler generic.Service, pbIdl string) server.Server {
	// initialise DescriptorProvider for DynamicGo
	opts := proto.Options{}
	p, err := generic.NewPbContentProviderDynamicGo(pbIdl, context.Background(), opts)
	if err != nil {
		panic(err)
	}
	test.Assert(t, err == nil)

	addr, _ := net.ResolveTCPAddr("tcp", address)

	g, err := generic.JSONPbGeneric(p)
	test.Assert(t, err == nil)
	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func initPbClientByIDLDynamicGo(t *testing.T, addr, destSvcName, pbIdl string) genericclient.Client {
	// initialise dynamicgo proto.ServiceDescriptor
	opts := proto.Options{}
	p, err := generic.NewPbContentProviderDynamicGo(pbIdl, context.Background(), opts)
	if err != nil {
		panic(err)
	}
	test.Assert(t, err == nil)

	g, err := generic.JSONPbGeneric(p)
	test.Assert(t, err == nil)
	cli := newGenericClient(destSvcName, g, addr)
	test.Assert(t, err == nil)
	return cli
}

func TestEcho(t *testing.T) {
	addr := test.GetLocalAddress()
	time.Sleep(1 * time.Second)
	svr := initPbServerByIDLDynamicGo(t, addr, new(TestEchoService), "./idl/echo.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDLDynamicGo(t, addr, "EchoService", "./idl/echo.proto")

	ctx := context.Background()

	// 'ExampleMethod' method name must be passed as param
	resp, err := cli.GenericCall(ctx, "Echo", `{"message": "this is the request"}`)
	// resp is a JSON string
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)

	svr.Stop()
}

func TestExampleMethod(t *testing.T) {
	addr := test.GetLocalAddress()
	time.Sleep(1 * time.Second)
	svr := initPbServerByIDLDynamicGo(t, addr, new(TestExampleMethodService), "./idl/example.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDLDynamicGo(t, addr, "ExampleService", "./idl/example.proto")

	ctx := context.Background()

	// 'ExampleMethod' method name must be passed as param
	resp, err := cli.GenericCall(ctx, "ExampleMethod", `{"reqs":["req_one","req_two","req_three"]}`)
	// resp is a JSON string
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)

	svr.Stop()
}

func TestVoid(t *testing.T) {
	addr := test.GetLocalAddress()
	time.Sleep(1 * time.Second)
	svr := initPbServerByIDLDynamicGo(t, addr, new(TestVoidService), "./idl/example.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDLDynamicGo(t, addr, "ExampleService", "./idl/example.proto")

	ctx := context.Background()

	// 'ExampleMethod' method name must be passed as param
	resp, err := cli.GenericCall(ctx, "VoidMethod", `{}`)
	// resp is a JSON string
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)

	svr.Stop()
}

func TestExampleMethod2(t *testing.T) {
	addr := test.GetLocalAddress()
	time.Sleep(1 * time.Second)
	svr := initPbServerByIDLDynamicGo(t, addr, new(TestExampleMethod2Service), "./idl/example2.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDLDynamicGo(t, addr, "ExampleMethod", "./idl/example2.proto")

	ctx := context.Background()

	// 'ExampleMethod' method name must be passed as param
	resp, err := cli.GenericCall(ctx, "ExampleMethod", getExampleReq())
	// resp is a JSON string
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)

	var jsonMapResp map[string]interface{}
	var jsonMapOriginal map[string]interface{}
	json.Unmarshal([]byte(resp.(string)), &jsonMapResp)
	json.Unmarshal([]byte(getExampleReq()), &jsonMapOriginal)
	test.Assert(t, mapsEqual(jsonMapResp, jsonMapOriginal))

	svr.Stop()
}

func TestInt2FloatMethod(t *testing.T) {
	addr := test.GetLocalAddress()
	ExampleInt2FloatReq := `{"Int32":1,"Float64":3.14,"String":"hello","Int64":2,"Subfix":0.92653}`
	//`{"Float64":` + strconv.Itoa(math.MaxInt64) + `}`
	time.Sleep(1 * time.Second)
	svr := initPbServerByIDLDynamicGo(t, addr, new(TestInt2FloatMethodService), "./idl/example2.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDLDynamicGo(t, addr, "Int2FloatMethod", "./idl/example2.proto")

	ctx := context.Background()

	// 'ExampleMethod' method name must be passed as param
	resp, err := cli.GenericCall(ctx, "Int2FloatMethod", ExampleInt2FloatReq)
	// resp is a JSON string
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)

	svr.Stop()
}

func TestInt2FloatMethod2(t *testing.T) {
	addr := test.GetLocalAddress()
	ExampleInt2FloatReq := `{"Int64":` + strconv.Itoa(math.MaxInt64) + `}`

	time.Sleep(1 * time.Second)
	svr := initPbServerByIDLDynamicGo(t, addr, new(TestInt2FloatMethod2Service), "./idl/example2.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDLDynamicGo(t, addr, "Int2FloatMethod2", "./idl/example2.proto")

	ctx := context.Background()

	resp, err := cli.GenericCall(ctx, "Int2FloatMethod", ExampleInt2FloatReq)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)

	svr.Stop()
}

func mapsEqual(map1, map2 map[string]interface{}) bool {
	if len(map1) != len(map2) {
		return false
	}

	for key, value1 := range map1 {
		value2, ok := map2[key]
		if !ok || !reflect.DeepEqual(value1, value2) {
			return false
		}
	}

	return true
}