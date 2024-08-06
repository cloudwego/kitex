package streamx

import (
	"context"
)

type StreamHandler struct {
	Handler              any
	StreamMiddleware     StreamMiddleware
	StreamRecvMiddleware StreamRecvMiddleware
	StreamSendMiddleware StreamSendMiddleware
}

type StreamEndpoint func(ctx context.Context, streamArgs StreamArgs, reqArgs StreamReqArgs, resArgs StreamResArgs) (err error)
type StreamMiddleware func(next StreamEndpoint) StreamEndpoint

type StreamRecvEndpoint func(ctx context.Context, stream Stream, res any) (err error)
type StreamSendEndpoint func(ctx context.Context, stream Stream, req any) (err error)

type StreamRecvMiddleware func(next StreamRecvEndpoint) StreamRecvEndpoint
type StreamSendMiddleware func(next StreamSendEndpoint) StreamSendEndpoint

func StreamMiddlewareChain(mws ...StreamMiddleware) StreamMiddleware {
	return func(next StreamEndpoint) StreamEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func StreamRecvMiddlewareChain(mws ...StreamRecvMiddleware) StreamRecvMiddleware {
	return func(next StreamRecvEndpoint) StreamRecvEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func StreamSendMiddlewareChain(mws ...StreamSendMiddleware) StreamSendMiddleware {
	return func(next StreamSendEndpoint) StreamSendEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}
