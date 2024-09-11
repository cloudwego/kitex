package streamxserver

import (
	"github.com/cloudwego/kitex/server"
)

type Server = server.Server

func NewServer(opts ...Option) server.Server {
	iopts := make([]server.Option, 0, len(opts)+1)
	for _, opt := range opts {
		iopts = append(iopts, convertServerOption(opt))
	}
	s := server.NewServer(iopts...)
	return s
}
