package streamxclient

import (
	"time"

	"github.com/cloudwego/kitex/client"
	internal_client "github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/utils"
)

type Option internal_client.Option

func WithHostPorts(hostports ...string) Option {
	return ConvertNativeClientOption(client.WithHostPorts(hostports...))
}

func WithRecvTimeout(timeout time.Duration) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamXOptions.RecvTimeout = timeout
	}}
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
		o.StreamXOptions.StreamMWs = append(o.StreamXOptions.StreamMWs, smw)
	}}
}

func WithStreamRecvMiddleware(smw streamx.StreamRecvMiddleware) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamXOptions.StreamRecvMWs = append(o.StreamXOptions.StreamRecvMWs, smw)
	}}
}

func WithStreamSendMiddleware(smw streamx.StreamSendMiddleware) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamXOptions.StreamSendMWs = append(o.StreamXOptions.StreamSendMWs, smw)
	}}
}

func ConvertNativeClientOption(o internal_client.Option) Option {
	return Option{F: o.F}
}

func ConvertStreamXClientOption(o Option) internal_client.Option {
	return internal_client.Option{F: o.F}
}
