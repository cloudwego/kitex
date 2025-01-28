// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package streamxserver

import (
	"context"
	"errors"

	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/streamx"
)

var errServerStreamArgsNotFound = errors.New("stream args not found")

func InvokeUnaryHandler[Req, Res any](
	ctx context.Context, args streaming.Args, methodHandler streamx.UnaryHandler[Req, Res],
) error {
	gs := streamx.NewGenericServerStream[Req, Res](args.ServerStream)
	req, err := gs.Recv(ctx)
	if err != nil {
		return err
	}
	res, err := methodHandler(ctx, req)
	if err != nil {
		return err
	}
	return gs.Send(ctx, res)
}

func InvokeClientStreamHandler[Req, Res any](
	ctx context.Context, args streaming.Args, methodHandler streamx.ClientStreamingHandler[Req, Res],
) (err error) {
	gs := streamx.NewGenericServerStream[Req, Res](args.ServerStream)
	res, err := methodHandler(ctx, gs)
	if err != nil {
		return err
	}
	return gs.Send(ctx, res)
}

func InvokeServerStreamHandler[Req, Res any](
	ctx context.Context, args streaming.Args, methodHandler streamx.ServerStreamingHandler[Req, Res],
) (err error) {
	gs := streamx.NewGenericServerStream[Req, Res](args.ServerStream)
	// before handler call
	req, err := gs.Recv(ctx)
	if err != nil {
		return err
	}
	return methodHandler(ctx, req, gs)
}

func InvokeBidiStreamHandler[Req, Res any](
	ctx context.Context, args streaming.Args, methodHandler streamx.BidiStreamingHandler[Req, Res],
) (err error) {
	// handler call
	gs := streamx.NewGenericServerStream[Req, Res](args.ServerStream)
	return methodHandler(ctx, gs)
}
