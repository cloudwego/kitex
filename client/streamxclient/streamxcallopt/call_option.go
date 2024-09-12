package streamxcallopt

import (
	"fmt"
	"strings"
	"time"
)

type CallOptions struct {
	RPCTimeout time.Duration
}

type CallOption struct {
	f func(o *CallOptions, di *strings.Builder)
}

type WithCallOption func(o *CallOption)

func WithRPCTimeout(rpcTimeout time.Duration) CallOption {
	return CallOption{f: func(o *CallOptions, di *strings.Builder) {
		di.WriteString(fmt.Sprintf("WithRPCTimeout(%d)", rpcTimeout))
		o.RPCTimeout = rpcTimeout
	}}
}
