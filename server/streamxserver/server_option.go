package streamxserver

import (
	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/streamx"
	"net"
)

type ServerOption internal_server.Option
type ServerOptions = internal_server.Options

func WithListener(ln net.Listener) ServerOption {
	return convertInternalServerOption(server.WithListener(ln))
}

func WithProvider(pvd streamx.ServerProvider) ServerOption {
	return ServerOption{F: func(o *internal_server.Options, di *utils.Slice) {
		o.RemoteOpt.Provider = pvd
	}}
}

func WithServerTransHandlerFactory(f remote.ServerTransHandlerFactory) ServerOption {
	return convertInternalServerOption(server.WithTransHandlerFactory(f))
}

func WithStreamMiddleware(smw streamx.StreamMiddleware) ServerOption {
	return ServerOption{F: func(o *internal_server.Options, di *utils.Slice) {
		o.SMWs = append(o.SMWs, smw)
	}}
}

func WithStreamRecvMiddleware(smw streamx.StreamRecvMiddleware) ServerOption {
	return ServerOption{F: func(o *internal_server.Options, di *utils.Slice) {
		o.SRecvMWs = append(o.SRecvMWs, smw)
	}}
}

func WithStreamSendMiddleware(smw streamx.StreamSendMiddleware) ServerOption {
	return ServerOption{F: func(o *internal_server.Options, di *utils.Slice) {
		o.SSendMWs = append(o.SSendMWs, smw)
	}}
}

func convertInternalServerOption(o internal_server.Option) ServerOption {
	return ServerOption{F: o.F}
}

func convertServerOption(o ServerOption) internal_server.Option {
	return internal_server.Option{F: o.F}
}
