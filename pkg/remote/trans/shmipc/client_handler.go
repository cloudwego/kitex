package shmipc

import (
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans"
)

type cliTransHandlerFactory struct{}

// NewCliTransHandlerFactory to build ClientTransHandlerFactory in shmipc mode
func NewCliTransHandlerFactory() remote.ClientTransHandlerFactory {
	return &cliTransHandlerFactory{}
}

func (f *cliTransHandlerFactory) NewTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	return newCliTransHandler(opt)
}

func newCliTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	return trans.NewDefaultCliTransHandler(opt, newShmIPCExtension())
}
