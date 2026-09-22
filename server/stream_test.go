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

package server

import (
	"context"
	"errors"
	"testing"
	"time"

	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type mockTracer struct {
	stats.Tracer
	rpcinfo.StreamEventReporter
}

func Test_server_initStreamMiddlewares(t *testing.T) {
	ctl := &rpcinfo.TraceController{}
	ctl.Append(mockTracer{})
	s := &server{
		opt: &internal_server.Options{
			TracerCtl: ctl,
		},
		svcs: &services{},
	}
	s.buildMiddlewares(context.Background())

	test.Assert(t, len(s.opt.Streaming.RecvMiddlewares) == 0, "init middlewares failed")
	test.Assert(t, len(s.opt.Streaming.SendMiddlewares) == 0, "init middlewares failed")
}

func Test_gRPCCompatibleServerStream(t *testing.T) {
	t.Run("GetGRPCStream returns wrapped Stream", func(t *testing.T) {
		mockSt := &mockStream{}
		mockGs := &mockGRPCStream{}
		wrapper := gRPCCompatibleServerStream{
			ServerStream: mockSt,
			st:           mockGs,
		}

		res := wrapper.GetGRPCStream()
		test.Assert(t, res == mockGs, res)
	})
	t.Run("GetGRPCStream returns nil when st is nil", func(t *testing.T) {
		mockSS := &mockStream{}
		wrapper := gRPCCompatibleServerStream{
			ServerStream: mockSS,
			st:           nil,
		}

		result := wrapper.GetGRPCStream()
		test.Assert(t, result == nil, result)
	})
}

func Test_gRPCCompatibleServerStream_with_middleware(t *testing.T) {
	t.Run("middleware wrapped Stream", func(t *testing.T) {
		testService := "test_service"
		testMethod := "test_method"

		hdl := func(ctx context.Context, handler, req, result interface{}) error {
			args, ok := req.(*streaming.Args)
			test.Assert(t, ok, req)
			test.Assert(t, args.Stream != nil, args)

			cs, ok := args.Stream.(contextStream)
			test.Assert(t, ok, args.Stream)
			_, ok = cs.Stream.(*wrappedStreamByMiddleware)
			test.Assert(t, ok, cs.Stream)

			return nil
		}

		methods := map[string]serviceinfo.MethodInfo{
			testMethod: serviceinfo.NewMethodInfo(
				hdl,
				nil,
				nil,
				false,
				serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
			),
		}

		svcInfo := &serviceinfo.ServiceInfo{
			ServiceName: testService,
			Methods:     methods,
		}

		mws := []endpoint.Middleware{
			func(next endpoint.Endpoint) endpoint.Endpoint {
				return func(ctx context.Context, req, resp interface{}) (err error) {
					if args, ok := req.(*streaming.Args); ok {
						// user middleware wrapping the old gRPC Stream interface
						if args.Stream != nil {
							args.Stream = &wrappedStreamByMiddleware{
								Stream: args.Stream,
							}
						}
					}
					return next(ctx, req, resp)
				}
			},
		}

		svr := newStreamingServer(svcInfo, mws)
		svr.buildInvokeChain(context.Background())

		ri := svr.initOrResetRPCInfoFunc()(nil, nil)
		ink, ok := ri.Invocation().(rpcinfo.InvocationSetter)
		test.Assert(t, ok)
		ink.SetServiceName(testService)
		ink.SetMethodName(testMethod)
		ink.SetMethodInfo(svcInfo.MethodInfo(context.Background(), testMethod))

		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		mock := &mockStream{}
		err := svr.eps(ctx, &streaming.Args{ServerStream: mock, Stream: mock.GetGRPCStream()}, nil)

		test.Assert(t, err == nil, err)
	})
	t.Run("middleware set Stream to nil", func(t *testing.T) {
		testService := "test_service"
		testMethod := "test_method"

		streamHdlr := func(ctx context.Context, handler, req, result interface{}) error {
			args, ok := req.(*streaming.Args)
			test.Assert(t, ok, req)
			test.Assert(t, args.ServerStream != nil, args)
			test.Assert(t, args.Stream == nil, args)
			return nil
		}

		methods := map[string]serviceinfo.MethodInfo{
			testMethod: serviceinfo.NewMethodInfo(
				streamHdlr,
				nil,
				nil,
				false,
				serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
			),
		}

		svcInfo := &serviceinfo.ServiceInfo{
			ServiceName: testService,
			Methods:     methods,
		}

		mws := []endpoint.Middleware{
			func(next endpoint.Endpoint) endpoint.Endpoint {
				return func(ctx context.Context, req, resp interface{}) (err error) {
					args, ok := req.(*streaming.Args)
					test.Assert(t, ok, req)
					test.Assert(t, args.ServerStream != nil)
					test.Assert(t, args.Stream != nil)
					args.Stream = nil
					return next(ctx, req, resp)
				}
			},
		}

		svr := newStreamingServer(svcInfo, mws)
		svr.buildInvokeChain(context.Background())

		ri := svr.initOrResetRPCInfoFunc()(nil, nil)
		ink, ok := ri.Invocation().(rpcinfo.InvocationSetter)
		test.Assert(t, ok)
		ink.SetServiceName(testService)
		ink.SetMethodName(testMethod)
		ink.SetMethodInfo(svcInfo.MethodInfo(context.Background(), testMethod))

		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		mock := &mockStream{}
		err := svr.eps(ctx, &streaming.Args{ServerStream: mock, Stream: mock.GetGRPCStream()}, nil)
		test.Assert(t, err == nil, err)
	})
}

// wrappedStreamByMiddleware simulates a user-wrapped stream in middleware
type wrappedStreamByMiddleware struct {
	streaming.Stream
}

// blockingServerStream is a mock ServerStream whose RecvMsg blocks until context is done.
type blockingServerStream struct {
	streaming.ServerStream
	recvCh chan struct{} // closed when recv should unblock
}

func newBlockingServerStream() *blockingServerStream {
	return &blockingServerStream{recvCh: make(chan struct{})}
}

func (s *blockingServerStream) RecvMsg(ctx context.Context, m interface{}) error {
	<-s.recvCh
	return nil
}

func (s *blockingServerStream) unblock() {
	close(s.recvCh)
}

// immediateServerStream is a mock ServerStream whose RecvMsg returns immediately.
type immediateServerStream struct {
	streaming.ServerStream
	err error
}

func (s *immediateServerStream) RecvMsg(ctx context.Context, m interface{}) error {
	return s.err
}

func newTestStream(ss streaming.ServerStream, recvTmCfg streaming.TimeoutConfig) *stream {
	recvEP := func(ctx context.Context, stream streaming.ServerStream, message interface{}) error {
		return stream.RecvMsg(ctx, message)
	}
	sendEP := func(ctx context.Context, stream streaming.ServerStream, message interface{}) error {
		return stream.SendMsg(ctx, message)
	}
	return &stream{
		ServerStream: ss,
		ctx:          context.Background(),
		ri:           rpcinfo.NewRPCInfo(nil, nil, nil, nil, nil),
		recv:         recvEP,
		send:         sendEP,
		traceCtl:     &rpcinfo.TraceController{},
		recvTmCfg:    recvTmCfg,
	}
}

func Test_stream_RecvMsg_withTimeout(t *testing.T) {
	ss := newBlockingServerStream()
	defer ss.unblock()
	st := newTestStream(ss, streaming.TimeoutConfig{Timeout: 50 * time.Millisecond})

	start := time.Now()
	err := st.RecvMsg(context.Background(), nil)
	elapsed := time.Since(start)

	test.Assert(t, err != nil, "expected timeout error")
	test.Assert(t, errors.Is(err, kerrors.ErrRPCTimeout), err)
	test.Assert(t, elapsed >= 50*time.Millisecond, elapsed)
	test.Assert(t, elapsed < 500*time.Millisecond, elapsed)
}

func Test_stream_RecvMsg_noTimeout(t *testing.T) {
	expectedErr := errors.New("test recv error")
	ss := &immediateServerStream{err: expectedErr}
	st := newTestStream(ss, streaming.TimeoutConfig{}) // no timeout

	err := st.RecvMsg(context.Background(), nil)
	test.Assert(t, err == expectedErr, err)
}

func Test_stream_RecvMsg_noTimeout_success(t *testing.T) {
	ss := &immediateServerStream{err: nil}
	st := newTestStream(ss, streaming.TimeoutConfig{}) // no timeout

	err := st.RecvMsg(context.Background(), nil)
	test.Assert(t, err == nil, err)
}

func Test_stream_RecvMsg_withTimeout_noBlock(t *testing.T) {
	ss := &immediateServerStream{err: nil}
	st := newTestStream(ss, streaming.TimeoutConfig{Timeout: 1 * time.Second})

	err := st.RecvMsg(context.Background(), nil)
	test.Assert(t, err == nil, err)
}

func Test_stream_SetRecvTimeoutConfig(t *testing.T) {
	ss := newBlockingServerStream()
	defer ss.unblock()
	st := newTestStream(ss, streaming.TimeoutConfig{}) // initially no timeout

	// Set timeout dynamically
	st.SetRecvTimeoutConfig(streaming.TimeoutConfig{Timeout: 50 * time.Millisecond})

	start := time.Now()
	err := st.RecvMsg(context.Background(), nil)
	elapsed := time.Since(start)

	test.Assert(t, errors.Is(err, kerrors.ErrRPCTimeout), err)
	test.Assert(t, elapsed >= 50*time.Millisecond, elapsed)
}

func Test_streaming_SetRecvTimeoutConfig_helper(t *testing.T) {
	ss := &immediateServerStream{}
	st := newTestStream(ss, streaming.TimeoutConfig{})

	// stream implements RecvTimeoutConfigSetter
	ok := streaming.SetRecvTimeoutConfig(st, streaming.TimeoutConfig{Timeout: 1 * time.Second})
	test.Assert(t, ok, "expected SetRecvTimeoutConfig to return true")
	test.Assert(t, st.recvTmCfg.Timeout == 1*time.Second, st.recvTmCfg)

	// plain ServerStream does not implement it
	ok = streaming.SetRecvTimeoutConfig(ss, streaming.TimeoutConfig{Timeout: 1 * time.Second})
	test.Assert(t, !ok, "expected SetRecvTimeoutConfig to return false for plain stream")
}

func Test_callWithTimeout_panic_recovery(t *testing.T) {
	tmCfg := streaming.TimeoutConfig{Timeout: 1 * time.Second}
	err := callWithTimeout(tmCfg, func() error {
		panic("test panic")
	})
	test.Assert(t, err != nil, "expected error from panic")
	test.Assert(t, !errors.Is(err, kerrors.ErrRPCTimeout), "panic error should not be ErrRPCTimeout")
}
