// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package thriftgo

import (
	"testing"

	"github.com/cloudwego/thriftgo/parser"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
)

func Test_parseStreamingTransport(t *testing.T) {
	t.Run("no-annotation", func(t *testing.T) {
		tp, err := parseStreamingTransport(nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == generator.GRPC, tp)
	})
	t.Run("wrong-annotation:no-value", func(t *testing.T) {
		anno := parser.Annotations{
			&parser.Annotation{
				Key:    generator.AnnotationKeyStreamingTransport,
				Values: []string{},
			},
		}
		_, err := parseStreamingTransport(anno)
		test.Assert(t, err != nil, err)
	})
	t.Run("wrong-annotation:multiple-values", func(t *testing.T) {
		anno := parser.Annotations{
			&parser.Annotation{
				Key:    generator.AnnotationKeyStreamingTransport,
				Values: []string{"1", "2"},
			},
		}
		_, err := parseStreamingTransport(anno)
		test.Assert(t, err != nil, err)
	})
	t.Run("wrong-annotation:wrong-value", func(t *testing.T) {
		anno := parser.Annotations{
			&parser.Annotation{
				Key:    generator.AnnotationKeyStreamingTransport,
				Values: []string{"xxx"},
			},
		}
		_, err := parseStreamingTransport(anno)
		test.Assert(t, err != nil, err)
	})
	t.Run("right-annotation:ttheader", func(t *testing.T) {
		anno := parser.Annotations{
			&parser.Annotation{
				Key:    generator.AnnotationKeyStreamingTransport,
				Values: []string{generator.TTHeader},
			},
		}
		tp, err := parseStreamingTransport(anno)
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == generator.TTHeader, tp)
	})
}

func Test_getCombineServiceStreamingTransport(t *testing.T) {
	t.Run("both-grpc", func(t *testing.T) {
		services := []*generator.ServiceInfo{
			{ServiceName: "A", StreamingTransport: generator.GRPC},
			{ServiceName: "B", StreamingTransport: generator.GRPC},
		}
		tp, err := getCombineServiceStreamingTransport(services)
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == generator.GRPC, tp)
	})
	t.Run("both-ttheader", func(t *testing.T) {
		services := []*generator.ServiceInfo{
			{ServiceName: "A", StreamingTransport: generator.TTHeader},
			{ServiceName: "B", StreamingTransport: generator.TTHeader},
		}
		tp, err := getCombineServiceStreamingTransport(services)
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == generator.TTHeader, tp)
	})
	t.Run("diff", func(t *testing.T) {
		services := []*generator.ServiceInfo{
			{ServiceName: "A", StreamingTransport: generator.GRPC},
			{ServiceName: "B", StreamingTransport: generator.TTHeader},
		}
		_, err := getCombineServiceStreamingTransport(services)
		test.Assert(t, err != nil, err)
	})
}
