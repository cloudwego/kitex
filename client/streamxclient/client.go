package streamxclient

import (
	"github.com/cloudwego/kitex/client"
	iclient "github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

type Client = client.StreamX

func NewClient(svcInfo *serviceinfo.ServiceInfo, opts ...Option) (Client, error) {
	iopts := make([]client.Option, 0, len(opts)+1)
	for _, opt := range opts {
		iopts = append(iopts, convertClientOption(opt))
	}
	nopts := iclient.NewOptions(iopts)
	c, err := client.NewClientWithOptions(svcInfo, nopts)
	if err != nil {
		return nil, err
	}
	return c.(client.StreamX), nil
}
