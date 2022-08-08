package xdsresource

import (
	"encoding/json"
	"fmt"

	v3clusterpb "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/proto"
)

type ClusterDiscoveryType int

const (
	ClusterDiscoveryTypeEDS ClusterDiscoveryType = iota
	ClusterDiscoveryTypeLogicalDNS
	ClusterDiscoveryTypeStatic
)

type ClusterLbPolicy int

const (
	ClusterLbRoundRobin = iota
	ClusterLbRingHash
)

type ClusterResource struct {
	DiscoveryType   ClusterDiscoveryType
	LbPolicy        ClusterLbPolicy
	EndpointName    string
	InlineEndpoints *EndpointsResource
}

func (c *ClusterResource) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		DiscoveryType   string             `json:"discoveryType"`
		LbPolicy        string             `json:"lbPolicy"`
		EndpointName    string             `json:"endpointName"`
		InlineEndpoints *EndpointsResource `json:"meta,omitempty"`
	}{
		DiscoveryType:   c.ClusterType(),
		LbPolicy:        c.GetLbPolicy(),
		EndpointName:    c.EndpointName,
		InlineEndpoints: c.InlineEDS(),
	})
}

func (r *ClusterResource) ClusterType() string {
	switch r.DiscoveryType {
	case ClusterDiscoveryTypeEDS:
		return "EDS"
	case ClusterDiscoveryTypeLogicalDNS:
		return "LOGICAL_DNS"
	}
	return "Static"
}

func (r *ClusterResource) GetLbPolicy() string {
	switch r.LbPolicy {
	case ClusterLbRoundRobin:
		return "roundrobin"
	case ClusterLbRingHash:
		return "ringhash"
	}
	return ""
}

func (r *ClusterResource) InlineEDS() *EndpointsResource {
	return r.InlineEndpoints
}

func unmarshalCluster(r *any.Any) (string, *ClusterResource, error) {
	if r.GetTypeUrl() != ClusterTypeUrl {
		return "", nil, fmt.Errorf("invalid cluster resource type: %s", r.GetTypeUrl())
	}
	c := &v3clusterpb.Cluster{}
	if err := proto.Unmarshal(r.GetValue(), c); err != nil {
		return "", nil, fmt.Errorf("unmarshal cluster failed: %s", err)
	}
	// TODO: circult breaker and outlier detection
	ret := &ClusterResource{
		DiscoveryType: convertDiscoveryType(c.GetType()),
		LbPolicy:      convertLbPolicy(c.GetLbPolicy()),
		EndpointName:  c.Name,
	}
	if n := c.GetEdsClusterConfig().GetServiceName(); n != "" {
		ret.EndpointName = n
	}
	// inline eds
	inlineEndpoints, err := parseClusterLoadAssignment(c.GetLoadAssignment())
	if err != nil {
		return c.Name, nil, err
	}
	ret.InlineEndpoints = inlineEndpoints
	return c.Name, ret, nil
}

func UnmarshalCDS(rawResources []*any.Any) (map[string]*ClusterResource, error) {
	ret := make(map[string]*ClusterResource, len(rawResources))
	errMap := make(map[string]error)
	var errSlice []error
	for _, r := range rawResources {
		name, res, err := unmarshalCluster(r)
		if err != nil {
			if name == "" {
				errSlice = append(errSlice, err)
				continue
			}
			errMap[name] = err
			continue
		}
		ret[name] = res
	}
	if len(errMap) == 0 && len(errSlice) == 0 {
		return ret, nil
	}
	return ret, processUnmarshalErrors(errSlice, errMap)
}

func convertDiscoveryType(t v3clusterpb.Cluster_DiscoveryType) ClusterDiscoveryType {
	switch t {
	case v3clusterpb.Cluster_EDS:
		return ClusterDiscoveryTypeEDS
	case v3clusterpb.Cluster_LOGICAL_DNS:
		return ClusterDiscoveryTypeLogicalDNS
	case v3clusterpb.Cluster_STATIC:
		return ClusterDiscoveryTypeStatic
	}

	return ClusterDiscoveryTypeEDS
}

func convertLbPolicy(lb v3clusterpb.Cluster_LbPolicy) ClusterLbPolicy {
	switch lb {
	case v3clusterpb.Cluster_ROUND_ROBIN:
		return ClusterLbRoundRobin
	case v3clusterpb.Cluster_RING_HASH:
		return ClusterLbRingHash
	}

	return ClusterLbRoundRobin
}
