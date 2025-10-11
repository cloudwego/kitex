package shmipc

import (
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans"
)

type svrTransHandlerFactory struct{}

// NewSvrTransHandlerFactory ...
func NewSvrTransHandlerFactory() remote.ServerTransHandlerFactory {
	return &svrTransHandlerFactory{}
}

func (f *svrTransHandlerFactory) NewTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	return newSvrTransHandler(opt)
}

func newSvrTransHandler(opt *remote.ServerOption) (remote.ServerTransHandler, error) {
	return trans.NewDefaultSvrTransHandler(opt, newShmIPCExtension())
}
