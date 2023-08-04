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
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/server"
)

func TestRun(t *testing.T) {
	t.Run("testPbEchoServiceEcho", testPbEchoServiceEcho)
	t.Run("testPbExampleServiceExampleMethod", testPbExampleServiceExampleMethod)
	t.Run("testPbExampleServiceVoid", testPbExampleServiceVoid)
}

func initPbClientByIDL(t *testing.T, addr, destSvcName, pbIdl string) genericclient.Client {
	pbf, err := os.Open(pbIdl)
	test.Assert(t, err == nil)
	pbContent, err := ioutil.ReadAll(pbf)
	test.Assert(t, err == nil)
	pbf.Close()
	pbp, err := generic.NewPbContentProvider(pbIdl, map[string]string{pbIdl: string(pbContent)})
	test.Assert(t, err == nil)
	g, err := generic.JSONPbGeneric(pbp)
	test.Assert(t, err == nil)
	cli := newGenericClient(destSvcName, g, addr)
	test.Assert(t, err == nil)
	return cli
}

func initPbServerByIDL(t *testing.T, address string, handler generic.Service, pbIdl string) server.Server {
	pbf, err := os.Open(pbIdl)
	test.Assert(t, err == nil)
	pbContent, err := ioutil.ReadAll(pbf)
	test.Assert(t, err == nil)

	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewPbContentProvider(pbIdl, map[string]string{pbIdl: string(pbContent)})
	test.Assert(t, err == nil)
	g, err := generic.JSONPbGeneric(p)
	test.Assert(t, err == nil)

	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func testPbEchoServiceEcho(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initPbServerByIDL(t, ":8128", new(GenericServiceImpl), "./idl/echo.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDL(t, "127.0.0.1:8128", "EchoService", "./idl/echo.proto")

	ctx := context.Background()

	// 'ExampleMethod' method name must be passed as param
	resp, err := cli.GenericCall(ctx, "Echo", `{"message": "helloss"}`)
	// resp is a JSON string
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)

	svr.Stop()
}

func testPbExampleServiceExampleMethod(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initPbServerByIDL(t, ":8129", new(ExampleGenericServiceImpl), "./idl/example.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDL(t, "127.0.0.1:8129", "ExampleService", "./idl/example.proto")

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

func testPbExampleServiceVoid(t *testing.T) {
	time.Sleep(1 * time.Second)
	svr := initPbServerByIDL(t, ":8130", new(ExampleVoidGenericServiceImpl), "./idl/example.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDL(t, "127.0.0.1:8130", "ExampleService", "./idl/example.proto")

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
