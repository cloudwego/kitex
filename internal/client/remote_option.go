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

// Package client defines the Options about remote transport of client.
package client

import (
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
)

func newClientRemoteOption() *remote.ClientOption {
	return &remote.ClientOption{
		CliHandlerFactory: netpoll.NewCliTransHandlerFactory(),
		Dialer:            netpoll.NewDialer(),
		Codec:             codec.NewDefaultCodec(),
	}
}
