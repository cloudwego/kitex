package streamxserver

import (
	"net"

	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/server"
)

type Option internal_server.Option
type Options = internal_server.Options

func WithListener(ln net.Listener) Option {
	return convertInternalServerOption(server.WithListener(ln))
}

func WithStreamMiddleware(mw streamx.StreamMiddleware) server.RegisterOption {
	return server.RegisterOption{F: func(o *internal_server.RegisterOptions) {
		o.StreamMiddlewares = append(o.StreamMiddlewares, mw)
	}}
}

func WithStreamRecvMiddleware(mw streamx.StreamRecvMiddleware) server.RegisterOption {
	return server.RegisterOption{F: func(o *internal_server.RegisterOptions) {
		o.StreamRecvMiddlewares = append(o.StreamRecvMiddlewares, mw)
	}}
}

func WithStreamSendMiddleware(mw streamx.StreamSendMiddleware) server.RegisterOption {
	return server.RegisterOption{F: func(o *internal_server.RegisterOptions) {
		o.StreamSendMiddlewares = append(o.StreamSendMiddlewares, mw)
	}}
}

func WithProvider(provider streamx.ServerProvider) server.RegisterOption {
	return server.RegisterOption{F: func(o *internal_server.RegisterOptions) {
		o.Provider = provider
	}}
}

func convertInternalServerOption(o internal_server.Option) Option {
	return Option{F: o.F}
}

func convertServerOption(o Option) internal_server.Option {
	return internal_server.Option{F: o.F}
}
