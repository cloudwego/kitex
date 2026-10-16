package remote

import (
	"context"
	"time"
)

type NativeBuffer interface {
	Bytes() []byte
	Release() error
}

type NativeIO struct {
	ConnID                                                  uint64
	Reused                                                  bool
	ConnectStart, ConnectEnd, WriteStart, WriteEnd, ReadEnd time.Time
	SentBytes, ReceivedBytes                                uint64
	FailureStage                                            UnaryFailureStage
}

type NativeClientConnection interface {
	AcquireTx(context.Context) (NativeBuffer, error)
	Exchange(context.Context, NativeBuffer, int) (NativeBuffer, NativeIO, error)
	Release(bool) error
}

type NativeClientTransport interface {
	UnaryLifecycle
	BuiltinMetaHandlers() []MetaHandler
	Connect(context.Context, *UnaryCall) (NativeClientConnection, NativeIO, error)
}
