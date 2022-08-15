/*
 * Copyright 2021 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package xdsresource

import (
	"reflect"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/golang/protobuf/ptypes/any"
	dnsProto "istio.io/istio/pkg/dns/proto"
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
	nt := &dnsProto.NameTable{
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
	test.Assert(t, len(ips) == 1)
	test.Assert(t, ips[0] == cip)
}
