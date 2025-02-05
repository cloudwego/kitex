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

package streamclient

import (
	"context"
	"fmt"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/utils"
)

// Deprecated: Use client.WithStreamRecvMiddleware instead, this requires enabling the streamx feature.
func WithRecvMiddleware(mw endpoint.RecvMiddleware) Option {
	mwb := func(ctx context.Context) endpoint.RecvMiddleware {
		return mw
	}
	return Option{F: func(o *client.Options, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithRecvMiddleware(%+v)", utils.GetFuncName(mw)))
		o.Streaming.RecvMiddlewareBuilders = append(o.Streaming.RecvMiddlewareBuilders, mwb)
	}}
}

// Deprecated: Use client.WithStreamRecvMiddlewareBuilder instead, this requires enabling the streamx feature.
func WithRecvMiddlewareBuilder(mwb endpoint.RecvMiddlewareBuilder) Option {
	return Option{F: func(o *client.Options, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithRecvMiddlewareBuilder(%+v)", utils.GetFuncName(mwb)))
		o.Streaming.RecvMiddlewareBuilders = append(o.Streaming.RecvMiddlewareBuilders, mwb)
	}}
}

// Deprecated: Use client.WithStreamSendMiddleware instead, this requires enabling the streamx feature.
func WithSendMiddleware(mw endpoint.SendMiddleware) Option {
	mwb := func(ctx context.Context) endpoint.SendMiddleware {
		return mw
	}
	return Option{F: func(o *client.Options, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithSendMiddleware(%+v)", utils.GetFuncName(mw)))
		o.Streaming.SendMiddlewareBuilders = append(o.Streaming.SendMiddlewareBuilders, mwb)
	}}
}

// Deprecated: Use client.WithStreamSendMiddlewareBuilder instead, this requires enabling the streamx feature.
func WithSendMiddlewareBuilder(mwb endpoint.SendMiddlewareBuilder) Option {
	return Option{F: func(o *client.Options, di *utils.Slice) {
		di.Push(fmt.Sprintf("WithSendMiddlewareBuilder(%+v)", utils.GetFuncName(mwb)))
		o.Streaming.SendMiddlewareBuilders = append(o.Streaming.SendMiddlewareBuilders, mwb)
	}}
}
