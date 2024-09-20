package streamxclient

import (
	"github.com/cloudwego/kitex/client"
	internal_client "github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/utils"
)

type Option internal_client.Option
type Options = internal_client.Options

func WithHostPorts(hostports ...string) Option {
	return ConvertNativeClientOption(client.WithHostPorts(hostports...))
}

func WithDestService(destService string) Option {
	return ConvertNativeClientOption(client.WithDestService(destService))
}

func WithProvider(pvd streamx.ClientProvider) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.RemoteOpt.Provider = pvd
	}}
}

func WithStreamMiddleware(smw streamx.StreamMiddleware) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.SMWs = append(o.SMWs, smw)
	}}
}

func WithStreamRecvMiddleware(smw streamx.StreamRecvMiddleware) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.SRecvMWs = append(o.SRecvMWs, smw)
	}}
}

func WithStreamSendMiddleware(smw streamx.StreamSendMiddleware) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.SSendMWs = append(o.SSendMWs, smw)
	}}
}

func ConvertNativeClientOption(o internal_client.Option) Option {
	return Option{F: o.F}
}

func ConvertStreamXClientOption(o Option) internal_client.Option {
	return internal_client.Option{F: o.F}
}
