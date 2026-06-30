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

package trans

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/cloudwego/kitex/internal/mocks"
	mockmessage "github.com/cloudwego/kitex/internal/mocks/message"
	remotemocks "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/mocks/stats"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

var (
	svcInfo     = mocks.ServiceInfo()
	svcSearcher = remotemocks.NewDefaultSvcSearcher()
)

func initOrResetMockServerRPCInfo(ri rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
	// When RPCInfo pooling is disabled, OnRead initializes a fresh RPCInfo by
	// passing nil to InitOrResetRPCInfoFunc. Keep the mock compatible with both
	// the pooled reset path and the non-pooled initialization path.
	if ri == nil {
		ri = newMockRPCInfo()
	}
	rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(addr)
	return ri
}

func mustReadDefaultServerRPCInfoAsync(t *testing.T, ctx context.Context) {
	t.Helper()

	done := make(chan interface{}, 1)
	go func() {
		defer func() {
			done <- recover()
		}()
		readDefaultServerRPCInfoForAsyncTest(ctx)
	}()
	if panicInfo := <-done; panicInfo != nil {
		t.Fatalf("async RPCInfo read panicked: %v", panicInfo)
	}
}

func readDefaultServerRPCInfoForAsyncTest(ctx context.Context) {
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil {
		panic("nil RPCInfo")
	}
	if from := ri.From(); from == nil {
		panic("nil From endpoint")
	} else {
		_ = from.ServiceName()
		_ = from.Method()
		_ = from.Address()
	}
	if to := ri.To(); to == nil {
		panic("nil To endpoint")
	} else {
		_ = to.ServiceName()
		_ = to.Method()
		_ = to.Address()
	}
	if inv := ri.Invocation(); inv == nil {
		panic("nil Invocation")
	} else {
		_ = inv.ServiceName()
		_ = inv.MethodName()
		_ = inv.StreamingMode()
	}
	if cfg := ri.Config(); cfg != nil {
		_ = cfg.RPCTimeout()
	}
	if stats := ri.Stats(); stats == nil {
		panic("nil RPCStats")
	} else {
		_ = stats.Level()
		_ = stats.Error()
	}
}

func TestDefaultSvrTransHandler(t *testing.T) {
	buf := remote.NewReaderWriterBuffer(1024)
	ext := &MockExtension{
		NewWriteByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
		NewReadByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
	}

	tagEncode, tagDecode := 0, 0
	opt := &remote.ServerOption{
		Codec: &MockCodec{
			EncodeFunc: func(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
				tagEncode++
				test.Assert(t, out == buf)
				return nil
			},
			DecodeFunc: func(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
				tagDecode++
				test.Assert(t, in == buf)
				return nil
			},
		},
		SvcSearcher: svcSearcher,
	}

	handler, err := NewDefaultSvrTransHandler(opt, ext)
	test.Assert(t, err == nil)

	ctx := context.Background()
	conn := &mocks.Conn{}
	msg := &mockmessage.MockMessage{
		RPCInfoFunc: func() rpcinfo.RPCInfo {
			return newMockRPCInfo()
		},
	}
	ctx, err = handler.Write(ctx, conn, msg)
	test.Assert(t, ctx != nil, ctx)
	test.Assert(t, err == nil, err)
	test.Assert(t, tagEncode == 1, tagEncode)
	test.Assert(t, tagDecode == 0, tagDecode)

	ctx, err = handler.Read(ctx, conn, msg)
	test.Assert(t, ctx != nil, ctx)
	test.Assert(t, err == nil, err)
	test.Assert(t, tagEncode == 1, tagEncode)
	test.Assert(t, tagDecode == 1, tagDecode)
}

