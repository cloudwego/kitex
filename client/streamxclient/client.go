package streamxclient

import (
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/streamx"
)

func NewClient(svcInfo *serviceinfo.ServiceInfo, opts ...ClientOption) (streamx.ClientStreamProvider, error) {
	iopts := make([]client.Option, 0, len(opts)+1)
	for _, opt := range opts {
		iopts = append(iopts, convertClientOption(opt))
	}
	c, err := client.NewClient(svcInfo, iopts...)
	if err != nil {
		return nil, err
	}
	return c.(streamx.ClientStreamProvider), nil
}
