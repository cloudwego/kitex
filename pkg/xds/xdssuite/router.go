package xdssuite

import (
	"context"
	"fmt"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	"math/rand"
	"time"
)

const (
	RouterClusterKey         = "XDS_Route_Picked_Cluster"
	defaultTotalWeight int32 = 100
)

type RouteResult struct {
	RPCTimeout    time.Duration
	ClusterPicked string
	// TODO: retry policy also in RDS
}

type XDSRouter struct{}

func (r *XDSRouter) Route(ctx context.Context, ri rpcinfo.RPCInfo) (*RouteResult, error) {
	routeConfig, err := getRouteConfig(ctx, ri)
	if err != nil {
		return nil, err
	}

	matchedRoute := matchRoute(ri, routeConfig)
	// no matched route
	if matchedRoute == nil {
		return nil, kerrors.ErrRoute
	}
	cluster := selectCluster(matchedRoute)
	if cluster == "" {
		return nil, fmt.Errorf("no cluster selected")
	}
	return &RouteResult{
		RPCTimeout:    matchedRoute.Timeout,
		ClusterPicked: cluster,
	}, nil
}

// getRouteConfig gets the route config from xdsResourceManager
func getRouteConfig(ctx context.Context, ri rpcinfo.RPCInfo) (*xdsresource.RouteConfigResource, error) {
	m, err := getXdsResourceManager()
	if err != nil {
		return nil, err
	}
	listenerName := ri.To().ServiceName()
	lds, err := m.Get(ctx, xdsresource.ListenerType, listenerName)
	if err != nil {
		return nil, fmt.Errorf("get listener failed: %v", err)
	}
	listener := lds.(*xdsresource.ListenerResource)

	// inline route config
	if listener.InlineRouteConfig != nil {
		return listener.InlineRouteConfig, nil
	}
	// Get the route config
	routeConfigName := listener.RouteConfigName
	rds, err := m.Get(ctx, xdsresource.RouteConfigType, routeConfigName)
	if err != nil {
		return nil, fmt.Errorf("get route failed: %v", err)
	}
	routeConfig := rds.(*xdsresource.RouteConfigResource)
	return routeConfig, nil
}

// matchRoute matchs one route in the provided routeConfig based on information in RPCInfo
func matchRoute(ri rpcinfo.RPCInfo, routeConfig *xdsresource.RouteConfigResource) *xdsresource.Route {
	path := ri.To().Method()
	toService := ri.To().ServiceName()
	var matchedRoute *xdsresource.Route
	for _, vh := range routeConfig.VirtualHosts {
		if vh.Name != toService {
			continue
		}
		// match the first route
		for _, r := range vh.Routes {
			match := r.Match
			if match.Matched(path, nil) {
				matchedRoute = r
				break
			}
		}
	}
	return matchedRoute
}

// selectCluster selects cluster based on the weight
func selectCluster(route *xdsresource.Route) string {
	// handle weighted cluster
	wcs := route.WeightedClusters
	if len(wcs) == 0 {
		return ""
	}
	if len(wcs) == 1 {
		return wcs[0].Name
	}
	currWeight := uint32(0)
	targetWeight := uint32(rand.Int31n(defaultTotalWeight))
	for _, wc := range wcs {
		currWeight += wc.Weight
		if currWeight >= targetWeight {
			return wc.Name
		}
	}
	// total weight is less than target weight
	return ""
}
