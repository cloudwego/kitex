package streamxserver

import (
	"context"
	"github.com/cloudwego/kitex/streamx"
	"testing"
)

func TestServerOption(t *testing.T) {
	WithMiddleware(func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
		return func(ctx context.Context, req, res any, stream streamx.StreamArgs) (err error) {
			return next(ctx, req, res, stream)
		}
	})
	WithMiddleware[streamx.StreamEndpoint](
		func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
			return func(ctx context.Context, req, res any, stream streamx.StreamArgs) (err error) {
				return next(ctx, req, res, stream)
			}
		},
	)
	WithMiddleware[streamx.ClientStreamEndpoint](
		func(next streamx.ClientStreamEndpoint) streamx.ClientStreamEndpoint {
			return func(ctx context.Context, stream streamx.StreamArgs) error {
				return next(ctx, stream)
			}
		},
	)
	WithMiddleware[streamx.ServerStreamEndpoint](
		func(next streamx.ServerStreamEndpoint) streamx.ServerStreamEndpoint {
			return func(ctx context.Context, req any, stream streamx.StreamArgs) error {
				return next(ctx, req, stream)
			}
		},
	)
}
