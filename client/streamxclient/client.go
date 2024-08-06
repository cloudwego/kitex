package streamxclient

import (
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func NewClient(svcInfo *serviceinfo.ServiceInfo, opts ...ClientOption) (client.StreamNewer, error) {
	iopts := make([]client.Option, 0, len(opts)+1)
	for _, opt := range opts {
		iopts = append(iopts, convertClientOption(opt))
	}
	c, err := client.NewClient(svcInfo, iopts...)
	if err != nil {
		return nil, err
	}
	return c.(client.StreamNewer), nil
}
