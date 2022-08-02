package xdsresource

import (
	"encoding/json"
	"fmt"
	v3routepb "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	matcherv3 "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/proto"
	"time"
)

// RouteConfigResource is used for routing
// HttpRouteConfig is the native http route config, which consists of a list of virtual hosts.
// ThriftRouteConfig is converted from the routeConfiguration in thrift proxy, which can only be configured in the listener filter
// For details of thrift_proxy, please refer to: https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/thrift_proxy/v3/route.proto#envoy-v3-api-msg-extensions-filters-network-thrift-proxy-v3-routeconfiguration
type RouteConfigResource struct {
	HttpRouteConfig   *HTTPRouteConfig
	ThriftRouteConfig *ThriftRouteConfig
}

type HTTPRouteConfig struct {
	VirtualHosts []*VirtualHost
}

type ThriftRouteConfig struct {
	Routes []*Route
}

type VirtualHost struct {
	Name   string
	Routes []*Route
}

type WeightedCluster struct {
	Name   string
	Weight uint32
}

type Route struct {
	Match            RouteMatch
	WeightedClusters []*WeightedCluster
	Timeout          time.Duration
}

type RouteMatch interface {
	MatchPath(path string) bool
	GetTags() map[string]string
}

type HTTPRouteMatch struct {
	Path   string
	Prefix string
	Tags   map[string]string
}

type ThriftRouteMatch struct {
	Method      string
	ServiceName string
	Tags        map[string]string
}

func (tm *ThriftRouteMatch) MatchPath(path string) bool {
	if tm.Method != "" && tm.Method != path {
		return false
	}
	return true
}

func (tm *ThriftRouteMatch) GetTags() map[string]string {
	return tm.Tags
}

func (rm *HTTPRouteMatch) MatchPath(path string) bool {
	if rm.Path != "" {
		return rm.Path == path
	}
	// default prefix
	return rm.Prefix == "/"
}

func (rm *HTTPRouteMatch) GetTags() map[string]string {
	return rm.Tags
}

func (r *Route) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Match            RouteMatch         `json:"Match"`
		WeightedClusters []*WeightedCluster `json:"WeightedClusters"`
		Timeout          string             `json:"Timeout"`
	}{
		Match:            r.Match,
		WeightedClusters: r.WeightedClusters,
		Timeout:          r.Timeout.String(),
	})
}

func unmarshalRoutes(rs []*v3routepb.Route) ([]*Route, error) {
	routes := make([]*Route, len(rs))
	for i, r := range rs {
		route := &Route{}
		routeMatch := &HTTPRouteMatch{}
		match := r.GetMatch()
		if match == nil {
			return nil, fmt.Errorf("no match in route %s\n", r.GetName())
		}
		// path match
		pathSpecifier := match.GetPathSpecifier()
		// only support exact match for path
		switch p := pathSpecifier.(type) {
		// default route if virtual service is not configured
		case *v3routepb.RouteMatch_Prefix:
			routeMatch.Prefix = p.Prefix
		case *v3routepb.RouteMatch_Path:
			routeMatch.Path = p.Path
			//default:
			//	return nil, fmt.Errorf("only support path match")
		}
		// header match
		tags := make(map[string]string)
		if hs := match.GetHeaders(); hs != nil {
			for _, h := range hs {
				var v string
				switch hm := h.GetHeaderMatchSpecifier().(type) {
				case *v3routepb.HeaderMatcher_StringMatch:
					switch p := hm.StringMatch.GetMatchPattern().(type) {
					case *matcherv3.StringMatcher_Exact:
						v = p.Exact
					}
				case *v3routepb.HeaderMatcher_ExactMatch:
					v = hm.ExactMatch
				}
				if v != "" {
					tags[h.Name] = v
				}
			}
		}
		routeMatch.Tags = tags
		route.Match = routeMatch
		// action
		action := r.GetAction()
		if action == nil {
			return nil, fmt.Errorf("no action in route %s\n", r.GetName())
		}
		switch a := action.(type) {
		case *v3routepb.Route_Route:
			switch cs := a.Route.GetClusterSpecifier().(type) {
			case *v3routepb.RouteAction_Cluster:
				route.WeightedClusters = []*WeightedCluster{
					{Name: cs.Cluster, Weight: 1},
				}
			case *v3routepb.RouteAction_WeightedClusters:
				wcs := cs.WeightedClusters
				clusters := make([]*WeightedCluster, len(wcs.Clusters))
				for j, wc := range wcs.GetClusters() {
					clusters[j] = &WeightedCluster{
						Name:   wc.GetName(),
						Weight: wc.GetWeight().GetValue(),
					}
				}
				route.WeightedClusters = clusters
			}
			route.Timeout = a.Route.GetTimeout().AsDuration()
		}
		routes[i] = route
	}
	return routes, nil
}

// unmarshalRouteConfig unmarshalls the RouteConfigResource, which only support HTTP route.
func unmarshalRouteConfig(routeConfig *v3routepb.RouteConfiguration) (*RouteConfigResource, error) {
	vhs := routeConfig.GetVirtualHosts()
	virtualHosts := make([]*VirtualHost, len(vhs))
	for i := 0; i < len(vhs); i++ {
		rs := vhs[i].GetRoutes()
		routes, err := unmarshalRoutes(rs)
		if err != nil {
			return nil, fmt.Errorf("processing route in virtual host %s failed: %s\n", vhs[i].GetName(), err)
		}
		virtualHost := &VirtualHost{
			Name:   vhs[i].GetName(),
			Routes: routes,
		}
		virtualHosts[i] = virtualHost
	}
	return &RouteConfigResource{
		HttpRouteConfig: &HTTPRouteConfig{virtualHosts},
	}, nil
}

func UnmarshalRDS(rawResources []*any.Any) (map[string]*RouteConfigResource, error) {
	ret := make(map[string]*RouteConfigResource, len(rawResources))
	errMap := make(map[string]error)
	var errSlice []error
	for _, r := range rawResources {
		if r.GetTypeUrl() != RouteTypeUrl {
			err := fmt.Errorf("invalid route config resource type: %s\n", r.GetTypeUrl())
			errSlice = append(errSlice, err)
			continue
		}
		rcfg := &v3routepb.RouteConfiguration{}
		if err := proto.Unmarshal(r.GetValue(), rcfg); err != nil {
			err = fmt.Errorf("unmarshal failed: %s\n", err)
			errSlice = append(errSlice, err)
			continue
		}
		res, err := unmarshalRouteConfig(rcfg)
		if err != nil {
			errMap[rcfg.Name] = err
			continue
		}
		ret[rcfg.Name] = res
	}
	if len(errMap) == 0 && len(errSlice) == 0 {
		return ret, nil
	}
	return ret, processUnmarshalErrors(errSlice, errMap)
}
