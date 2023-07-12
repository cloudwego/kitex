package test

import (
	"context"
	"fmt"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"net"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/genericserver"
)

var reqMsg = `{"Msg":"hello","InnerBase":{"Base":{"LogID":"log_id_inner"}},"Base":{"LogID":"log_id"}}`

var respMsgWithExtra = `{"Msg":"world","required_field":"required_field","extra_field":"extra_field"}`

var errResp = "Test Error"

func newGenericClient(destService string, g generic.Generic, targetIPPort string) genericclient.Client {
	var opts []client.Option
	opts = append(opts, client.WithHostPorts(targetIPPort))
	genericCli, _ := genericclient.NewClient(destService, g, opts...)
	return genericCli
}

func newGenericServer(g generic.Generic, addr net.Addr, handler generic.Service) server.Server {
	var opts []server.Option
	opts = append(opts, server.WithServiceAddr(addr))
	svr := genericserver.NewServer(handler, g, opts...)
	go func() {
		err := svr.Run()
		if err != nil {
			panic(err)
		}
	}()
	return svr
}

// GenericServiceImpl ...
type GenericServiceImpl struct{}

// GenericCall ...
func (g *GenericServiceImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	buf := request.(string)
	rpcinfo := rpcinfo.GetRPCInfo(ctx)
	fmt.Printf("Method from Ctx: %s\n", rpcinfo.Invocation().MethodName())
	fmt.Printf("Recv: %v\n", buf)
	fmt.Printf("Method: %s\n", method)
	return "{\"message\": \"this is the response\"}", nil
}

// GenericService for example.proto
type ExampleGenericServiceImpl struct{}

// GenericCall ...
func (g *ExampleGenericServiceImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	buf := request.(string)
	rpcinfo := rpcinfo.GetRPCInfo(ctx)
	fmt.Printf("Method from Ctx: %s\n", rpcinfo.Invocation().MethodName())
	fmt.Printf("Recv: %v\n", buf)
	fmt.Printf("Method: %s\n", method)
	return `{"resps":["res_one","res_two","res_three"]}`, nil
}

// GenericService for example.proto
type ExampleVoidGenericServiceImpl struct{}

// GenericCall ...
func (g *ExampleVoidGenericServiceImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	buf := request.(string)
	rpcinfo := rpcinfo.GetRPCInfo(ctx)
	fmt.Printf("Method from Ctx: %s\n", rpcinfo.Invocation().MethodName())
	fmt.Printf("Recv: %v\n", buf)
	fmt.Printf("Method: %s\n", method)
	return `{}`, nil
}
