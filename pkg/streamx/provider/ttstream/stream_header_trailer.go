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

var _ ClientStreamMeta = (*clientStream)(nil)
var _ ServerStreamMeta = (*serverStream)(nil)

func (s *clientStream) Header() (Header, error) {
	<-s.headerSig
	return s.header, nil
}

func (s *clientStream) Trailer() (Trailer, error) {
	<-s.trailerSig
	return s.trailer, nil
}

func (s *serverStream) SetHeader(hd Header) error {
	return s.writeHeader(hd)
}

func (s *serverStream) SendHeader(hd Header) error {
	err := s.writeHeader(hd)
	if err != nil {
		return err
	}
	return s.stream.sendHeader()
}

func (s *serverStream) SetTrailer(tl Trailer) error {
	return s.writeTrailer(tl)
}
