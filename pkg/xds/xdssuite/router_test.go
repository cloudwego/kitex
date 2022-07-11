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
		Match: &xdsresource.HTTPRouteMatch{
			Prefix: "/prefix",
		},
	}
	route1 = &xdsresource.Route{
		Match: &xdsresource.HTTPRouteMatch{
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
		Match: &xdsresource.HTTPRouteMatch{
			Path: svcName + "/" + method,
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
		HttpRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: nil,
		},
	}
	routeConfigPrefixNotMatched = &xdsresource.RouteConfigResource{
		HttpRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: []*xdsresource.VirtualHost{
				{
					Routes: []*xdsresource.Route{
						route0,
					},
				},
			},
		},
	}
	routeConfigDefaultPrefix = &xdsresource.RouteConfigResource{
		HttpRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: []*xdsresource.VirtualHost{
				{
					Routes: []*xdsresource.Route{
						route1,
					},
				},
			},
		},
	}
	routeConfigPathMatched = &xdsresource.RouteConfigResource{
		HttpRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: []*xdsresource.VirtualHost{
				{
					Routes: []*xdsresource.Route{
						route2,
					},
				},
			},
		},
	}
	routeConfigInOrder = &xdsresource.RouteConfigResource{
		HttpRouteConfig: &xdsresource.HTTPRouteConfig{
			VirtualHosts: []*xdsresource.VirtualHost{
				{
					Routes: []*xdsresource.Route{
						route1,
						route2,
					},
				},
			},
		},
	}
)

func Test_matchHTTPRoute(t *testing.T) {
	to := rpcinfo.NewEndpointInfo(svcName, method, nil, nil)
	ri := rpcinfo.NewRPCInfo(nil, to, rpcinfo.NewInvocation(svcName, method), nil, nil)

	var r *xdsresource.Route
	// not matched
	r = matchHTTPRoute(ri, routeConfigNil)
	test.Assert(t, r == nil)
	r = matchHTTPRoute(ri, routeConfigPrefixNotMatched)
	test.Assert(t, r == nil)

	// default prefix
	r = matchHTTPRoute(ri, routeConfigDefaultPrefix)
	test.Assert(t, r.WeightedClusters[0].Name == clusterName1)
	// path
	r = matchHTTPRoute(ri, routeConfigPathMatched)
	test.Assert(t, r != nil)
	test.Assert(t, r.WeightedClusters[0].Name == clusterName2)
	// test the order, return the first matched
	r = matchHTTPRoute(ri, routeConfigInOrder)
	test.Assert(t, r != nil)
	test.Assert(t, r.WeightedClusters[0].Name == clusterName1)
}
