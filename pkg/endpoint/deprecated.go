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
	"context"

	"github.com/cloudwego/kitex/pkg/streaming"
)

// Deprecated, use client.StreamRecvEndpoint or server.StreamRecvEndpoint instead.
type RecvEndpoint func(stream streaming.Stream, message interface{}) (err error)

// Deprecated, use client.StreamRecvMiddleware or server.StreamRecvMiddleware instead.
type RecvMiddleware func(next RecvEndpoint) RecvEndpoint

// Deprecated, use client.StreamRecvMiddlewareBuilder or server.StreamRecvMiddlewareBuilder instead.
type RecvMiddlewareBuilder func(ctx context.Context) RecvMiddleware

// Deprecated, use client.StreamRecvChain or server.StreamRecvChain instead.
func RecvChain(mws ...RecvMiddleware) RecvMiddleware {
	return func(next RecvEndpoint) RecvEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

// Deprecated, use client.StreamSendEndpoint or server.StreamSendEndpoint instead.
type SendEndpoint func(stream streaming.Stream, message interface{}) (err error)

// Deprecated, use client.StreamSendMiddleware or server.StreamSendMiddleware instead.
type SendMiddleware func(next SendEndpoint) SendEndpoint

// Deprecated, use client.StreamSendMiddlewareBuilder or server.StreamSendMiddlewareBuilder instead.
type SendMiddlewareBuilder func(ctx context.Context) SendMiddleware

// Deprecated, use client.StreamSendChain or server.StreamSendChain instead.
func SendChain(mws ...SendMiddleware) SendMiddleware {
	return func(next SendEndpoint) SendEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}
