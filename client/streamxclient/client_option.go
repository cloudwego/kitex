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

package streamxclient

import (
	"time"

	"github.com/cloudwego/kitex/client"
	internal_client "github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/utils"
)

type Option internal_client.Option

func WithHostPorts(hostports ...string) Option {
	return ConvertNativeClientOption(client.WithHostPorts(hostports...))
}

func WithRecvTimeout(timeout time.Duration) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamXOptions.RecvTimeout = timeout
	}}
}

func WithDestService(destService string) Option {
	return ConvertNativeClientOption(client.WithDestService(destService))
}

func WithProvider(pvd streamx.ClientProvider) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.RemoteOpt.Provider = pvd
	}}
}

func WithStreamMiddleware(smw streamx.StreamMiddleware) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamXOptions.StreamMWs = append(o.StreamXOptions.StreamMWs, smw)
	}}
}

func WithStreamRecvMiddleware(smw streamx.StreamRecvMiddleware) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamXOptions.StreamRecvMWs = append(o.StreamXOptions.StreamRecvMWs, smw)
	}}
}

func WithStreamSendMiddleware(smw streamx.StreamSendMiddleware) Option {
	return Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamXOptions.StreamSendMWs = append(o.StreamXOptions.StreamSendMWs, smw)
	}}
}

func ConvertNativeClientOption(o internal_client.Option) Option {
	return Option{F: o.F}
}

func ConvertStreamXClientOption(o Option) internal_client.Option {
	return internal_client.Option{F: o.F}
}
