package streamx

import (
	"context"
)

type StreamEndpoint func(ctx context.Context, reqArgs, resArgs any, streamArgs StreamArgs) (err error)
type UnaryEndpoint func(ctx context.Context, reqArgs, resArgs any) error
type ClientStreamEndpoint func(ctx context.Context, streamArgs StreamArgs) error
type ServerStreamEndpoint func(ctx context.Context, reqArgs any, streamArgs StreamArgs) error
type BidiStreamEndpoint func(ctx context.Context, streamArgs StreamArgs) error

type StreamMiddleware func(next StreamEndpoint) StreamEndpoint
type GenericMiddleware[endpoint StreamEndpoint | UnaryEndpoint | ClientStreamEndpoint | ServerStreamEndpoint | BidiStreamEndpoint] func(next endpoint) endpoint

type StreamRecvEndpoint func(ctx context.Context, msg any, streamArgs StreamArgs) (err error)
type StreamSendEndpoint func(ctx context.Context, msg any, streamArgs StreamArgs) (err error)

type StreamRecvMiddleware func(next StreamRecvEndpoint) StreamRecvEndpoint
type StreamSendMiddleware func(next StreamSendEndpoint) StreamSendEndpoint
