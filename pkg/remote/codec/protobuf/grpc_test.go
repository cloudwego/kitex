// Copyright 2023 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package protobuf

import (
	"testing"

	"github.com/bytedance/gopkg/lang/mcache"
)

func Test_mallocWithFirstByteZeroed(t *testing.T) {
	type args struct {
		size int
		data []byte
	}
	tests := []struct {
		name string
		args args
		want byte
	}{
		{
			name: "test_with_no_data",
			args: args{
				size: 4,
				data: nil,
			},
			want: 0,
		},
		{
			name: "test_with_zeroed_data",
			args: args{
				size: 8,
				data: []byte{0, 0, 0, 0, 0, 0, 0, 0},
			},
			want: 0,
		},
		{
			name: "test_with_non_zeroed_data",
			args: args{
				size: 16,
				data: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.data != nil {
				buf := mcache.Malloc(tt.args.size)
				copy(buf, tt.args.data)
				mcache.Free(buf)
			}
			if got := mallocWithFirstByteZeroed(tt.args.size); got[0] != tt.want {
				t.Errorf("mallocWithFirstByteZeroed() = %v, want %v", got, tt.want)
			}
		})
	}
}
