package xdsresource

import (
	"fmt"

	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/proto"
	dnsProto "istio.io/istio/pkg/dns/proto"
)

type NDSResource struct {
	NameTable map[string][]string
}

func UnmarshalNDS(rawResources []*any.Any) (*NDSResource, error) {
	if len(rawResources) < 1 {
		return nil, fmt.Errorf("no NDS resource found in the response")
	}
	r := rawResources[0]
	if r.GetTypeUrl() != NameTableTypeUrl {
		return nil, fmt.Errorf("invalid nameTable resource type: %s", r.GetTypeUrl())
	}
	nt := &dnsProto.NameTable{}
	if err := proto.Unmarshal(r.GetValue(), nt); err != nil {
		return nil, fmt.Errorf("unmarshal NameTable failed: %s", err)
	}
	res := make(map[string][]string)
	for k, v := range nt.Table {
		res[k] = v.Ips
	}

	return &NDSResource{
		NameTable: res,
	}, nil
}
