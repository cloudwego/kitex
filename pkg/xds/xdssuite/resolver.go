package xdssuite

import (
	"context"
	"fmt"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
)

type XDSResolver struct {
}

// Target should return a description for the given target that is suitable for being a key for cache.
func (r *XDSResolver) Target(ctx context.Context, target rpcinfo.EndpointInfo) (description string) {
	dest, ok := target.Tag(RouterClusterKey)
	// TODO: if no cluster is specified, the discovery will fail
	if !ok {
		return target.ServiceName()
	}
	return dest
}

// Resolve returns a list of instances for the given description of a target.
func (r *XDSResolver) Resolve(ctx context.Context, desc string) (discovery.Result, error) {
	eps, err := getEndpoints(ctx, desc)
	if err != nil {
		return discovery.Result{}, kerrors.ErrServiceDiscovery.WithCause(err)
	}
	instances := make([]discovery.Instance, len(eps))
	for i, e := range eps {
		instances[i] = discovery.NewInstance(e.Addr().Network(), e.Addr().String(), int(e.Weight()), e.Meta())
	}
	res := discovery.Result{
		Cacheable: false,
		CacheKey:  "",
		Instances: instances,
	}
	return res, nil
}

// getEndpoints gets endpoints for this desc from xdsResourceManager
func getEndpoints(ctx context.Context, desc string) ([]*xdsresource.Endpoint, error) {
	m, err := getXdsResourceManager()
	if err != nil {
		return nil, err
	}
	res, err := m.Get(ctx, xdsresource.ClusterType, desc)
	if err != nil {
		return nil, err
	}
	cluster := res.(*xdsresource.ClusterResource)
	var endpoints *xdsresource.EndpointsResource
	if cluster.InlineEndpoints != nil {
		endpoints = cluster.InlineEndpoints
	} else {
		resource, err := m.Get(ctx, xdsresource.EndpointsType, cluster.EndpointName)
		if err != nil {
			return nil, err
		}
		cla := resource.(*xdsresource.EndpointsResource)
		endpoints = cla
	}

	if endpoints == nil || len(endpoints.Localities) == 0 {
		return nil, fmt.Errorf("no endpoints for cluster: %s", desc)
	}
	// TODO: filter localities
	return endpoints.Localities[0].Endpoints, nil
}

func (r *XDSResolver) Diff(cacheKey string, prev, next discovery.Result) (discovery.Change, bool) {
	return discovery.Change{}, false
}

// Name returns the name of the resolver.
func (r *XDSResolver) Name() string {
	return "xdsResolver"
}
