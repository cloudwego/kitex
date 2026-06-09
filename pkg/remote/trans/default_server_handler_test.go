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
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/cloudwego/kitex/internal/mocks"
	mocksklog "github.com/cloudwego/kitex/internal/mocks/klog"
	mockmessage "github.com/cloudwego/kitex/internal/mocks/message"
	remotemocks "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/mocks/stats"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

var (
	svcInfo     = mocks.ServiceInfo()
	svcSearcher = remotemocks.NewDefaultSvcSearcher()
)

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
		SvcSearcher: svcSearcher,
		TracerCtl:   tracerCtl,
		InitOrResetRPCInfoFunc: func(ri rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
			rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(addr)
			return ri
		},
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
		SvcSearcher: svcSearcher,
		TracerCtl:   tracerCtl,
		InitOrResetRPCInfoFunc: func(ri rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
			rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(addr)
			return ri
		},
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

func TestSvrTransHandlerReadErrLoggedWithRemoteService(t *testing.T) {
	poolEnabled := rpcinfo.PoolEnabled()
	defer rpcinfo.EnablePool(poolEnabled)

	for _, enablePool := range []bool{true, false} {
		t.Run(fmt.Sprintf("pool_enabled_%t", enablePool), func(t *testing.T) {
			rpcinfo.EnablePool(enablePool)
			ctrl := gomock.NewController(t)
			prevLogger := klog.DefaultLogger()
			defer func() {
				klog.SetLogger(prevLogger)
				ctrl.Finish()
			}()

			buf := remote.NewReaderWriterBuffer(1024)
			ext := &MockExtension{
				NewWriteByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
					return buf
				},
				NewReadByteBufferFunc: func(ctx context.Context, conn net.Conn, msg remote.Message) remote.ByteBuffer {
					return buf
				},
			}

			mockErr := errors.New("mock decode error")
			const callerService = "mockCaller"
			opt := &remote.ServerOption{
				Codec: &MockCodec{
					DecodeFunc: func(ctx context.Context, msg remote.Message, in remote.ByteBuffer) error {
						err := rpcinfo.AsMutableEndpointInfo(msg.RPCInfo().From()).SetServiceName(callerService)
						test.Assert(t, err == nil, err)
						return mockErr
					},
				},
				SvcSearcher: svcSearcher,
				InitOrResetRPCInfoFunc: func(ri rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
					if ri == nil {
						ri = newMockRPCInfo()
					}
					remote := rpcinfo.AsMutableEndpointInfo(ri.From())
					test.Assert(t, remote != nil)
					_ = remote.SetServiceName("")
					remote.SetAddress(addr)
					return ri
				},
			}
			ri := rpcinfo.NewRPCInfo(rpcinfo.EmptyEndpointInfo(), rpcinfo.FromBasicInfo(&rpcinfo.EndpointBasicInfo{}),
				rpcinfo.NewInvocation("", mocks.MockMethod), nil, rpcinfo.NewRPCStats())
			ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
			conn := &mocks.Conn{
				RemoteAddrFunc: func() (r net.Addr) {
					return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8888}
				},
			}

			mocklogger := mocksklog.NewMockFullLogger(ctrl)
			klog.SetLogger(mocklogger)
			var logs []string
			mocklogger.EXPECT().CtxErrorf(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(ctx context.Context, format string, v ...interface{}) {
				logs = append(logs, fmt.Sprintf(format, v...))
			}).Times(1)

			svrHandler, err := NewDefaultSvrTransHandler(opt, ext)
			test.Assert(t, err == nil)
			pl := remote.NewTransPipeline(svrHandler)
			svrHandler.SetPipeline(pl)
			err = svrHandler.OnRead(ctx, conn)
			test.Assert(t, err != nil)
			test.Assert(t, errors.Is(err, mockErr))

			svrHandler.OnError(ctx, err, conn)
			test.Assert(t, len(logs) == 1, logs)
			test.Assert(t, strings.Contains(logs[0], "remoteService="+callerService), logs[0])
			test.Assert(t, strings.Contains(logs[0], "remoteAddr=127.0.0.1:8888"), logs[0])
			test.Assert(t, strings.Contains(logs[0], "error="+mockErr.Error()), logs[0])
		})
	}
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
		SvcSearcher: svcSearcher,
		TracerCtl:   tracerCtl,
		InitOrResetRPCInfoFunc: func(ri rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
			rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(addr)
			return ri
		},
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
		SvcSearcher: svcSearcher,
		TracerCtl:   tracerCtl,
		InitOrResetRPCInfoFunc: func(ri rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
			rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(addr)
			return ri
		},
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
