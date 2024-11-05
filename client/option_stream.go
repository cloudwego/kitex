package client

import (
	"github.com/cloudwego/kitex/internal/client"
	cep "github.com/cloudwego/kitex/pkg/endpoint/client"
	"github.com/cloudwego/kitex/pkg/utils"
)

func WithStreamOptions(opts ...StreamOption) Option {
	return Option{F: func(o *client.Options, di *utils.Slice) {
		if o.StreamOptions == nil {
			o.StreamOptions = &StreamOptions{}
		}
		for _, opt := range opts {
			opt.F(o.StreamOptions, nil)
		}
	}}
}

func WithStreamMiddleware(mw cep.StreamMiddleware) StreamOption {
	return StreamOption{F: func(o *StreamOptions, di *utils.Slice) {
		o.StreamMiddlewares = append(o.StreamMiddlewares, mw)
	}}
}

func WithStreamRecvMiddleware(mw cep.StreamRecvMiddleware) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		o.StreamRecvMiddlewares = append(o.StreamRecvMiddlewares, mw)
	}}
}

func WithStreamSendMiddleware(mw cep.StreamSendMiddleware) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		o.StreamSendMiddlewares = append(o.StreamSendMiddlewares, mw)
	}}
}
