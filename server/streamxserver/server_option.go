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

package streamxserver

import (
	"net"

	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/server"
)

type (
	Option  internal_server.Option
	Options = internal_server.Options
)

func WithListener(ln net.Listener) Option {
	return ConvertNativeServerOption(server.WithListener(ln))
}

func WithStreamMiddleware(mw streamx.StreamMiddleware) server.RegisterOption {
	return server.RegisterOption{F: func(o *internal_server.RegisterOptions) {
		o.StreamMiddlewares = append(o.StreamMiddlewares, mw)
	}}
}

func WithStreamRecvMiddleware(mw streamx.StreamRecvMiddleware) server.RegisterOption {
	return server.RegisterOption{F: func(o *internal_server.RegisterOptions) {
		o.StreamRecvMiddlewares = append(o.StreamRecvMiddlewares, mw)
	}}
}

func WithStreamSendMiddleware(mw streamx.StreamSendMiddleware) server.RegisterOption {
	return server.RegisterOption{F: func(o *internal_server.RegisterOptions) {
		o.StreamSendMiddlewares = append(o.StreamSendMiddlewares, mw)
	}}
}

func WithProvider(provider streamx.ServerProvider) server.RegisterOption {
	return server.RegisterOption{F: func(o *internal_server.RegisterOptions) {
		o.Provider = provider
	}}
}

func ConvertNativeServerOption(o internal_server.Option) Option {
	return Option{F: o.F}
}

func ConvertStreamXServerOption(o Option) internal_server.Option {
	return internal_server.Option{F: o.F}
}
