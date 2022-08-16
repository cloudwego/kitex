package xds

import (
	"github.com/cloudwego/kitex/pkg/xds/internal/manager"
)

// WithXDSServerAddress specifies the address of the xDS server the manager will connect to.
func WithXDSServerAddress(address string) manager.Option {
	return manager.Option{
		F: func(o *manager.Options) {
			o.XDSSvrConfig.SvrAddr = address
		},
	}
}
