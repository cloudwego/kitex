package streamx

import (
	"context"

	"github.com/cloudwego/kitex/pkg/stats"
)

func NewStreamRecvStatMiddleware(ehandler EventHandler) StreamRecvMiddleware {
	return func(next StreamRecvEndpoint) StreamRecvEndpoint {
		return func(ctx context.Context, stream Stream, res any) (err error) {
			err = next(ctx, stream, res)
			ehandler(ctx, stats.StreamRecv, err)
			return err
		}
	}
}

func NewStreamSendStatMiddleware(ehandler EventHandler) StreamSendMiddleware {
	return func(next StreamSendEndpoint) StreamSendEndpoint {
		return func(ctx context.Context, stream Stream, res any) (err error) {
			err = next(ctx, stream, res)
			ehandler(ctx, stats.StreamSend, err)
			return err
		}
	}
}
