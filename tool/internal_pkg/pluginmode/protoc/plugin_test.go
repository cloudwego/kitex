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

package protoc

import (
	"testing"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/tool/internal_pkg/generator"
)

func Test_parseStreamingTransport(t *testing.T) {
	t.Run("grpc", func(t *testing.T) {
		comments := protogen.CommentSet{
			Leading: "// @streaming.transport=grpc",
		}
		tp, err := parseStreamingTransport(comments)
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == generator.GRPC)
	})
	t.Run("ttheader", func(t *testing.T) {
		comments := protogen.CommentSet{
			Leading: "// @streaming.transport=ttheader",
		}
		tp, err := parseStreamingTransport(comments)
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == generator.TTHeader)
	})
	t.Run("no-comment", func(t *testing.T) {
		comments := protogen.CommentSet{}
		tp, err := parseStreamingTransport(comments)
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == generator.GRPC)
	})
	t.Run("unknown", func(t *testing.T) {
		comments := protogen.CommentSet{
			Leading: "// @streaming.transport=xxx",
		}
		_, err := parseStreamingTransport(comments)
		test.Assert(t, err != nil, err)
	})
}