func TestSvrTransHandlerBizError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracer := stats.NewMockTracer(ctrl)
	mockTracer.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) context.Context { return ctx }).AnyTimes()
	mockTracer.EXPECT().Finish(gomock.Any()).DoAndReturn(func(ctx context.Context) {
		err := rpcinfo.GetRPCInfo(ctx).Stats().Error()
		test.Assert(t, err != nil)
	}).AnyTimes()

	buf := remote.NewReaderWriterBuffer(1024)
	ext := &MockExtension{
		NewWriteByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
		NewReadByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
	}

	tracerCtl := &rpcinfo.TraceController{}
	tracerCtl.Append(mockTracer)
	opt := &remote.ServerOption{
		Codec: &MockCodec{
			EncodeFunc: func(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
				return nil
			},
			DecodeFunc: func(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
				mink := msg.RPCInfo().Invocation().(rpcinfo.InvocationSetter)
				mink.SetServiceName(mocks.MockServiceName)
				mink.SetMethodName(mocks.MockMethod)
				mink.SetMethodInfo(svcInfo.MethodInfo(context.Background(), mocks.MockMethod))
				return nil
			},
		},
		SvcSearcher:            svcSearcher,
		TracerCtl:              tracerCtl,
		InitOrResetRPCInfoFunc: initOrResetMockServerRPCInfo,
	}
	ri := rpcinfo.NewRPCInfo(rpcinfo.EmptyEndpointInfo(), rpcinfo.FromBasicInfo(&rpcinfo.EndpointBasicInfo{}),
		rpcinfo.NewInvocation("", mocks.MockMethod), nil, rpcinfo.NewRPCStats())
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

	svrHandler, err := NewDefaultSvrTransHandler(opt, ext)
	pl := remote.NewTransPipeline(svrHandler)
	svrHandler.SetPipeline(pl)
	if setter, ok := svrHandler.(remote.InvokeHandleFuncSetter); ok {
		setter.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			return kerrors.ErrBiz.WithCause(errors.New("mock"))
		})
	}
	test.Assert(t, err == nil)
	err = svrHandler.OnRead(ctx, &mocks.Conn{})
	test.Assert(t, err == nil)
}

func TestDefaultSvrTransHandlerDisablePoolKeepsRPCInfoReadableAfterOnRead(t *testing.T) {
	buf := remote.NewReaderWriterBuffer(1024)
	ext := &MockExtension{
		NewWriteByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
		NewReadByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
	}
	opt := &remote.ServerOption{
		Codec: &MockCodec{
			EncodeFunc: func(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
				return nil
			},
			DecodeFunc: func(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
				mink := msg.RPCInfo().Invocation().(rpcinfo.InvocationSetter)
				mink.SetServiceName(mocks.MockServiceName)
				mink.SetMethodName(mocks.MockMethod)
				mink.SetMethodInfo(svcInfo.MethodInfo(context.Background(), mocks.MockMethod))
				return nil
			},
		},
		SvcSearcher:            svcSearcher,
		TracerCtl:              &rpcinfo.TraceController{},
		InitOrResetRPCInfoFunc: initOrResetMockServerRPCInfo,
	}

	var captured context.Context
	svrHandler, err := NewDefaultSvrTransHandler(opt, ext)
	test.Assert(t, err == nil)
	pl := remote.NewTransPipeline(svrHandler)
	svrHandler.SetPipeline(pl)
	if setter, ok := svrHandler.(remote.InvokeHandleFuncSetter); ok {
		setter.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) error {
			captured = ctx
			return nil
		})
	}

	err = svrHandler.OnRead(context.Background(), &mocks.Conn{})
	test.Assert(t, err == nil, err)
	test.Assert(t, captured != nil)
	mustReadDefaultServerRPCInfoAsync(t, captured)
}

func TestSvrTransHandlerReadErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracer := stats.NewMockTracer(ctrl)
	mockTracer.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) context.Context { return ctx }).AnyTimes()
	mockTracer.EXPECT().Finish(gomock.Any()).DoAndReturn(func(ctx context.Context) {
		err := rpcinfo.GetRPCInfo(ctx).Stats().Error()
		test.Assert(t, err != nil)
	}).AnyTimes()

	buf := remote.NewReaderWriterBuffer(1024)
	ext := &MockExtension{
		NewWriteByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
		NewReadByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
	}

	mockErr := errors.New("mock")
	tracerCtl := &rpcinfo.TraceController{}
	tracerCtl.Append(mockTracer)
	opt := &remote.ServerOption{
		Codec: &MockCodec{
			EncodeFunc: func(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
				return nil
			},
			DecodeFunc: func(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
				return mockErr
			},
		},
		SvcSearcher:            svcSearcher,
		TracerCtl:              tracerCtl,
		InitOrResetRPCInfoFunc: initOrResetMockServerRPCInfo,
	}
	ri := rpcinfo.NewRPCInfo(rpcinfo.EmptyEndpointInfo(), rpcinfo.FromBasicInfo(&rpcinfo.EndpointBasicInfo{}),
		rpcinfo.NewInvocation("", mocks.MockMethod), nil, rpcinfo.NewRPCStats())
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

	svrHandler, err := NewDefaultSvrTransHandler(opt, ext)
	test.Assert(t, err == nil)
	pl := remote.NewTransPipeline(svrHandler)
	svrHandler.SetPipeline(pl)
	err = svrHandler.OnRead(ctx, &mocks.Conn{})
	test.Assert(t, err != nil)
	test.Assert(t, errors.Is(err, mockErr))
}

