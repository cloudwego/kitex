package client

import (
	"time"

	"github.com/cloudwego/kitex/internal/client"
	cep "github.com/cloudwego/kitex/pkg/endpoint/client"
	"github.com/cloudwego/kitex/pkg/utils"
)

// WithStreamOptions add stream options for client.
// It is used to isolate options that are only effective for streaming methods.
func WithStreamOptions(opts ...StreamOption) Option {
	return Option{F: func(o *client.Options, di *utils.Slice) {
		for _, opt := range opts {
			opt.F(&o.StreamOptions, nil)
		}
	}}
}

// WithStreamRecvTimeout add recv timeout for stream.Recv function.
func WithStreamRecvTimeout(timeout time.Duration) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		o.RecvTimeout = timeout
	}}
}

// WithStreamMiddleware add middleware for stream.
func WithStreamMiddleware(mw cep.StreamMiddleware) StreamOption {
	return StreamOption{F: func(o *StreamOptions, di *utils.Slice) {
		o.StreamMiddlewares = append(o.StreamMiddlewares, mw)
	}}
}

// WithStreamMiddlewareBuilder add middleware builder for stream.
func WithStreamMiddlewareBuilder(mwb cep.StreamMiddlewareBuilder) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		o.StreamMiddlewareBuilders = append(o.StreamMiddlewareBuilders, mwb)
	}}
}

// WithStreamRecvMiddleware add recv middleware for stream.
func WithStreamRecvMiddleware(mw cep.StreamRecvMiddleware) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		o.StreamRecvMiddlewares = append(o.StreamRecvMiddlewares, mw)
	}}
}

// WithStreamRecvMiddlewareBuilder add recv middleware builder for stream.
func WithStreamRecvMiddlewareBuilder(mwb cep.StreamRecvMiddlewareBuilder) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		o.StreamRecvMiddlewareBuilders = append(o.StreamRecvMiddlewareBuilders, mwb)
	}}
}

// WithStreamSendMiddleware add send middleware for stream.
func WithStreamSendMiddleware(mw cep.StreamSendMiddleware) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		o.StreamSendMiddlewares = append(o.StreamSendMiddlewares, mw)
	}}
}

// WithStreamSendMiddlewareBuilder add send middleware builder for stream.
func WithStreamSendMiddlewareBuilder(mwb cep.StreamSendMiddlewareBuilder) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		o.StreamSendMiddlewareBuilders = append(o.StreamSendMiddlewareBuilders, mwb)
	}}
}
