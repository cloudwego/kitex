package client

import (
	"github.com/cloudwego/kitex/internal/client"
	cep "github.com/cloudwego/kitex/pkg/endpoint/client"
	"github.com/cloudwego/kitex/pkg/utils"
)

func WithUnaryOptions(opts ...UnaryOption) Option {
	return Option{F: func(o *client.Options, di *utils.Slice) {
		if o.UnaryOptions == nil {
			o.UnaryOptions = &UnaryOptions{}
		}
		for _, opt := range opts {
			opt.F(o.UnaryOptions, nil)
		}
	}}
}

func WithUnaryMiddleware(mw cep.UnaryMiddleware) UnaryOption {
	return UnaryOption{F: func(o *UnaryOptions, di *utils.Slice) {
		o.UnaryMiddlewares = append(o.UnaryMiddlewares, mw)
	}}
}
