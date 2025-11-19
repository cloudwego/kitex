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

package streaming

import (
	"context"
	"runtime/debug"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	streaming_types "github.com/cloudwego/kitex/pkg/streaming/types"
)

// todo: think about stream session metadata

// todo: add an event to help debug the potential slow callback
type FinishCallback struct {
	// Framework/Proxy error
	OnWireError func(ctx context.Context, wErr streaming_types.WireError)
	// Timeout error, including Stream Timeout，Recv Timeout and Send Timeout
	OnTimeoutError func(ctx context.Context, tErr streaming_types.TimeoutError)
	// Cancel error, including Cascaded Cancel error and Active Cancel error
	OnCancelError func(ctx context.Context, cErr streaming_types.CancelError)
	// Business error, including error returned from remote handler/MW, and other errors except from
	// WireError, TimeoutError, CancelError
	OnBusinessError func(ctx context.Context, bizErr kerrors.BizStatusErrorIface)
}

type ServerStreamingCallback[Res any, Stream ServerStreamingClient[Res]] func(ctx context.Context, stream Stream) error

type ClientStreamingCallback[Req, Res any, Stream ClientStreamingClient[Req, Res]] func(ctx context.Context, stream Stream) error

type BidiStreamingCallback[Req any, Res any, Stream BidiStreamingClient[Req, Res]] func(ctx context.Context, stream Stream) error

func HandleServerStreaming[Res any, Stream ServerStreamingClient[Res]](ctx context.Context, stream Stream, callback ServerStreamingCallback[Res, Stream]) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// todo: retrieve rpcinfo and print detailed information
			klog.CtxWarnf(ctx, "Panic happened during HandleServerStreaming. This may caused by callback: error=%v, stack=%s", r, string(debug.Stack()))
		}
		stream.Cancel(err)
		FinishClientStream(stream, err)
	}()

	return callback(ctx, stream)
}

func HandleClientStreaming[Req, Res any, Stream ClientStreamingClient[Req, Res]](ctx context.Context, stream Stream, callback ClientStreamingCallback[Req, Res, Stream]) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// todo: retrieve rpcinfo and print detailed information
			klog.CtxWarnf(ctx, "Panic happened during HandleClientStreaming. This may caused by callback: error=%v, stack=%s", r, string(debug.Stack()))
		}
		stream.Cancel(err)
		FinishClientStream(stream, err)
	}()
	return callback(ctx, stream)
}

func HandleBidiStreaming[Req, Res any, Stream BidiStreamingClient[Req, Res]](ctx context.Context, stream Stream, callback BidiStreamingCallback[Req, Res, Stream]) (err error) {
	defer func() {
		if r := recover(); r != nil {
			// todo: retrieve rpcinfo and print detailed information
			klog.CtxWarnf(ctx, "Panic happened during HandleBidiStreaming. This may caused by callback: error=%v, stack=%s", r, string(debug.Stack()))
		}
		stream.Cancel(err)
		FinishClientStream(stream, err)
	}()

	return callback(ctx, stream)
}
