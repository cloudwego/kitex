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
	rawResources := []*any.Any{
		MarshalAny(Cluster1),
	}
	got, err := UnmarshalCDS(rawResources)
	test.Assert(t, err == nil)
	test.Assert(t, len(got) == 1)
	cluster := got[ClusterName1]
	test.Assert(t, cluster != nil)
	test.Assert(t, cluster.EndpointName == EndpointName1)
	test.Assert(t, cluster.DiscoveryType == ClusterDiscoveryTypeEDS)
	test.Assert(t, cluster.LbPolicy == ClusterLbRoundRobin)
	test.Assert(t, cluster.InlineEndpoints == nil)
}
