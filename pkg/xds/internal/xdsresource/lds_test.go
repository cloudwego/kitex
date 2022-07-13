package xdsresource

import (
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/xds/internal/testutil"
	v3listenerpb "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	v3httppb "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"github.com/golang/protobuf/ptypes/any"
	"reflect"
	"testing"
)

func TestUnmarshalLDSError(t *testing.T) {
	tests := []struct {
		name         string
		rawResources []*any.Any
		want         map[string]*ListenerResource
		wantErr      bool
	}{
		{
			name:         "resource is nil",
			rawResources: nil,
			want:         map[string]*ListenerResource{},
			wantErr:      false,
		},
		{
			name: "incorrect resource type url",
			rawResources: []*any.Any{
				{TypeUrl: EndpointTypeUrl, Value: []byte{}},
			},
			want:    map[string]*ListenerResource{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalLDS(tt.rawResources)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalLDS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalLDS() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalLDSSuccess(t *testing.T) {
	listenerName := "listener"
	routeConfigName := "route_config"
	rawResources := []*any.Any{
		testutil.MarshalAny(&v3listenerpb.Listener{
			Name: listenerName,
			ApiListener: &v3listenerpb.ApiListener{
				ApiListener: testutil.MarshalAny(
					&v3httppb.HttpConnectionManager{
						RouteSpecifier: &v3httppb.HttpConnectionManager_Rds{
							Rds: &v3httppb.Rds{
								RouteConfigName: routeConfigName,
							},
						},
					},
				),
			},
		}),
	}
	res, err := UnmarshalLDS(rawResources)
	test.Assert(t, err == nil)
	test.Assert(t, len(res) == 1)
	listener := res[listenerName]
	test.Assert(t, listener != nil)
	test.Assert(t, listener.RouteConfigName == routeConfigName)
}
