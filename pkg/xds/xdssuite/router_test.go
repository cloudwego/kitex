/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package xdssuite

import (
	"fmt"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
)

var (
	pkgName      = "pkg"
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
			Path: fmt.Sprintf("/%s.%s/%s", pkgName, svcName, method),
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
	ri := rpcinfo.NewRPCInfo(nil, to, rpcinfo.NewInvocation(svcName, method, pkgName), nil, nil)

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
