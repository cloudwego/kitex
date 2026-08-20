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
	"context"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

type cascadeCancelContext struct {
	context.Context
	err error
}

// Err models the process-local cancellation reason exposed by the gRPC server
// context when it directly drives an outbound client-side stream.
func (c *cascadeCancelContext) Err() error {
	return c.err
}

func TestStream(t *testing.T) {
	// init
	opt := newMockServerOption()
	conn := newMockNpConn(mockAddr0)
	conn.mockSettingFrame()
	tr, err := newMockServerTransport(conn)
	test.Assert(t, err == nil, err)
	s := grpc.CreateStream(context.Background(), 1, func(i int) {}, "")
	serverConn := newServerConn(tr, s)
	defer serverConn.Close()

	handler, err := NewSvrTransHandlerFactory().NewTransHandler(opt)
	test.Assert(t, err == nil, err)
	ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingNone)

	// test newServerStream()
	stream := newServerStream(ctx, serverConn, handler)

	// test Context()
	strCtx := stream.GetGRPCStream().Context()
	test.Assert(t, strCtx == ctx)

	// test recvMsg()
	msg := newMockNewMessage().Data()
	newMockStreamRecvHelloRequest(s)
	err = stream.RecvMsg(ctx, msg)
	test.Assert(t, err == nil, err)

	// test SendMsg()
	err = stream.SendMsg(ctx, msg)
	test.Assert(t, err == nil, err)

	// test Close()
	err = stream.GetGRPCStream().Close()
	test.Assert(t, err == nil, err)
}

func TestClientStreamRecvMsgCascadeCancel(t *testing.T) {
	reason := status.Err(codes.Canceled, "inbound RPC terminated")
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	ctx := rpcinfo.NewCtxWithRPCInfo(
		&cascadeCancelContext{Context: parent, err: reason},
		newMockRPCInfo(serviceinfo.StreamingBidirectional),
	)

	opt := newMockClientOption(nil)
	defer opt.ConnPool.Close()
	conn, err := opt.ConnPool.Get(ctx, "tcp", mockAddr0, newMockConnOption())
	test.Assert(t, err == nil, err)

	handler, err := NewCliTransHandlerFactory().NewTransHandler(opt)
	test.Assert(t, err == nil, err)
	stream := NewClientStream(ctx, nil, conn, handler)

	errCh := make(chan error, 1)
	go func() {
		errCh <- stream.RecvMsg(ctx, &HelloRequest{})
	}()

	select {
	case err = <-errCh:
	case <-time.After(time.Second):
		t.Fatal("RecvMsg did not return after cascade cancellation")
	}

	st, ok := status.FromError(err)
	test.Assert(t, ok, err)
	test.Assert(t, st.IsCascadeCancel())
	test.Assert(t, st.Code() == codes.Canceled, st.Code())
	test.Assert(t, st.Message() == "inbound RPC terminated", st.Message())
}
