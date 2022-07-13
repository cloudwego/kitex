package xdsresource

import (
	"github.com/cloudwego/kitex/pkg/xds/internal/testutil"
	"github.com/cloudwego/thriftgo/pkg/test"
	v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3endpointpb "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"reflect"
	"strconv"
	"testing"
)

func TestUnmarshalEDSError(t *testing.T) {
	tests := []struct {
		name         string
		rawResources []*any.Any
		want         map[string]*EndpointsResource
		wantErr      bool
	}{
		{
			name:         "resource is nil",
			rawResources: nil,
			want:         map[string]*EndpointsResource{},
			wantErr:      false,
		},
		{
			name: "incorrect resource type url",
			rawResources: []*any.Any{
				{TypeUrl: ListenerTypeUrl, Value: []byte{}},
			},
			want:    map[string]*EndpointsResource{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalEDS(tt.rawResources)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalEDS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalEDS() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalEDSSuccess(t *testing.T) {
	claName := "test"
	addr := "127.0.0.1"
	port1, port2 := 8080, 8081
	weight1, weight2 := 50, 50
	rawResources := []*any.Any{
		testutil.MarshalAny(&v3endpointpb.ClusterLoadAssignment{
			ClusterName: claName,
			Endpoints: []*v3endpointpb.LocalityLbEndpoints{
				{
					LbEndpoints: []*v3endpointpb.LbEndpoint{
						{
							HostIdentifier: &v3endpointpb.LbEndpoint_Endpoint{
								Endpoint: &v3endpointpb.Endpoint{
									Address: &v3.Address{
										Address: &v3.Address_SocketAddress{
											SocketAddress: &v3.SocketAddress{
												Protocol: v3.SocketAddress_TCP,
												Address:  addr,
												PortSpecifier: &v3.SocketAddress_PortValue{
													PortValue: uint32(port1),
												},
											},
										},
									},
								},
							},
							LoadBalancingWeight: &wrapperspb.UInt32Value{
								Value: uint32(weight1),
							},
						},
						{
							HostIdentifier: &v3endpointpb.LbEndpoint_Endpoint{
								Endpoint: &v3endpointpb.Endpoint{
									Address: &v3.Address{
										Address: &v3.Address_SocketAddress{
											SocketAddress: &v3.SocketAddress{
												Protocol: v3.SocketAddress_TCP,
												Address:  addr,
												PortSpecifier: &v3.SocketAddress_PortValue{
													PortValue: uint32(port2),
												},
											},
										},
									},
								},
							},
							LoadBalancingWeight: &wrapperspb.UInt32Value{
								Value: uint32(weight2),
							},
						},
					},
				},
			},
		}),
	}
	got, err := UnmarshalEDS(rawResources)
	test.Assert(t, err == nil)
	test.Assert(t, len(got) == 1)
	cla := got[claName]
	test.Assert(t, cla != nil)
	test.Assert(t, len(cla.Localities) == 1)
	test.Assert(t, len(cla.Localities[0].Endpoints) == 2)
	e1 := cla.Localities[0].Endpoints[0]
	test.Assert(t, e1.Weight() == 50)
	test.Assert(t, e1.Addr().String() == addr+":"+strconv.Itoa(port1))
	e2 := cla.Localities[0].Endpoints[1]
	test.Assert(t, e2.Weight() == 50)
	test.Assert(t, e2.Addr().String() == addr+":"+strconv.Itoa(port2))
}
