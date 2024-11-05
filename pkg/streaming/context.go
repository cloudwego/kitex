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

package streaming

import "context"

type (
	streamKey       struct{}
	serverStreamKey struct{}
	clientStreamKey struct{}
)

// Deprecated, use NewCtxWithServerStream instead.
func NewCtxWithStream(ctx context.Context, stream Stream) context.Context {
	return context.WithValue(ctx, streamKey{}, stream)
}

// Deprecated, use GetServerStream instead.
func GetStream(ctx context.Context) Stream {
	if s, ok := ctx.Value(streamKey{}).(Stream); ok {
		return s
	}
	return nil
}

func NewCtxWithServerStream(ctx context.Context, stream ServerStream) context.Context {
	return context.WithValue(ctx, serverStreamKey{}, stream)
}

func GetServerStream(ctx context.Context) ServerStream {
	if s, ok := ctx.Value(serverStreamKey{}).(ServerStream); ok {
		return s
	}
	return nil
}
