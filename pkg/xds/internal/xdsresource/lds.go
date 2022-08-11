package xdsresource

import (
	"fmt"

	v3listenerpb "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	v3routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	v3httppb "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	v3thrift_proxy "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/thrift_proxy/v3"
	v3matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/proto"
)

// ListenerResource is the resource parsed from the lds response.
// Only includes NetworkFilters now
type ListenerResource struct {
	NetworkFilters []*NetworkFilter
}

type NetworkFilterType int

const (
	NetworkFilterTypeHTTP NetworkFilterType = iota
	NetworkFilterTypeThrift
)

// NetworkFilter contains the network filter config we need got from Listener.FilterChains.
// Only support HttpConnectionManager and ThriftProxy.
// If InlineRouteConfig is not nil, use it to perform routing.
// Or, use RouteConfigName as the resourceName to get the RouteConfigResource.
type NetworkFilter struct {
	FilterType        NetworkFilterType
	RouteConfigName   string
	InlineRouteConfig *RouteConfigResource
}

// UnmarshalLDS unmarshalls the LDS Response.
// Only focus on OutboundListener now.
// Get the InlineRouteConfig or RouteConfigName from the listener.
func UnmarshalLDS(rawResources []*any.Any) (map[string]*ListenerResource, error) {
	ret := make(map[string]*ListenerResource, len(rawResources))
	errMap := make(map[string]error)
	var errSlice []error

	for _, r := range rawResources {
		if r.GetTypeUrl() != ListenerTypeUrl {
			err := fmt.Errorf("invalid listener resource type: %s", r.GetTypeUrl())
			errSlice = append(errSlice, err)
			continue
		}
		lis := &v3listenerpb.Listener{}
		if err := proto.Unmarshal(r.GetValue(), lis); err != nil {
			err = fmt.Errorf("unmarshal Listener failed, error=%s", err)
			errSlice = append(errSlice, fmt.Errorf("unmarshal Listener failed, error=%s", err))
			continue
		}
		// Process Network Filters
		// Process both FilterChains and DefaultFilterChain
		var filtersErr []error
		var nfs []*NetworkFilter
		if fcs := lis.FilterChains; fcs != nil {
			for _, fc := range fcs {
				res, err := unmarshalFilterChain(fc)
				if err != nil {
					filtersErr = append(filtersErr, err)
					continue
				}
				nfs = append(nfs, res...)
			}
		}
		if fc := lis.DefaultFilterChain; fc != nil {
			res, err := unmarshalFilterChain(fc)
			if err != nil {
				filtersErr = append(filtersErr, err)
			}
			nfs = append(nfs, res...)
		}
		ret[lis.Name] = &ListenerResource{
			NetworkFilters: nfs,
		}
		if len(filtersErr) > 0 {
			errMap[lis.Name] = combineErrors(filtersErr)
		}
	}
	if len(errMap) == 0 && len(errSlice) == 0 {
		return ret, nil
	}
	return ret, processUnmarshalErrors(errSlice, errMap)
}

// unmarshalFilterChain unmarshalls the filter chain.
// Only process HttpConnectionManager and ThriftProxy now.
func unmarshalFilterChain(fc *v3listenerpb.FilterChain) ([]*NetworkFilter, error) {
	var filters []*NetworkFilter
	var errSlice []error
	for _, f := range fc.Filters {
		switch cfgType := f.GetConfigType().(type) {
		case *v3listenerpb.Filter_TypedConfig:
			if cfgType.TypedConfig.TypeUrl == ThriftProxyTypeUrl {
				r, err := unmarshalThriftProxy(cfgType.TypedConfig)
				if err != nil {
					errSlice = append(errSlice, err)
					continue
				}
				filters = append(filters, &NetworkFilter{
					FilterType:        NetworkFilterTypeThrift,
					InlineRouteConfig: r,
				})
			}
			if cfgType.TypedConfig.TypeUrl == HTTPConnManagerTypeUrl {
				n, r, err := unmarshallHttpConnectionManager(cfgType.TypedConfig)
				if err != nil {
					errSlice = append(errSlice, err)
					continue
				}
				filters = append(filters, &NetworkFilter{
					FilterType:        NetworkFilterTypeHTTP,
					RouteConfigName:   n,
					InlineRouteConfig: r,
				})
			}
		}
	}
	return filters, combineErrors(errSlice)
}

