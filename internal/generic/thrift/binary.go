package thrift

import (
	"context"
	"io"

	"github.com/cloudwego/gopkg/protocol/thrift/base"
)

// WriteBinary implement of MessageWriter
type WriteBinary struct {
}

func NewWriteBinary() *WriteBinary {
	return &WriteBinary{}
}

func (w *WriteBinary) Write(ctx context.Context, out io.Writer, msg interface{}, method string, isClient bool, requestBase *base.Base) error {
	return nil
}
