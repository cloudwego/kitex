package test

import (
	"context"
	"fmt"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/server"
	"io/ioutil"
	"log"
	"net"
	"os"
	"testing"
	"time"
)

var (
	path    = "./jsonpb_test/idl/echo.proto"
	content = `syntax = "proto3";
	package pbapi;
	// The greeting service definition.
	option go_package = "pbapi";
	
	message Request {
	  string message = 1;
	}
	
	message Response {
	  string message = 1;
	}
	
	service Echo {
	  rpc Echo (Request) returns (Response) {}
	}`

	includes = map[string]string{
		path: content,
	}

	destSvcName = "Echo"

	addr = "0.0.0.0:8888"
)

func TestRun(t *testing.T) {
	t.Run("testPbEchoServiceEcho", testPbEchoServiceEcho)
	t.Run("testPbExampleServiceExampleMethod", testPbExampleServiceExampleMethod)
	t.Run("testPbExampleServiceVoid", testPbExampleServiceVoid)
}

func initPbClientByPbContentProvider(t *testing.T, addr, destSvcName string, path string, includes map[string]string) genericclient.Client {
	pbp, err := generic.NewPbContentProvider(path, includes)
	test.Assert(t, err == nil)
	g, err := generic.JSONPbGeneric(pbp)
	test.Assert(t, err == nil)
	cli := newGenericClient(destSvcName, g, addr)
	test.Assert(t, err == nil)
	return cli
}

func initPbClientByIDL(t *testing.T, addr, destSvcName string, pbIdl string) genericclient.Client {
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

	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewPbContentProvider(pbIdl, map[string]string{pbIdl: string(pbContent)})
	test.Assert(t, err == nil)
	g, err := generic.JSONPbGeneric(p)
	test.Assert(t, err == nil)

	svr := newGenericServer(g, addr, handler)
	test.Assert(t, err == nil)
	return svr
}

func initPbServerByPbContentProvider(t *testing.T, address string, handler generic.Service, idlPath string, includes map[string]string) server.Server {
	addr, _ := net.ResolveTCPAddr("tcp", address)
	p, err := generic.NewPbContentProvider(idlPath, includes)
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
	svr := initPbServerByIDL(t, "127.0.0.1:8128", new(ExampleGenericServiceImpl), "./idl/example.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDL(t, "127.0.0.1:8128", "ExampleService", "./idl/example.proto")

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
	svr := initPbServerByIDL(t, "127.0.0.1:8128", new(ExampleVoidGenericServiceImpl), "./idl/example.proto")
	time.Sleep(500 * time.Millisecond)

	cli := initPbClientByIDL(t, "127.0.0.1:8128", "ExampleService", "./idl/example.proto")

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
