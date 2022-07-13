package xdssuite

import (
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	"testing"
)

var (
	svcName      = "svc"
	method       = "method"
	clusterName1 = "cluster1"
	clusterName2 = "cluster2"
	route0       = &xdsresource.Route{
		Match: &xdsresource.RouteMatch{
			Prefix: "/p",
		},
	}
	route1 = &xdsresource.Route{
		Match: &xdsresource.RouteMatch{
			Prefix: "/",
		},
		WeightedClusters: []*xdsresource.WeightedCluster{
			{
				Name:   clusterName1,
				Weight: 100,
			},
		},
		Timeout: 0,
	}
	route2 = &xdsresource.Route{
		Match: &xdsresource.RouteMatch{
			Path: method,
		},
		WeightedClusters: []*xdsresource.WeightedCluster{
			{
				Name:   clusterName2,
				Weight: 100,
			},
		},
		Timeout: 0,
	}
	routeConfigNil = &xdsresource.RouteConfigResource{
		VirtualHosts: nil,
	}
	routeConfigPrefixNotMatched = &xdsresource.RouteConfigResource{
		VirtualHosts: []*xdsresource.VirtualHost{
			{
				Name: svcName,
				Routes: []*xdsresource.Route{
					route0,
				},
			},
		},
	}
	routeConfigDefaultPrefix = &xdsresource.RouteConfigResource{
		VirtualHosts: []*xdsresource.VirtualHost{
			{
				Name: svcName,
				Routes: []*xdsresource.Route{
					route1,
				},
			},
		},
	}
	routeConfigPathMatched = &xdsresource.RouteConfigResource{
		VirtualHosts: []*xdsresource.VirtualHost{
			{
				Name: svcName,
				Routes: []*xdsresource.Route{
					route2,
				},
			},
		},
	}
	routeConfigInOrder = &xdsresource.RouteConfigResource{
		VirtualHosts: []*xdsresource.VirtualHost{
			{
				Name: svcName,
				Routes: []*xdsresource.Route{
					route1,
					route2,
				},
			},
		},
	}
)

func Test_matchRoute(t *testing.T) {
	to := rpcinfo.NewEndpointInfo(svcName, method, nil, nil)
	ri := rpcinfo.NewRPCInfo(nil, to, nil, nil, nil)

	var r *xdsresource.Route
	// not matched
	r = matchRoute(ri, routeConfigNil)
	test.Assert(t, r == nil)
	r = matchRoute(ri, routeConfigPrefixNotMatched)
	test.Assert(t, r == nil)

	// default prefix
	r = matchRoute(ri, routeConfigDefaultPrefix)
	test.Assert(t, r.WeightedClusters[0].Name == clusterName1)
	// path
	r = matchRoute(ri, routeConfigPathMatched)
	test.Assert(t, r.WeightedClusters[0].Name == clusterName2)
	// test the order, return the first matched
	r = matchRoute(ri, routeConfigInOrder)
	test.Assert(t, r.WeightedClusters[0].Name == clusterName1)
}
