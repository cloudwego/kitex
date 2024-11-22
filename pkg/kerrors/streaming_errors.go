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

package kerrors

import "errors"

var (
	// errStreaming is the parent type of all streaming errors.
	errStreaming = &basicError{message: "Streaming"}

	// ErrStreamingProtocol is the parent type of all streaming protocol(e.g. gRPC, TTHeader Streaming)
	// related but not user-aware errors.
	ErrStreamingProtocol = &basicError{message: "protocol error", parent: errStreaming}

	// errStreamingTimeout is the parent type of all streaming timeout errors.
	errStreamingTimeout = &basicError{message: "timeout error", parent: errStreaming}
	// ErrStreamTimeout denotes the timeout of the whole stream.
	ErrStreamTimeout = errStreamingTimeout.WithCause(errors.New("stream timeout"))

	// ErrStreamingCanceled is the parent type of all streaming canceled errors.
	ErrStreamingCanceled = &basicError{message: "canceled error", parent: errStreaming}
	// ErrBizCanceled denotes the stream is canceled by the biz code invoking cancel().
	ErrBizCanceled = ErrStreamingCanceled.WithCause(errors.New("business canceled"))
	// ErrGracefulShutdown denotes the stream is canceled due to graceful shutdown.
	ErrGracefulShutdown     = ErrStreamingCanceled.WithCause(errors.New("graceful shutdown"))
	ErrServerStreamFinished = ErrStreamingCanceled.WithCause(errors.New("server stream finished"))

	// errStreamingMeta is the parent type of all streaming meta errors.
	errStreamingMeta = &basicError{message: "meta error", parent: errStreaming}
	// ErrMetaSizeExceeded denotes the streaming meta size exceeds the limit.
	ErrMetaSizeExceeded = errStreamingMeta.WithCause(errors.New("size exceeds limit"))
)

// IsStreamingError reports whether the given err is a streaming err
func IsStreamingError(err error) bool {
	return errors.Is(err, errStreaming)
}
