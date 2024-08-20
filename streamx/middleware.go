package streamx

import (
	"context"
)

func Chain(mws ...StreamMiddleware) StreamMiddleware {
	return func(next StreamEndpoint) StreamEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

type StreamEndpoint func(ctx context.Context, reqArgs StreamReqArgs, resArgs StreamResArgs, streamArgs StreamArgs) (err error)
type StreamMiddleware func(next StreamEndpoint) StreamEndpoint

//type UnaryEndpoint func(ctx context.Context, reqArgs, resArgs any) error
//type ClientStreamEndpoint func(ctx context.Context, streamArgs StreamArgs) error
//type ServerStreamEndpoint func(ctx context.Context, reqArgs any, streamArgs StreamArgs) error
//type BidiStreamEndpoint func(ctx context.Context, streamArgs StreamArgs) error

//type GenericMiddleware[endpoint StreamEndpoint | UnaryEndpoint | ClientStreamEndpoint | ServerStreamEndpoint | BidiStreamEndpoint] func(next endpoint) endpoint

type StreamRecvEndpoint func(ctx context.Context, msg any, streamArgs StreamArgs) (err error)
type StreamSendEndpoint func(ctx context.Context, msg any, streamArgs StreamArgs) (err error)

type StreamRecvMiddleware func(next StreamRecvEndpoint) StreamRecvEndpoint
type StreamSendMiddleware func(next StreamSendEndpoint) StreamSendEndpoint
