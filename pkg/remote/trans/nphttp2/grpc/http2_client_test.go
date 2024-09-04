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

package grpc

import (
	"context"
	"math"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"golang.org/x/net/http2"
)

func TestHttp2ClientTrailerRace(t *testing.T) {
	_, ct := setUp(t, 0, math.MaxUint32, normal)
	s, err := ct.NewStream(context.Background(), &CallHdr{})
	test.Assert(t, err == nil, err)

	go func() {
		s.Trailer()
	}()
	go func() {
		mdata := metadata.Pairs("k", "v")
		ct.closeStream(s, nil, false, http2.ErrCodeNo, status.New(codes.OK, ""), mdata, true)
	}()
}
