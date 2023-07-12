package proto

import (
	"context"
)

// MessageReader read from ActualMsgBuf with method and returns a string
type MessageReader interface {
	Read(ctx context.Context, method string, actualMsgBuf []byte) (interface{}, error)
}

// MessageWriter writes to a dynamic message and returns an output bytebuffer
type MessageWriter interface {
	Write(ctx context.Context, msg interface{}) (interface{}, error)
}
