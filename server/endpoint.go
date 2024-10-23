package server

import (
	"context"

	"github.com/cloudwego/kitex/pkg/streaming"
)

type InnerEndpoint func(ctx context.Context, st streaming.ServerStream) (err error)

type InnerMiddleware func(next InnerEndpoint) InnerEndpoint

type UnaryEndpoint func(ctx context.Context, req, resp interface{}) (err error)

type UnaryMiddleware func(next UnaryEndpoint) UnaryEndpoint

type StreamEndpoint func(ctx context.Context, st streaming.ServerStream) (err error)

type StreamMiddleware func(next StreamEndpoint) StreamEndpoint

func InnerChain(mws ...InnerMiddleware) InnerMiddleware {
	return func(next InnerEndpoint) InnerEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func UnaryChain(mws ...UnaryMiddleware) UnaryMiddleware {
	return func(next UnaryEndpoint) UnaryEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func StreamChain(mws ...StreamMiddleware) StreamMiddleware {
	return func(next StreamEndpoint) StreamEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}
