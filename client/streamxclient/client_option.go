package streamxclient

import (
	"github.com/cloudwego/kitex/client"
	internal_client "github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/streamx"
)

type ClientOption internal_client.Option
type ClientOptions = internal_client.Options

func WithHostPorts(hostports ...string) ClientOption {
	return convertInternalClientOption(client.WithHostPorts(hostports...))
}

func WithDestService(destService string) ClientOption {
	return convertInternalClientOption(client.WithDestService(destService))
}

func WithProvider(pvd streamx.ClientProvider) ClientOption {
	return ClientOption{F: func(o *internal_client.Options, di *utils.Slice) {
		o.RemoteOpt.Provider = pvd
	}}
}

func WithStreamMiddleware(smw streamx.StreamMiddleware) ClientOption {
	return ClientOption{F: func(o *internal_client.Options, di *utils.Slice) {
		o.SMWs = append(o.SMWs, smw)
	}}
}

func WithStreamRecvMiddleware(smw streamx.StreamRecvMiddleware) ClientOption {
	return ClientOption{F: func(o *internal_client.Options, di *utils.Slice) {
		o.SRecvMWs = append(o.SRecvMWs, smw)
	}}
}

func WithStreamSendMiddleware(smw streamx.StreamSendMiddleware) ClientOption {
	return ClientOption{F: func(o *internal_client.Options, di *utils.Slice) {
		o.SSendMWs = append(o.SSendMWs, smw)
	}}
}

func convertInternalClientOption(o internal_client.Option) ClientOption {
	return ClientOption{F: o.F}
}

func convertClientOption(o ClientOption) internal_client.Option {
	return internal_client.Option{F: o.F}
}
