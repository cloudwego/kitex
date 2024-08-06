package streamxserver

import (
	"context"
	"fmt"
	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/pkg/endpoint"
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

func WithServerProvider(pvd streamx.ServerProvider) ServerOption {
	return ServerOption{F: func(o *internal_server.Options, di *utils.Slice) {
		o.RemoteOpt.Provider = pvd
	}}
}

func WithServerTransHandlerFactory(f remote.ServerTransHandlerFactory) ServerOption {
	return convertInternalServerOption(server.WithTransHandlerFactory(f))
}

func WithServerMiddleware[Endpoint streamx.StreamEndpoint | streamx.UnaryEndpoint | streamx.ClientStreamEndpoint | streamx.ServerStreamEndpoint | streamx.BidiStreamEndpoint](
	gmw streamx.GenericMiddleware[Endpoint]) ServerOption {
	mw := convertStreamMiddleware(gmw)
	return convertInternalServerOption(server.WithMiddleware(mw))
}

func convertInternalServerOption(o internal_server.Option) ServerOption {
	return ServerOption{F: o.F}
}

func convertServerOption(o ServerOption) internal_server.Option {
	return internal_server.Option{F: o.F}
}

func convertStreamMiddleware[Endpoint streamx.StreamEndpoint | streamx.UnaryEndpoint | streamx.ClientStreamEndpoint | streamx.ServerStreamEndpoint | streamx.BidiStreamEndpoint](
	gmw streamx.GenericMiddleware[Endpoint]) endpoint.Middleware {
	switch tmw := any(gmw).(type) {
	case streamx.GenericMiddleware[streamx.StreamEndpoint]:
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, reqArgs, resArgs any) (err error) {
				stream, err := streamx.AsStream(reqArgs)
				if err != nil {
					return err
				}
				_ = stream
				return tmw(func(ctx context.Context, reqArgs, resArgs any, streamArgs streamx.StreamArgs) (err error) {
					return next(ctx, reqArgs, resArgs)
				})(ctx, reqArgs, resArgs, nil)
			}
		}
	case streamx.GenericMiddleware[streamx.UnaryEndpoint]:
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, reqArgs, resArgs any) (err error) {
				return tmw(func(ctx context.Context, reqArgs, resArgs any) error {
					return next(ctx, reqArgs, resArgs)
				})(ctx, reqArgs, resArgs)
			}
		}
	case streamx.GenericMiddleware[streamx.ClientStreamEndpoint]:
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, reqArgs, resArgs any) (err error) {
				return tmw(func(ctx context.Context, streamArgs streamx.StreamArgs) error {
					return next(ctx, nil, nil)
				})(ctx, nil)
			}
		}
	case streamx.GenericMiddleware[streamx.ServerStreamEndpoint]:
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, reqArgs, resArgs any) (err error) {
				return tmw(func(ctx context.Context, reqArgs any, streamArgs streamx.StreamArgs) error {
					return next(ctx, nil, nil)
				})(ctx, nil, nil)
			}
		}
	case streamx.GenericMiddleware[streamx.BidiStreamEndpoint]:
		return func(next endpoint.Endpoint) endpoint.Endpoint {
			return func(ctx context.Context, reqArgs, resArgs any) (err error) {
				return tmw(func(ctx context.Context, streamArgs streamx.StreamArgs) error {
					return next(ctx, nil, nil)
				})(ctx, nil)
			}
		}
	default:
		panic(fmt.Sprintf("Unknow Middleware Type: %T", tmw))
	}
}
