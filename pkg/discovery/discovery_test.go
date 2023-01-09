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

// Package discovery resolver and implements
package discovery

import (
	"context"
	"reflect"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
)

func TestDefaultDiff(t *testing.T) {
	type args struct {
		key  string
		prev Result
		next Result
	}
	tests := []struct {
		name  string
		args  args
		want  Change
		want1 bool
	}{
		{"change", args{"1", Result{
			Cacheable: false,
			Instances: []Instance{
				NewInstance("tcp", "1", 10, nil),
				NewInstance("tcp", "2", 10, nil),
				NewInstance("tcp", "3", 10, nil),
				NewInstance("tcp", "4", 10, nil),
			},
		}, Result{
			Cacheable: true,
			Instances: []Instance{
				NewInstance("tcp", "1", 10, nil),
				NewInstance("tcp", "2", 10, nil),
				NewInstance("tcp", "3", 10, nil),
				NewInstance("tcp", "5", 10, nil),
			},
		}}, Change{
			Result: Result{Instances: []Instance{
				NewInstance("tcp", "1", 10, nil),
				NewInstance("tcp", "2", 10, nil),
				NewInstance("tcp", "3", 10, nil),
				NewInstance("tcp", "5", 10, nil),
			}, CacheKey: "1", Cacheable: true},
			Added:   []Instance{NewInstance("tcp", "5", 10, nil)},
			Removed: []Instance{NewInstance("tcp", "4", 10, nil)},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := DefaultDiff(tt.args.key, tt.args.prev, tt.args.next)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultDiff() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("DefaultDiff() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestFilter(t *testing.T) {
	instances := []Instance{
		NewInstance("tcp", "1", 10, map[string]string{"tag": "1"}),
		NewInstance("tcp", "1", 10, nil),
		NewInstance("unix", "1", 10, map[string]string{"tag": "1"}),
	}
	var (
		filterFunc1 = func(ctx context.Context, instance []Instance) []Instance {
			var res []Instance
			for _, ins := range instance {
				if ins.Address().Network() == "tcp" {
					res = append(res, ins)
				}
			}
			return res
		}
		filterFunc2 = func(ctx context.Context, instance []Instance) []Instance {
			var res []Instance
			for _, ins := range instance {
				if v, ok := ins.Tag("tag"); ok && v == "1" {
					res = append(res, ins)
				}
			}
			return res
		}
	)

	var res []Instance
	filter := &InstanceFilter{}
	res = filter.Filter(context.Background(), instances)
	test.Assert(t, reflect.DeepEqual(res, instances))

	rule1 := &FilterRule{
		Name:  "mock_filter_rule1",
		Funcs: []FilterFunc{filterFunc1},
	}

	filter.Rule = rule1
	res = filter.Filter(context.Background(), instances)
	test.Assert(t, len(res) == 2)

	rule2 := &FilterRule{
		Name:  "mock_filter_rule2",
		Funcs: []FilterFunc{filterFunc2},
	}
	filter.Rule = rule2
	res = filter.Filter(context.Background(), instances)
	test.Assert(t, len(res) == 2)

	rule3 := &FilterRule{
		Name:  "mock_filter_rule3",
		Funcs: []FilterFunc{filterFunc1, filterFunc2},
	}
	filter.Rule = rule3
	res = filter.Filter(context.Background(), instances)
	test.Assert(t, len(res) == 1)
}
