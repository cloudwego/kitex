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

package client

import (
	"fmt"
	"time"

	"github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/endpoint/cep"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/utils"
)

// WithStreamOptions add stream options for client.
// It is used to isolate options that are only effective for streaming methods.
func WithStreamOptions(opts ...StreamOption) Option {
	return Option{F: func(o *client.Options, di *utils.Slice) {
		var udi utils.Slice
		for _, opt := range opts {
			opt.F(&o.StreamOptions, &udi)
		}
		di.Push(map[string]interface{}{
			"WithStreamOptions": udi,
		})
	}}
}

// WithStreamRecvTimeout add recv timeout for stream.Recv function.
// NOTICE: ONLY effective for ttheader streaming protocol for now.
//
// Deprecated: using WithStreamRecvTimeoutConfig
// When WithStreamRecvTimeout and WithStreamRecvTimeoutConfig are both configured for ttstream,
// WithStreamRecvTimeoutConfig has higher priority.
func WithStreamRecvTimeout(d time.Duration) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithStreamRecvTimeout(%dms)", d.Milliseconds()))

		o.RecvTimeout = d
	}}
}

// WithStreamRecvTimeoutConfig add recv timeout for stream.Recv function.
// By default, it will cancel the remote peer when timeout.
//
// However, in certain scenarios (e.g., resume-enabled transfers: A → B → C), A may not wish to cancel B and C upon detecting a timeout.
// It expects B and C to complete one round of streaming communication and cache the results.
// This allows A to resume the request from the disconnected point on the next attempt, completing the resume-from-breakpoint process.
// Config like this:
//
//	WithStreamRecvTimeoutConfig(streaming.TimeoutConfig{
//		    Timeout: tm,
//		    DisableCancelRemote: true,
//	})
//
// The remote peer must promptly exit the handler; otherwise, there is a risk of stream leakage!
func WithStreamRecvTimeoutConfig(cfg streaming.TimeoutConfig) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithStreamRecvTimeoutConfig(%+v)", cfg))

		o.RecvTimeoutConfig = cfg
	}}
}

// WithStreamMiddleware add middleware for stream.
func WithStreamMiddleware(mw cep.StreamMiddleware) StreamOption {
	return StreamOption{F: func(o *StreamOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithStreamMiddleware(%+v)", utils.GetFuncName(mw)))

		o.StreamMiddlewares = append(o.StreamMiddlewares, mw)
	}}
}

// WithStreamMiddlewareBuilder add middleware builder for stream.
func WithStreamMiddlewareBuilder(mwb cep.StreamMiddlewareBuilder) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithStreamMiddlewareBuilder(%+v)", utils.GetFuncName(mwb)))

		o.StreamMiddlewareBuilders = append(o.StreamMiddlewareBuilders, mwb)
	}}
}

// WithStreamRecvMiddleware add recv middleware for stream.
func WithStreamRecvMiddleware(mw cep.StreamRecvMiddleware) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithStreamRecvMiddleware(%+v)", utils.GetFuncName(mw)))

		o.StreamRecvMiddlewares = append(o.StreamRecvMiddlewares, mw)
	}}
}

// WithStreamRecvMiddlewareBuilder add recv middleware builder for stream.
func WithStreamRecvMiddlewareBuilder(mwb cep.StreamRecvMiddlewareBuilder) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithStreamRecvMiddlewareBuilder(%+v)", utils.GetFuncName(mwb)))

		o.StreamRecvMiddlewareBuilders = append(o.StreamRecvMiddlewareBuilders, mwb)
	}}
}

// WithStreamSendMiddleware add send middleware for stream.
func WithStreamSendMiddleware(mw cep.StreamSendMiddleware) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithStreamSendMiddleware(%+v)", utils.GetFuncName(mw)))

		o.StreamSendMiddlewares = append(o.StreamSendMiddlewares, mw)
	}}
}

// WithStreamSendMiddlewareBuilder add send middleware builder for stream.
func WithStreamSendMiddlewareBuilder(mwb cep.StreamSendMiddlewareBuilder) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithStreamSendMiddlewareBuilder(%+v)", utils.GetFuncName(mwb)))

		o.StreamSendMiddlewareBuilders = append(o.StreamSendMiddlewareBuilders, mwb)
	}}
}

// WithStreamEventHandler add StreamEventHandler for detailed streaming event tracing
func WithStreamEventHandler(hdl rpcinfo.ClientStreamEventHandler) StreamOption {
	return StreamOption{F: func(o *client.StreamOptions, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithStreamEventHandler(%+v)", hdl))

		o.StreamEventHandlers = append(o.StreamEventHandlers, hdl)
	}}
}
