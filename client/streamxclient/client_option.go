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

	internal_client "github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/utils"
)

func WithProvider(pvd streamx.ClientProvider) internal_client.Option {
	return internal_client.Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.RemoteOpt.Provider = pvd
	}}
}

func WithStreamRecvTimeout(timeout time.Duration) internal_client.Option {
	return internal_client.Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamX.RecvTimeout = timeout
	}}
}

func WithStreamMiddleware(smw streamx.StreamMiddleware) internal_client.Option {
	return internal_client.Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamX.StreamMWs = append(o.StreamX.StreamMWs, smw)
	}}
}

func WithStreamRecvMiddleware(smw streamx.StreamRecvMiddleware) internal_client.Option {
	return internal_client.Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamX.StreamRecvMWs = append(o.StreamX.StreamRecvMWs, smw)
	}}
}

func WithStreamSendMiddleware(smw streamx.StreamSendMiddleware) internal_client.Option {
	return internal_client.Option{F: func(o *internal_client.Options, di *utils.Slice) {
		o.StreamX.StreamSendMWs = append(o.StreamX.StreamSendMWs, smw)
	}}
}
