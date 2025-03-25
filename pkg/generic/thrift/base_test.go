/*
 * Copyright 2024 CloudWeGo Authors
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

package thrift

import (
	"reflect"
	"testing"
)

func TestMergeRequestBase(t *testing.T) {
	type args struct {
		jsonBase Base
		frkBase  Base
	}
	tests := []struct {
		name string
		args args
		want Base
	}{
		{
			name: "jsonBase.Extra is empty",
			args: args{
				jsonBase: Base{},
				frkBase:  Base{LogID: "1", Extra: map[string]string{"a": "1"}},
			},
			want: Base{LogID: "1", Extra: map[string]string{"a": "1"}},
		},
		{
			name: "frkBase is empty",
			args: args{
				jsonBase: Base{LogID: "2", Addr: "2", Extra: map[string]string{"a": "2", "b": "2"}},
				frkBase:  Base{},
			},
			want: Base{Extra: map[string]string{"a": "2", "b": "2"}},
		},
		{
			name: "jsonBase is not empty",
			args: args{
				jsonBase: Base{LogID: "2", Addr: "2", Extra: map[string]string{"a": "2", "b": "2"}},
				frkBase:  Base{LogID: "1", Extra: map[string]string{"a": "1", "c": "1"}},
			},
			want: Base{LogID: "1", Addr: "", Extra: map[string]string{"a": "1", "b": "2", "c": "1"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergeBase(tt.args.jsonBase, tt.args.frkBase); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeRequestBase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mergeBaseAny(t *testing.T) {
	type args struct {
		jsonBase interface{}
		frkBase  *Base
	}
	tests := []struct {
		name string
		args args
		want *Base
	}{
		{
			name: "jsonBase is nil",
			args: args{
				jsonBase: map[string]any(nil),
				frkBase:  &Base{LogID: "1", Extra: map[string]string{"a": "1"}},
			},
			want: &Base{LogID: "1", Extra: map[string]string{"a": "1"}},
		},
		{
			name: "frkBase is empty",
			args: args{
				jsonBase: map[string]any{"LogID": "2", "Addr": "2", "Extra": map[string]any{"a": "2", "b": "2"}},
				frkBase:  &Base{},
			},
			want: &Base{Extra: map[string]string{"a": "2", "b": "2"}},
		},
		{
			name: "jsonBase is not empty",
			args: args{
				jsonBase: map[string]any{"LogID": "2", "Addr": "2", "Extra": map[string]any{"a": "2", "b": "2"}},
				frkBase:  &Base{LogID: "1", Extra: map[string]string{"a": "1", "c": "1"}},
			},
			want: &Base{LogID: "1", Addr: "", Extra: map[string]string{"a": "1", "b": "2", "c": "1"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeBaseAny(tt.args.jsonBase, tt.args.frkBase); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeBaseAny() = %v, want %v", got, tt.want)
			}
		})
	}
}