func unmarshalThriftProxy(rawResources *any.Any) (*RouteConfigResource, error) {
	tp := &v3thrift_proxy.ThriftProxy{}
	if err := proto.Unmarshal(rawResources.GetValue(), tp); err != nil {
		return nil, fmt.Errorf("unmarshal HttpConnectionManager failed: %s", err)
	}
	rs := tp.RouteConfig.GetRoutes()
	routes := make([]*Route, len(rs))
	for i, r := range rs {
		route := &Route{}
		routeMatch := &ThriftRouteMatch{}
		match := r.GetMatch()
		if match == nil {
			return nil, fmt.Errorf("no match in routeConfig:%s", tp.RouteConfig.Name)
		}
		// method or service match
		switch t := match.GetMatchSpecifier().(type) {
		case *v3thrift_proxy.RouteMatch_MethodName:
			routeMatch.Method = t.MethodName
		case *v3thrift_proxy.RouteMatch_ServiceName:
			routeMatch.ServiceName = t.ServiceName
		}
		// header match
		tags := make(map[string]string)
		if hs := match.GetHeaders(); hs != nil {
			for _, h := range hs {
				var v string
				switch hm := h.GetHeaderMatchSpecifier().(type) {
				case *v3routepb.HeaderMatcher_StringMatch:
					switch p := hm.StringMatch.GetMatchPattern().(type) {
					case *v3matcher.StringMatcher_Exact:
						v = p.Exact
					}
				}
				if v != "" {
					tags[h.Name] = v
				}
			}
		}
		routeMatch.Tags = tags
		route.Match = routeMatch
		// action
		action := r.Route
		if action == nil {
			return nil, fmt.Errorf("no action in routeConfig:%s", tp.RouteConfig.Name)
		}
		switch cs := action.GetClusterSpecifier().(type) {
		case *v3thrift_proxy.RouteAction_Cluster:
			route.WeightedClusters = []*WeightedCluster{
				{Name: cs.Cluster, Weight: 1},
			}
		case *v3thrift_proxy.RouteAction_WeightedClusters:
			wcs := cs.WeightedClusters
			clusters := make([]*WeightedCluster, len(wcs.Clusters))
			for i, wc := range wcs.GetClusters() {
				clusters[i] = &WeightedCluster{
					Name:   wc.GetName(),
					Weight: wc.GetWeight().GetValue(),
				}
			}
			route.WeightedClusters = clusters
		}
		routes[i] = route
	}
	return &RouteConfigResource{
		ThriftRouteConfig: &ThriftRouteConfig{Routes: routes},
	}, nil
}

func unmarshallHttpConnectionManager(rawResources *any.Any) (string, *RouteConfigResource, error) {
	httpConnMng := &v3httppb.HttpConnectionManager{}

	if err := proto.Unmarshal(rawResources.GetValue(), httpConnMng); err != nil {
		return "", nil, fmt.Errorf("unmarshal HttpConnectionManager failed: %s", err)
	}
	// convert listener
	// 1. RDS
	// 2. inline route config
	switch httpConnMng.RouteSpecifier.(type) {
	case *v3httppb.HttpConnectionManager_Rds:
		if httpConnMng.GetRds() == nil {
			return "", nil, fmt.Errorf("no Rds in the apiListener")
		}
		if httpConnMng.GetRds().GetRouteConfigName() == "" {
			return "", nil, fmt.Errorf("no route config Name")
		}
		return httpConnMng.GetRds().GetRouteConfigName(), nil, nil
	case *v3httppb.HttpConnectionManager_RouteConfig:
		if rcfg := httpConnMng.GetRouteConfig(); rcfg == nil {
			return "", nil, fmt.Errorf("no inline route config")
		} else {
			inlineRouteConfig, err := unmarshalRouteConfig(rcfg)
			if err != nil {
				return "", nil, err
			}
			return httpConnMng.GetRouteConfig().GetName(), inlineRouteConfig, nil
		}
	}
	return "", nil, nil
}
