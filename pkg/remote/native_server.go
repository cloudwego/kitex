package remote

import (
	"context"
	"net"
	"time"
)

type NativeListenerOptions struct {
	MaxConnectionIdleTime, ReadWriteTimeout time.Duration
}

type NativeIngress struct {
	IO          NativeIO
	Begin       time.Time
	Peer, Local net.Addr
}

type NativeServerRequest interface {
	Context() context.Context
	Frame() NativeBuffer
	Facts() NativeIngress
	AcquireTx(context.Context) (NativeBuffer, error)
	Send(context.Context, NativeBuffer, int) (NativeIO, error)
	Release(bool) error
}

type NativeServerRequestScope interface {
	RequestContext() (context.Context, bool)
	WithRequestDeadline(context.Context, time.Time) (context.Context, error)
}

type NativeListener interface {
	Address() net.Addr
	Accept(context.Context) (NativeServerRequest, error)
	Close() error
}

type NativeConcurrentListener interface {
	ConcurrentAccept() bool
}

type NativeServerRequestCleanup interface {
	RetainFrameForCleanup() bool
	ReleaseWithBuffer(bool, NativeBuffer) (bufferErr, requestErr error)
}

type NativeServerTransport interface {
	UnaryLifecycle
	BuiltinMetaHandlers() []MetaHandler
	Listen(context.Context, net.Addr, NativeListenerOptions) (NativeListener, error)
	ProjectIngress(base, decoded context.Context, call *UnaryCall) (context.Context, error)
}

type NativeServerDeferredMetadata interface {
	DeferIngressMetadata() bool
	PrepareIngress(*UnaryCall) error
	ReadIngressMetadata(context.Context, *UnaryCall, []MetaHandler) (context.Context, error)
}
