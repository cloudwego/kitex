package streamxserver

import (
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/streamx"
)

type Server server.Server

func NewServer(opts ...ServerOption) Server {
	iopts := make([]server.Option, 0, len(opts)+1)
	iopts = append(iopts, server.WithTransHandlerFactory(streamx.NewSvrTransHandlerFactory()))
	for _, opt := range opts {
		iopts = append(iopts, convertServerOption(opt))
	}
	s := server.NewServer(iopts...)
	return s
}
