package xdssuite

import (
	"context"

	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
)

var (
	xdsResourceManager    XDSResourceManager
	newXDSResourceManager func() (XDSResourceManager, error)
)

// XDSResourceManager is the interface for the xds resource manager.
// Get() returns the resources according to the input resourceType and resourceName.
// Get() returns error when the fetching fails or the resource is not found in the latest update.
type XDSResourceManager interface {
	Get(ctx context.Context, resourceType xdsresource.ResourceType, resourceName string) (interface{}, error)
}

// BuildXDSResourceManager builds the XDSResourceManager using the input function.
func BuildXDSResourceManager(f func() (XDSResourceManager, error)) error {
	newXDSResourceManager = f
	m, err := newXDSResourceManager()
	if err != nil {
		return err
	}
	xdsResourceManager = m
	return nil
}
