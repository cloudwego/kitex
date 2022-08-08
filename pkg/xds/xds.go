package xds

import (
	"sync"

	"github.com/cloudwego/kitex/pkg/xds/internal/manager"
	"github.com/cloudwego/kitex/pkg/xds/xdssuite"
)

func newManager() (xdssuite.XDSResourceManager, error) {
	return manager.NewXDSResourceManager(nil)
}

var once sync.Once

// Init initializes the xds resource manager.
func Init() error {
	var err error
	once.Do(func() {
		err = xdssuite.BuildXDSResourceManager(newManager)
	})
	return err
}
