package genericserver

import (
	"context"

	iserver "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/server"
)

func RegisterService(svr server.Server, handler server.UnknownMethodHandler) error {
	return svr.RegisterService(&serviceinfo.ServiceInfo{
		UnknownMethodHandler: func(ctx context.Context, name string) serviceinfo.MethodInfo {
			return serviceinfo.NewMethodInfo(func(ctx context.Context, handler, args, result interface{}) error {
				handler.(server.UnknownMethodHandler).HandleUnary(ctx)
			}, nil, nil, false)
		},
	}, handler, iserver.WithUnknownService())
}
