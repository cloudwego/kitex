/*
 * Copyright 2026 CloudWeGo Authors
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

package clientstream

import (
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// GRPCForwardStream is a gRPC-compatible (streaming.Stream) adapter that forwards
// all message I/O to a CommonStream, so send/recv event tracing, BizStatusError
// surfacing and DoFinish are handled in exactly one place. It is used when the
// underlying stream exposes a gRPC stream but no dedicated gRPC send/recv
// middleware chain needs to run (e.g. the in-process LocalCaller path).
type GRPCForwardStream struct {
	streaming.Stream
	cs *CommonStream
}

var _ streaming.WithDoFinish = (*GRPCForwardStream)(nil)

// NewGRPCForwardStream wraps grpcStream, forwarding recv/send to cs.
func NewGRPCForwardStream(grpcStream streaming.Stream, cs *CommonStream) *GRPCForwardStream {
	return &GRPCForwardStream{Stream: grpcStream, cs: cs}
}

func (s *GRPCForwardStream) RecvMsg(m interface{}) error {
	return s.cs.RecvMsg(s.cs.ctx, m)
}

func (s *GRPCForwardStream) SendMsg(m interface{}) error {
	return s.cs.SendMsg(s.cs.ctx, m)
}

func (s *GRPCForwardStream) Header() (md metadata.MD, err error) {
	if md, err = s.Stream.Header(); err != nil {
		s.cs.DoFinish(err)
	}
	return
}

func (s *GRPCForwardStream) DoFinish(err error) {
	s.cs.DoFinish(err)
}
