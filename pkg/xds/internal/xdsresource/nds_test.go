package xdsresource

import (
	"github.com/cloudwego/kitex/internal/test"
	"github.com/golang/protobuf/ptypes/any"
	dnsProto "istio.io/istio/pkg/dns/proto"
	"reflect"
	"testing"
)

func TestUnmarshalNDSError(t *testing.T) {
	type args struct {
		rawResources []*any.Any
	}
	tests := []struct {
		name    string
		args    args
		want    *NDSResource
		wantErr bool
	}{
		{
			name: "no resource",
			args: args{
				nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "incorrect resource type url",
			args: args{
				[]*any.Any{
					{TypeUrl: ListenerTypeUrl, Value: []byte{}},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalNDS(tt.args.rawResources)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalNDS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalNDS() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnmarshalNDSSuccess(t *testing.T) {
	ln, cip := "listener1", "0.0.0.0"
	nt:= &dnsProto.NameTable{
		Table: map[string]*dnsProto.NameTable_NameInfo{
			"listener1": {
				Ips: []string{cip},
			},
		},
	}
	rawResources := []*any.Any{
		MarshalAny(nt),
	}
	res, err := UnmarshalNDS(rawResources)
	test.Assert(t, err == nil)
	test.Assert(t, res.NameTable != nil)
	ips := res.NameTable[ln]
	test.Assert(t, ips != nil && len(ips) == 1)
	test.Assert(t, ips[0] == cip)
}
