package xdsresource

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/cloudwego/kitex/pkg/utils"
	v3endpointpb "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/proto"
)

type Endpoint struct {
	addr   net.Addr
	weight uint32
	meta   map[string]string // Tag in discovery.instance.tag
}

func (e *Endpoint) Addr() net.Addr {
	return e.addr
}

func (e *Endpoint) Weight() uint32 {
	return e.weight
}

func (e *Endpoint) Meta() map[string]string {
	return e.meta
}

func (e *Endpoint) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Addr   string            `json:"edsAddr"`
		Weight uint32            `json:"Weight"`
		Meta   map[string]string `json:"meta,omitempty"`
	}{
		Addr:   e.addr.String(),
		Weight: e.weight,
		Meta:   e.meta,
	})
}

func (e *Endpoint) Tag(key string) (value string, exist bool) {
	value, exist = e.meta[key]
	return
}

type Locality struct {
	Endpoints []*Endpoint
	// TODO: do not support locality priority yet
}

type EndpointsResource struct {
	Localities []*Locality
}

func parseClusterLoadAssignment(cla *v3endpointpb.ClusterLoadAssignment) (*EndpointsResource, error) {
	if cla == nil || len(cla.GetEndpoints()) == 0 {
		return nil, nil
	}
	localities := make([]*Locality, len(cla.GetEndpoints()))
	for idx1, leps := range cla.GetEndpoints() {
		eps := make([]*Endpoint, len(leps.GetLbEndpoints()))
		for idx2, ep := range leps.GetLbEndpoints() {
			addr := ep.GetEndpoint().GetAddress().GetSocketAddress()
			weight := ep.GetLoadBalancingWeight()
			eps[idx2] = &Endpoint{
				addr: utils.NewNetAddr("tcp",
					net.JoinHostPort(addr.GetAddress(), strconv.Itoa(int(addr.GetPortValue())))),
				weight: weight.GetValue(),
				meta:   nil,
			}
			// TODO: add healthcheck
		}
		localities[idx1] = &Locality{
			Endpoints: eps,
		}
	}
	return &EndpointsResource{
		Localities: localities,
	}, nil
}

func UnmarshalEDS(rawResources []*any.Any) (map[string]*EndpointsResource, error) {
	ret := make(map[string]*EndpointsResource, len(rawResources))
	errMap := make(map[string]error)
	var errSlice []error
	for _, r := range rawResources {
		if r.GetTypeUrl() != EndpointTypeUrl {
			err := fmt.Errorf("invalid endpoint resource type: %s", r.GetTypeUrl())
			errSlice = append(errSlice, err)
			continue
		}
		cla := &v3endpointpb.ClusterLoadAssignment{}
		if err := proto.Unmarshal(r.GetValue(), cla); err != nil {
			err = fmt.Errorf("unmarshal ClusterLoadAssignment failed, error=%s", err)
			errSlice = append(errSlice, err)
			continue
		}
		endpoints, err := parseClusterLoadAssignment(cla)
		if err != nil {
			errMap[cla.ClusterName] = err
			continue
		}
		ret[cla.ClusterName] = endpoints
	}
	if len(errMap) == 0 && len(errSlice) == 0 {
		return ret, nil
	}
	return ret, processUnmarshalErrors(errSlice, errMap)
}
