package remote

import (
	"context"
	"errors"
	"time"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// ClientCallType identifies the execution boundary being constructed. Streaming
// includes client, server and bidirectional streams; the invocation identifies
// the method's concrete streaming mode at call time.
type ClientCallType uint8

const (
	ClientCallUnary ClientCallType = iota
	ClientCallStreaming
)

var ErrUnsupportedClientCall = errors.New("client transport does not support this call type")

// ClientTaskRunner runs a task with Kitex's timeout worker. Arguments remain
// owned by the task until it exits, even if its caller returns on cancellation.
type ClientTaskRunner func(context.Context, time.Duration, any, any, endpoint.Endpoint) (context.Context, error)

type ClientCallConfig struct {
	Type         ClientCallType
	ServiceInfo  *serviceinfo.ServiceInfo
	RPCConfig    rpcinfo.RPCConfig
	RemoteOption *ClientOption
	MetaHandlers []MetaHandler
	TransHandler ClientTransHandler
	RunTask      ClientTaskRunner
}

// ClientCallFactory is an optional capability of ClientTransHandlerFactory.
// It constructs a call's execution boundary around next, in place of the
// framework's default execution (RPC timeout for unary, direct for streaming).
// The factory owns timeout/cancellation execution for the call types it handles.
// Routing, middleware and connection configuration remain independent APIs.
// Returning ErrUnsupportedClientCall rejects that call type when invoked without
// preventing construction of a client supporting other call types.
type ClientCallFactory interface {
	NewClientCall(context.Context, ClientCallConfig, endpoint.Endpoint) (endpoint.Endpoint, error)
}
