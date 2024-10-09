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

package ttstream

import "github.com/cloudwego/kitex/pkg/streamx"

var _ ClientStreamMeta = (*clientStream)(nil)
var _ ServerStreamMeta = (*serverStream)(nil)

func (s *clientStream) Header() (streamx.Header, error) {
	sig := <-s.headerSig
	if sig < 0 {
		return nil, ErrClosedStream
	}
	return s.header, nil
}

func (s *clientStream) Trailer() (streamx.Trailer, error) {
	sig := <-s.trailerSig
	if sig < 0 {
		return nil, ErrClosedStream
	}
	return s.trailer, nil
}

func (s *serverStream) SetHeader(hd streamx.Header) error {
	s.writeHeader(hd)
	return nil
}

func (s *serverStream) SendHeader(hd streamx.Header) error {
	s.writeHeader(hd)
	return s.stream.sendHeader()
}

func (s *serverStream) SetTrailer(tl streamx.Trailer) error {
	return s.writeTrailer(tl)
}
