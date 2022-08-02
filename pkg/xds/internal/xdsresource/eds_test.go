package xdsresource

import (
	"github.com/cloudwego/thriftgo/pkg/test"
	"github.com/golang/protobuf/ptypes/any"
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
	//edsAddr := "127.0.0.1"
	//edsPort1, edsPort2 := 8080, 8081
	//edsWeight1, edsWeight2 := 50, 50
	rawResources := []*any.Any{
		MarshalAny(Endpoints1),
	}
	got, err := UnmarshalEDS(rawResources)
	test.Assert(t, err == nil)
	test.Assert(t, len(got) == 1)
	cla := got[EndpointName1]
	test.Assert(t, cla != nil)
	test.Assert(t, len(cla.Localities) == 1)
	test.Assert(t, len(cla.Localities[0].Endpoints) == 2)
	e1 := cla.Localities[0].Endpoints[0]
	test.Assert(t, e1.Weight() == 50)
	test.Assert(t, e1.Addr().String() == edsAddr+":"+strconv.Itoa(edsPort1))
	e2 := cla.Localities[0].Endpoints[1]
	test.Assert(t, e2.Weight() == 50)
	test.Assert(t, e2.Addr().String() == edsAddr+":"+strconv.Itoa(edsPort2))
}
