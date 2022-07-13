package xdssuite

import (
	"context"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
)

type XDSResourceManager interface {
	Get(ctx context.Context, resourceType xdsresource.ResourceType, resourceName string) (interface{}, error)
	Dump()
}

var (
	manager               XDSResourceManager
	newXDSResourceManager func() (XDSResourceManager, error)
)

func BuildXDSResourceManager(f func() (XDSResourceManager, error)) error {
	newXDSResourceManager = f
	m, err := newXDSResourceManager()
	if err != nil {
		return err
	}
	manager = m
	return nil
}

func getXdsResourceManager() (XDSResourceManager, error) {
	if manager != nil {
		return manager, nil
	}

	m, err := newXDSResourceManager()
	manager = m
	return manager, err
}
