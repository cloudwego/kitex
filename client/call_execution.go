package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/remote"
)

func (kc *kClient) buildClientCall(ctx context.Context, kind remote.ClientCallType, handler remote.ClientTransHandler, next endpoint.Endpoint) (endpoint.Endpoint, error) {
	if factory, ok := kc.opt.RemoteOpt.CliHandlerFactory.(remote.ClientCallFactory); ok {
		call, err := factory.NewClientCall(ctx, remote.ClientCallConfig{
			Type: kind, ServiceInfo: kc.svcInfo, RPCConfig: kc.opt.Configs,
			RemoteOption: kc.opt.RemoteOpt, MetaHandlers: kc.opt.MetaHandlers,
			TransHandler: handler, RunTask: workerPool.RunTask,
		}, next)
		if errors.Is(err, remote.ErrUnsupportedClientCall) {
			return func(context.Context, interface{}, interface{}) error { return err }, nil
		}
		if err != nil {
			return nil, err
		}
		if call == nil {
			return nil, fmt.Errorf("client call factory %T returned a nil endpoint", factory)
		}
		return call, nil
	}
	if kind == remote.ClientCallUnary {
		return rpcTimeoutMW(ctx).ToMiddleware()(next), nil
	}
	return next, nil
}
