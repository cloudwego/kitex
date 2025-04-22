//go:build !windows
// +build !windows

/*
 * Copyright 2022 CloudWeGo Authors
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

// Package server defines the Options about remote transport of client.
package server

import (
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans/detection"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	streamxstrans "github.com/cloudwego/kitex/pkg/remote/trans/streamx"
	"github.com/cloudwego/kitex/pkg/remote/trans/streamx/provider"

	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream"
)

func newServerRemoteOption() (*remote.ServerOption, provider.ServerProvider) {
	ttstreamServerProvider := ttstream.NewServerProvider()
	return &remote.ServerOption{
		TransServerFactory: netpoll.NewTransServerFactory(),
		SvrHandlerFactory: detection.NewSvrTransHandlerFactory(
			netpoll.NewSvrTransHandlerFactory(),
			nphttp2.NewSvrTransHandlerFactory(),
			streamxstrans.NewSvrTransHandlerFactory(ttstreamServerProvider),
		),
		Codec:                 codec.NewDefaultCodec(),
		Address:               defaultAddress,
		ExitWaitTime:          defaultExitWaitTime,
		MaxConnectionIdleTime: defaultConnectionIdleTime,
		AcceptFailedDelayTime: defaultAcceptFailedDelayTime,
		GRPCCfg:               grpc.DefaultServerConfig(),
	}, ttstreamServerProvider
}
