package xds

import (
	"github.com/cloudwego/kitex/pkg/xds/internal/manager"
	"github.com/cloudwego/kitex/pkg/xds/xdssuite"
)

func newManager() (xdssuite.XDSResourceManager, error) {
	return manager.NewXDSResourceManager(nil)
}

// Init initializes the xds resource manager.
func Init() error {
	return xdssuite.BuildXDSResourceManager(newManager)
}
