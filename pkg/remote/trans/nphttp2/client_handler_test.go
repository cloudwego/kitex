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

package nphttp2

import (
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
)

func TestClientHandler(t *testing.T) {
	// init
	opt := newMockClientOption()
	ctx := newMockCtxWithRPCInfo()
	msg := newMockNewMessage()
	conn, err := opt.ConnPool.Get(ctx, "tcp", mockAddr0, remote.ConnOption{Dialer: opt.Dialer, ConnectTimeout: time.Second})
	test.Assert(t, err == nil, err)

	// test NewTransHandler()
	cliTransHandler, err := NewCliTransHandlerFactory().NewTransHandler(opt)
	test.Assert(t, err == nil, err)

	// test Read()
	newMockStreamRecvHelloRequest(conn.(*clientConn).s)
	err = cliTransHandler.Read(ctx, conn, msg)
	test.Assert(t, err == nil, err)

	// test write()
	err = cliTransHandler.Write(ctx, conn, msg)
	test.Assert(t, err == nil, err)
}

func TestClientHandlerOnAction(t *testing.T) {
	// init
	opt := newMockClientOption()
	ctx := newMockCtxWithRPCInfo()
	conn, err := opt.ConnPool.Get(ctx, "tcp", mockAddr0, remote.ConnOption{Dialer: opt.Dialer, ConnectTimeout: time.Second})
	test.Assert(t, err == nil, err)
	defer conn.Close()
	cliTransHandler, err := newCliTransHandler(opt)
	test.Assert(t, err == nil, err)
	// test OnConnection()
	cliCtx, err := cliTransHandler.OnConnect(ctx)
	test.Assert(t, err == nil, err)
	test.Assert(t, cliCtx == ctx)

	// test OnMessage()
	cliCtx, err = cliTransHandler.OnMessage(ctx, new(mockMessage), new(mockMessage))
	test.Assert(t, err == nil, err)
	test.Assert(t, cliCtx == ctx)

	// test OnRead()
	setDontWaitHeader(conn.(*clientConn).s)
	_, err = cliTransHandler.OnRead(ctx, conn)
	test.Assert(t, err == nil, err)

	// test SetPipeline()
	cliTransHandler.SetPipeline(nil)
}
