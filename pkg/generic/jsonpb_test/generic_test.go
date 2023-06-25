package test

import (
	"context"
	"fmt"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
	"log"
	"testing"
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

	addr = "0.0.0.0:8000"
)

func TestRun(t *testing.T) {
	t.Run("testPbNormalEcho", testPbNormalEcho)
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

func testPbNormalEcho(t *testing.T) {
	cli := initPbClientByPbContentProvider(t, addr, destSvcName, path, includes)

	ctx := context.Background()

	// 'ExampleMethod' method name must be passed as param
	resp, err := cli.GenericCall(ctx, "Echo", "{\"message\": \"helloss\"}")
	// resp is a JSON string
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)
}
