package server

import (
	"context"

	"github.com/cloudwego/kitex/pkg/streaming"
)

type UnaryEndpoint func(ctx context.Context, req, resp interface{}) (err error)

type UnaryMiddleware func(next UnaryEndpoint) UnaryEndpoint

type StreamEndpoint func(ctx context.Context, st streaming.ServerStream) (err error)

type StreamMiddleware func(next StreamEndpoint) StreamEndpoint

type StreamRecvEndpoint func(ctx context.Context, stream streaming.ServerStream, message interface{}) (err error)

type StreamRecvMiddleware func(next StreamRecvEndpoint) StreamRecvEndpoint

type StreamSendEndpoint func(ctx context.Context, stream streaming.ServerStream, message interface{}) (err error)

type StreamSendMiddleware func(next StreamSendEndpoint) StreamSendEndpoint

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

func StreamRecvChain(mws ...StreamRecvMiddleware) StreamRecvMiddleware {
	return func(next StreamRecvEndpoint) StreamRecvEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func StreamSendChain(mws ...StreamSendMiddleware) StreamSendMiddleware {
	return func(next StreamSendEndpoint) StreamSendEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}
