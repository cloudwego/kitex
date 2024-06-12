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

package generator

import (
	"testing"

	"github.com/cloudwego/thriftgo/pkg/test"
)

func TestGetStreamingTransport(t *testing.T) {
	t.Run("grpc", func(t *testing.T) {
		tp, err := GetStreamingTransport("grpc")
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == GRPC)
	})
	t.Run("gRPC", func(t *testing.T) {
		tp, err := GetStreamingTransport("gRPC")
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == GRPC)
	})
	t.Run("ttheader", func(t *testing.T) {
		tp, err := GetStreamingTransport("ttheader")
		test.Assert(t, err == nil, err)
		test.Assert(t, tp == TTHeader)
	})
	t.Run("unknown", func(t *testing.T) {
		_, err := GetStreamingTransport("xxx")
		test.Assert(t, err != nil, err)
	})
}
