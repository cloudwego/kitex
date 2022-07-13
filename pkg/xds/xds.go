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
	// TODO: add some init process to subscribe xds resource.
	// Load ENV on server-side?
	err := xdssuite.BuildXDSResourceManager(newManager)
	return err
}
