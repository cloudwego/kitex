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
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/xds/internal/xdsresource"
	"github.com/cloudwego/kitex/transport"
)

const (
	RouterClusterKey         = "XDS_Route_Picked_Cluster"
	defaultTotalWeight int32 = 100
)

// RouteResult stores the result of route
type RouteResult struct {
	RPCTimeout    time.Duration
	ClusterPicked string
	// TODO: retry policy also in RDS
}

// XDSRouter is a router that uses xds to route the rpc call
type XDSRouter struct {
	manager XDSResourceManager
}

func NewXDSRouter() *XDSRouter {
	m := xdsResourceManager.getManager()
	if m == nil {
		panic("xds resource manager has not been initialized")
	}
	return &XDSRouter{
		manager: m,
	}
}

// Route routes the rpc call to a cluster based on the RPCInfo
func (r *XDSRouter) Route(ctx context.Context, ri rpcinfo.RPCInfo) (*RouteResult, error) {
	route, err := r.matchRoute(ctx, ri)
	// no matched route
	if err != nil {
		return nil, kerrors.ErrXDSRoute.WithCause(fmt.Errorf("no matched route for service %s", ri.To().ServiceName()))
	}
	// pick cluster from the matched route
	cluster := pickCluster(route)
	if cluster == "" {
		return nil, kerrors.ErrXDSRoute.WithCause(fmt.Errorf("no cluster selected"))
	}
	return &RouteResult{
		RPCTimeout:    route.Timeout,
		ClusterPicked: cluster,
	}, nil
}

// matchRoute gets the route config from xdsResourceManager
func (r *XDSRouter) matchRoute(ctx context.Context, ri rpcinfo.RPCInfo) (*xdsresource.Route, error) {
	// use serviceName as the listener name
	ln := ri.To().ServiceName()
	lds, err := r.manager.Get(ctx, xdsresource.ListenerType, ln)
	if err != nil {
		return nil, fmt.Errorf("get listener failed: %v", err)
	}
	lis := lds.(*xdsresource.ListenerResource)

	var thrift, http *xdsresource.NetworkFilter
	for _, f := range lis.NetworkFilters {
		if f.FilterType == xdsresource.NetworkFilterTypeHTTP {
			http = f
		}
		if f.FilterType == xdsresource.NetworkFilterTypeThrift {
			thrift = f
		}
	}
	// match thrift route first, only inline route is supported
	if ri.Config().TransportProtocol() != transport.GRPC {
		if thrift != nil && thrift.InlineRouteConfig != nil {
			r := matchThriftRoute(ri, thrift.InlineRouteConfig)
			if r != nil {
				return r, nil
			}
		}
	}
	if http == nil {
		return nil, fmt.Errorf("no http filter found in listener %s", ln)
	}
	// inline route config
	if http.InlineRouteConfig != nil {
		r := matchHTTPRoute(ri, http.InlineRouteConfig)
		if r != nil {
			return r, nil
		}
	}
	// Get the route config
	rds, err := r.manager.Get(ctx, xdsresource.RouteConfigType, http.RouteConfigName)
	if err != nil {
		return nil, fmt.Errorf("get route failed: %v", err)
	}
	rcfg := rds.(*xdsresource.RouteConfigResource)
	rt := matchHTTPRoute(ri, rcfg)
	if rt != nil {
		return rt, nil
	}
	return nil, fmt.Errorf("no matched route")
}

// matchHTTPRoute matches one http route
func matchHTTPRoute(ri rpcinfo.RPCInfo, routeConfig *xdsresource.RouteConfigResource) *xdsresource.Route {
	if rcfg := routeConfig.HttpRouteConfig; rcfg != nil {
		var svcName string
		if ri.Invocation().PackageName() == "" {
			svcName = ri.Invocation().ServiceName()
		} else {
			svcName = fmt.Sprintf("%s.%s", ri.Invocation().PackageName(), ri.Invocation().ServiceName())
		}
		// path defined in the virtual services should be ${serviceName}/${methodName}
		path := fmt.Sprintf("/%s/%s", svcName, ri.Invocation().MethodName())
		// match
		for _, vh := range rcfg.VirtualHosts {
			// skip the domain match now.

			// use the first matched route.
			for _, r := range vh.Routes {
				if routeMatched(path, ri.To(), r) {
					return r
				}
			}
		}
	}
	return nil
}

// matchThriftRoute matches one thrift route
func matchThriftRoute(ri rpcinfo.RPCInfo, routeConfig *xdsresource.RouteConfigResource) *xdsresource.Route {
	if rcfg := routeConfig.ThriftRouteConfig; rcfg != nil {
		for _, r := range rcfg.Routes {
			if routeMatched(ri.To().Method(), ri.To(), r) {
				return r
			}
		}
	}
	return nil
}

// routeMatched checks if the route matches the info provided in the RPCInfo
func routeMatched(path string, to rpcinfo.EndpointInfo, r *xdsresource.Route) bool {
	if r.Match != nil && r.Match.MatchPath(path) {
		// return r
		tagMatched := true
		for mk, mv := range r.Match.GetTags() {
			if v, ok := to.Tag(mk); !ok || v != mv {
				tagMatched = false
				break
			}
		}
		if tagMatched {
			return true
		}
	}
	return false
}

// pickCluster selects cluster based on the weight
func pickCluster(route *xdsresource.Route) string {
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
