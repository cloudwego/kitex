package streamx

import (
	"context"
	"time"

	"github.com/cloudwego/kitex/pkg/stats"
)

type EventHandler func(ctx context.Context, evt stats.Event, err error)

type ClientOptions struct {
	RecvTimeout   time.Duration
	StreamMWs     []StreamMiddleware
	StreamRecvMWs []StreamRecvMiddleware
	StreamSendMWs []StreamSendMiddleware
}