func TestSvrTransHandlerReadPanic(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracer := stats.NewMockTracer(ctrl)
	mockTracer.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) context.Context { return ctx }).AnyTimes()
	mockTracer.EXPECT().Finish(gomock.Any()).DoAndReturn(func(ctx context.Context) {
		err := rpcinfo.GetRPCInfo(ctx).Stats().Error()
		test.Assert(t, err != nil)
	}).AnyTimes()

	buf := remote.NewReaderWriterBuffer(1024)
	ext := &MockExtension{
		NewWriteByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
		NewReadByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
	}

	tracerCtl := &rpcinfo.TraceController{}
	tracerCtl.Append(mockTracer)
	opt := &remote.ServerOption{
		Codec: &MockCodec{
			EncodeFunc: func(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
				return nil
			},
			DecodeFunc: func(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
				panic("mock")
			},
		},
		SvcSearcher:            svcSearcher,
		TracerCtl:              tracerCtl,
		InitOrResetRPCInfoFunc: initOrResetMockServerRPCInfo,
	}
	ri := rpcinfo.NewRPCInfo(rpcinfo.EmptyEndpointInfo(), rpcinfo.FromBasicInfo(&rpcinfo.EndpointBasicInfo{}),
		rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

	svrHandler, err := NewDefaultSvrTransHandler(opt, ext)
	test.Assert(t, err == nil)
	pl := remote.NewTransPipeline(svrHandler)
	svrHandler.SetPipeline(pl)
	err = svrHandler.OnRead(ctx, &mocks.Conn{})
	test.Assert(t, err != nil)
	test.Assert(t, strings.Contains(err.Error(), "panic"))
}

func TestSvrTransHandlerOnReadHeartbeat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTracer := stats.NewMockTracer(ctrl)
	mockTracer.EXPECT().Start(gomock.Any()).DoAndReturn(func(ctx context.Context) context.Context { return ctx }).AnyTimes()
	mockTracer.EXPECT().Finish(gomock.Any()).DoAndReturn(func(ctx context.Context) {
		err := rpcinfo.GetRPCInfo(ctx).Stats().Error()
		test.Assert(t, err == nil)
	}).AnyTimes()

	buf := remote.NewReaderWriterBuffer(1024)
	ext := &MockExtension{
		NewWriteByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
		NewReadByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
			return buf
		},
	}

	tracerCtl := &rpcinfo.TraceController{}
	tracerCtl.Append(mockTracer)
	opt := &remote.ServerOption{
		Codec: &MockCodec{
			EncodeFunc: func(ctx context.Context, msg remote.Message, out remote.ByteBuffer) error {
				if msg.MessageType() != remote.Heartbeat {
					return errors.New("response is not of MessageType Heartbeat")
				}
				return nil
			},
			DecodeFunc: func(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
				msg.SetMessageType(remote.Heartbeat)
				return nil
			},
		},
		SvcSearcher:            svcSearcher,
		TracerCtl:              tracerCtl,
		InitOrResetRPCInfoFunc: initOrResetMockServerRPCInfo,
	}
	ri := rpcinfo.NewRPCInfo(rpcinfo.EmptyEndpointInfo(), rpcinfo.FromBasicInfo(&rpcinfo.EndpointBasicInfo{}),
		rpcinfo.NewInvocation("", mocks.MockMethod), nil, rpcinfo.NewRPCStats())
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

	svrHandler, err := NewDefaultSvrTransHandler(opt, ext)
	test.Assert(t, err == nil)
	pl := remote.NewTransPipeline(svrHandler)
	svrHandler.SetPipeline(pl)
	err = svrHandler.OnRead(ctx, &mocks.Conn{})
	test.Assert(t, err == nil)
}
