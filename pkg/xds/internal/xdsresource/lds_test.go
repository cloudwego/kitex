package xdsresource

import (
	"github.com/cloudwego/kitex/internal/test"
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

func TestUnmarshalLDSHttpConnectionManager(t *testing.T) {
	rawResources := []*any.Any{
		MarshalAny(Listener1),
		MarshalAny(Listener2),
	}
	res, err := UnmarshalLDS(rawResources)
	test.Assert(t, err == nil)
	test.Assert(t, len(res) == 2)
	// rds
	lis1 := res[ReturnedLisName1]
	test.Assert(t, lis1 != nil)
	test.Assert(t, lis1.NetworkFilters != nil)
	test.Assert(t, lis1.NetworkFilters[0].RouteConfigName == RouteConfigName1)
	// inline route config
	lis2 := res[ReturnedLisName2]
	test.Assert(t, lis2 != nil)
	test.Assert(t, lis2.NetworkFilters != nil)
	inlineRcfg := lis2.NetworkFilters[0].InlineRouteConfig
	test.Assert(t, inlineRcfg != nil)
	test.Assert(t, inlineRcfg.HttpRouteConfig != nil)
}

func TestUnmarshallLDSThriftProxy(t *testing.T) {
	rawResources := []*any.Any{
		MarshalAny(Listener3),
	}
	res, err := UnmarshalLDS(rawResources)
	test.Assert(t, err == nil)
	test.Assert(t, len(res) == 1)
	lis := res[ReturnedLisName3]
	test.Assert(t, lis != nil)
	test.Assert(t, len(lis.NetworkFilters) == 2)
	f := func(filter *NetworkFilter) {
		test.Assert(t, filter.FilterType == NetworkFilterTypeThrift)
		tp := filter.InlineRouteConfig.ThriftRouteConfig
		test.Assert(t, tp != nil)
		test.Assert(t, len(tp.Routes) == 1)
		r := tp.Routes[0]
		test.Assert(t, r.Match != nil)
		test.Assert(t, r.Match.MatchPath("method") == true)
		for k, v := range map[string]string{"k1": "v1", "k2": "v2"} {
			test.Assert(t, r.Match.GetTags()[k] == v)
		}
		test.Assert(t, r.WeightedClusters != nil)
	}
	f(lis.NetworkFilters[0])
	f(lis.NetworkFilters[1])
}
