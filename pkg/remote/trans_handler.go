/*
 * Copyright 2021 CloudWeGo Authors
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

package remote

import (
	"context"
	"net"

	"github.com/cloudwego/kitex/pkg/streaming"
)

// ClientTransHandlerFactory to new TransHandler for client
type ClientTransHandlerFactory interface {
	NewTransHandler(opt *ClientOption) (ClientTransHandler, error)
}

// ServerTransHandlerFactory to new TransHandler for server
type ServerTransHandlerFactory interface {
	NewTransHandler(opt *ServerOption) (ServerTransHandler, error)
}

type ClientStream interface {
	Write(ctx context.Context, send Message) (nctx context.Context, err error)
	Read(ctx context.Context, msg Message) (nctx context.Context, err error)
}

// ClientTransHandler is just TransHandler.
type ClientTransHandler interface {
	OnInactive(ctx context.Context, conn net.Conn)
	OnError(ctx context.Context, err error, conn net.Conn)
	NewStream(ctx context.Context, conn net.Conn) (streaming.ClientStream, error)
	SetPipeline(pipeline *ClientTransPipeline)
}

type ServerStream interface {
	Write(ctx context.Context, send Message) (nctx context.Context, err error)
	Read(ctx context.Context, msg Message) (nctx context.Context, err error)
}

// ServerTransHandler have some new functions.
type ServerTransHandler interface {
	OnInactive(ctx context.Context, conn net.Conn)
	OnError(ctx context.Context, err error, conn net.Conn)
	OnActive(ctx context.Context, conn net.Conn) (context.Context, error)
	OnRead(ctx context.Context, conn net.Conn) error
	OnStream(ctx context.Context, st streaming.ServerStream) error
	SetPipeline(pipeline *ServerTransPipeline)
}

// GracefulShutdown supports closing connections in a graceful manner.
type GracefulShutdown interface {
	GracefulShutdown(ctx context.Context) error
}

type InnerServerEndpoint func(ctx context.Context, st streaming.ServerStream) (err error)

// InvokeHandleFuncSetter is used to set invoke handle func.
type InvokeHandleFuncSetter interface {
	SetInvokeHandleFunc(inkHdlFunc InnerServerEndpoint)
}
