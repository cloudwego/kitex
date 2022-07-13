package xdsresource

import (
	"fmt"
	v3listenerpb "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	v3httppb "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/proto"
)

type ListenerResource struct {
	RouteConfigName   string
	InlineRouteConfig *RouteConfigResource
}

func unmarshallApiListener(apiListener *v3listenerpb.ApiListener) (*ListenerResource, error) {
	if apiListener.GetApiListener() == nil {
		return nil, fmt.Errorf("apiListener is empty")
	}
	// unmarshal listener
	if apiListener.GetApiListener().GetTypeUrl() != HTTPConnManagerTypeUrl {
		return nil, fmt.Errorf("invalid apiListener type: %s", apiListener.GetApiListener().GetTypeUrl())
	}
	apiLis := &v3httppb.HttpConnectionManager{}
	if err := proto.Unmarshal(apiListener.GetApiListener().GetValue(), apiLis); err != nil {
		return nil, fmt.Errorf("unmarshal api listener failed: %s", err)
	}
	// convert listener
	// 1. RDS
	// 2. inline route config
	var res *ListenerResource
	switch apiLis.RouteSpecifier.(type) {
	case *v3httppb.HttpConnectionManager_Rds:
		if apiLis.GetRds() == nil {
			return nil, fmt.Errorf("no Rds in the apiListener")
		}
		if apiLis.GetRds().GetRouteConfigName() == "" {
			return nil, fmt.Errorf("no route config Name")
		}
		res = &ListenerResource{RouteConfigName: apiLis.GetRds().GetRouteConfigName()}
	case *v3httppb.HttpConnectionManager_RouteConfig:
		if rcfg := apiLis.GetRouteConfig(); rcfg == nil {
			return nil, fmt.Errorf("no inline route config")
		} else {
			inlineRouteConfig, err := unmarshalRouteConfig(rcfg)
			if err != nil {
				return nil, err
			}
			res = &ListenerResource{
				RouteConfigName:   apiLis.GetRouteConfig().GetName(),
				InlineRouteConfig: inlineRouteConfig,
			}
		}
	}
	return res, nil
}

func UnmarshalLDS(rawResources []*any.Any) (map[string]*ListenerResource, error) {
	ret := make(map[string]*ListenerResource, len(rawResources))
	errMap := make(map[string]error)
	var errSlice []error

	for _, r := range rawResources {
		if r.GetTypeUrl() != ListenerTypeUrl {
			err := fmt.Errorf("invalid listener resource type: %s\n", r.GetTypeUrl())
			errSlice = append(errSlice, err)
			continue
		}
		lis := &v3listenerpb.Listener{}
		if err := proto.Unmarshal(r.GetValue(), lis); err != nil {
			err = fmt.Errorf("unmarshal Listener failed, error=%s\n", err)
			errSlice = append(errSlice, fmt.Errorf("unmarshal Listener failed, error=%s\n", err))
			continue
		}
		ret[lis.Name] = &ListenerResource{}
		// http listener
		if apiListener := lis.GetApiListener(); apiListener != nil {
			res, err := unmarshallApiListener(apiListener)
			if err != nil {
				errMap[lis.Name] = err
				continue
			}
			ret[lis.Name] = res
		}
	}
	if len(errMap) == 0 && len(errSlice) == 0 {
		return ret, nil
	}
	return ret, processUnmarshalErrors(errSlice, errMap)
}
