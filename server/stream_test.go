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
	"testing"

	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/endpoint"
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
