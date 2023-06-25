package test

import (
	"context"
	"net"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/genericserver"
)

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

// GenericServiceReadRequiredFiledImpl ...
type GenericServiceEchoImpl struct{}

// GenericCall ...
func (g *GenericServiceEchoImpl) GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error) {
	req := request.(map[string]interface{})
	return req, nil
}
