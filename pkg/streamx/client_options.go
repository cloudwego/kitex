package streamx

import "time"

type ClientOptions struct {
	RecvTimeout   time.Duration
	StreamMWs     []StreamMiddleware
	StreamRecvMWs []StreamRecvMiddleware
	StreamSendMWs []StreamSendMiddleware
}
