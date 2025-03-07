/*
 * Copyright 2025 CloudWeGo Authors
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

// Package cep means client endpoint in short.
package cep

import (
	"context"
	"unsafe"

	"github.com/cloudwego/kitex/pkg/streaming"
)

// StreamEndpoint represent one Stream call, it returns a stream.
type StreamEndpoint func(ctx context.Context) (st streaming.ClientStream, err error)

// StreamMiddleware deal with input StreamEndpoint and output StreamEndpoint.
type StreamMiddleware func(next StreamEndpoint) StreamEndpoint

// StreamMiddlewareBuilder builds a stream middleware with information from a context.
type StreamMiddlewareBuilder func(ctx context.Context) StreamMiddleware

// StreamRecvEndpoint represent one Stream Recv call, the inner endpoint will call stream.RecvMsg(ctx, message).
type StreamRecvEndpoint func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error)

func (e StreamRecvEndpoint) EqualsTo(e2 StreamRecvEndpoint) bool {
	return *(*unsafe.Pointer)(unsafe.Pointer(&e)) == *(*unsafe.Pointer)(unsafe.Pointer(&e2))
}

// StreamRecvMiddleware deal with input StreamRecvEndpoint and output StreamRecvEndpoint.
type StreamRecvMiddleware func(next StreamRecvEndpoint) StreamRecvEndpoint

// StreamRecvMiddlewareBuilder builds a stream recv middleware with information from a context.
type StreamRecvMiddlewareBuilder func(ctx context.Context) StreamRecvMiddleware

// StreamSendEndpoint represent one Stream Send call.
type StreamSendEndpoint func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error)

func (e StreamSendEndpoint) EqualsTo(e2 StreamSendEndpoint) bool {
	return *(*unsafe.Pointer)(unsafe.Pointer(&e)) == *(*unsafe.Pointer)(unsafe.Pointer(&e2))
}

// StreamSendMiddleware deal with input StreamSendEndpoint and output StreamSendEndpoint.
type StreamSendMiddleware func(next StreamSendEndpoint) StreamSendEndpoint

// StreamSendMiddlewareBuilder builds a stream send middleware with information from a context.
type StreamSendMiddlewareBuilder func(ctx context.Context) StreamSendMiddleware

// StreamChain connect middlewares into one middleware.
func StreamChain(mws ...StreamMiddleware) StreamMiddleware {
	return func(next StreamEndpoint) StreamEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

// StreamRecvChain connect recv middlewares into one middleware.
func StreamRecvChain(mws ...StreamRecvMiddleware) StreamRecvMiddleware {
	return func(next StreamRecvEndpoint) StreamRecvEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

// StreamSendChain connect send middlewares into one middleware.
func StreamSendChain(mws ...StreamSendMiddleware) StreamSendMiddleware {
	return func(next StreamSendEndpoint) StreamSendEndpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

// DummyDummyMiddleware is a dummy middleware.
func DummyDummyMiddleware(next StreamEndpoint) StreamEndpoint {
	return next
}
