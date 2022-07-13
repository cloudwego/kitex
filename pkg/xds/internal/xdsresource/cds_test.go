package xdsresource

import (
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/xds/internal/testutil"
	v3clusterpb "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	"github.com/golang/protobuf/ptypes/any"
	"reflect"
	"testing"
)

func TestUnmarshalCDSError(t *testing.T) {
	tests := []struct {
		name         string
		rawResources []*any.Any
		want         map[string]*ClusterResource
		wantErr      bool
	}{
		{
			name:         "resource is nil",
			rawResources: nil,
			want:         map[string]*ClusterResource{},
			wantErr:      false,
		},
		{
			name: "incorrect resource type url",
			rawResources: []*any.Any{
				{TypeUrl: ListenerTypeUrl, Value: []byte{}},
			},
			want:    map[string]*ClusterResource{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalCDS(tt.rawResources)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalCDS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalCDS() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalCDSSuccess(t *testing.T) {
	clusterName := "test"
	edsName := "test_eds"
	rawResources := []*any.Any{
		testutil.MarshalAny(&v3clusterpb.Cluster{
			Name:                 "test",
			ClusterDiscoveryType: &v3clusterpb.Cluster_Type{Type: v3clusterpb.Cluster_EDS},
			EdsClusterConfig: &v3clusterpb.Cluster_EdsClusterConfig{
				ServiceName: "test_eds",
			},
			LbPolicy:       v3clusterpb.Cluster_ROUND_ROBIN,
			LoadAssignment: nil,
		}),
	}
	got, err := UnmarshalCDS(rawResources)
	test.Assert(t, err == nil)
	test.Assert(t, len(got) == 1)
	cluster := got[clusterName]
	test.Assert(t, cluster != nil)
	test.Assert(t, cluster.EndpointName == edsName)
	test.Assert(t, cluster.DiscoveryType == ClusterDiscoveryTypeEDS)
	test.Assert(t, cluster.LbPolicy == ClusterLbRoundRobin)
	test.Assert(t, cluster.InlineEndpoints == nil)
}
