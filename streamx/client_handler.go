package streamx

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/pkg/remote"
	"net"
)

var _ remote.ClientTransHandler = (*clientTransHandler)(nil)

func NewClientTransHandlerFactor() remote.ClientTransHandlerFactory {
	return &clientTransHandlerFactory{}
}

type clientTransHandlerFactory struct{}

func (s clientTransHandlerFactory) NewTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	return NewClientTransHandler(opt)
}

func NewClientTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	cp, ok := opt.Provider.(ClientProvider)
	if !ok || cp == nil {
		return nil, errors.New("invalid client provider")
	}
	return &clientTransHandler{
		opt:      opt,
		provider: cp,
	}, nil
}

type clientTransHandler struct {
	opt      *remote.ClientOption
	provider ClientProvider
}

func (c *clientTransHandler) Write(ctx context.Context, conn net.Conn, send remote.Message) (nctx context.Context, err error) {
	return ctx, nil
}

func (c *clientTransHandler) Read(ctx context.Context, conn net.Conn, msg remote.Message) (nctx context.Context, err error) {
	return ctx, nil
}

func (c *clientTransHandler) OnInactive(ctx context.Context, conn net.Conn) {
	return
}

func (c *clientTransHandler) OnError(ctx context.Context, err error, conn net.Conn) {
	return
}

func (c *clientTransHandler) OnMessage(ctx context.Context, args, result remote.Message) (context.Context, error) {
	return ctx, nil
}

func (c *clientTransHandler) SetPipeline(pipeline *remote.TransPipeline) {
	return
}
