/*
 * Copyright 2023 CloudWeGo Authors
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

package endpoint

import (
	"github.com/cloudwego/kitex/pkg/streaming"
)

// streamWithMiddleware enables interception for stream's SendMsg and RecvMsg.
type streamWithMiddleware struct {
	streaming.Stream

	recvEndpoint RecvEndpoint
	sendEndpoint SendEndpoint
}

// NewStreamWithMiddleware creates a new Stream with recv/send middleware support
func NewStreamWithMiddleware(st streaming.Stream, recv RecvEndpoint, send SendEndpoint) streaming.Stream {
	return &streamWithMiddleware{
		Stream:       st,
		recvEndpoint: recv,
		sendEndpoint: send,
	}
}

// RecvMsg implements the streaming.Stream interface with recv middlewares
func (s *streamWithMiddleware) RecvMsg(m interface{}) error {
	return s.recvEndpoint(s.Stream, m)
}

// SendMsg implements the streaming.Stream interface with send middlewares
func (s *streamWithMiddleware) SendMsg(m interface{}) error {
	return s.sendEndpoint(s.Stream, m)
}
