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

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/internal/mocks"
	mocksnet "github.com/cloudwego/kitex/internal/mocks/net"
	mock_remote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/endpoint/cep"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/remotecli"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/transport"
)

var (
	svcInfo   = mocks.ServiceInfo()
	req, resp = &streaming.Args{}, &streaming.Result{}
)

func newOpts(ctrl *gomock.Controller) []Option {
	return []Option{
		WithTransHandlerFactory(newMockCliTransHandlerFactory(ctrl)),
		WithResolver(resolver404(ctrl)),
		WithDialer(newDialer(ctrl)),
		WithDestService("destService"),
	}
}

func TestStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	var err error
	kc := &kClient{
		opt:     client.NewOptions(newOpts(ctrl)),
		svcInfo: svcInfo,
	}
	rpcinfo.AsMutableRPCConfig(kc.opt.Configs).SetTransportProtocol(transport.GRPCStreaming)

	_ = kc.init()

	err = kc.Stream(ctx, mocks.MockStreamingMethod, req, resp)
	test.Assert(t, err == nil, err)

	err = kc.Stream(ctx, mocks.MockMethod, req, resp)
	test.Assert(t, err.Error() == "internal exception: not a streaming method, service: MockService, method: mock")

	_, err = kc.StreamX(ctx, mocks.MockStreamingMethod)
	test.Assert(t, err == nil, err)

	_, err = kc.StreamX(ctx, mocks.MockMethod)
	test.Assert(t, err.Error() == "internal exception: not a streaming method, service: MockService, method: mock")
}

func TestStreamNoMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	kc := &kClient{
		opt:     client.NewOptions(newOpts(ctrl)),
		svcInfo: svcInfo,
	}
	_ = kc.init()
	_, err := kc.StreamX(ctx, "mock_method_not_found")
	test.Assert(t, err.Error() == "internal exception: non-existent method, service: MockService, method: mock_method_not_found")

	err = kc.Stream(ctx, "mock_method_not_found", req, resp)
	test.Assert(t, err.Error() == "internal exception: non-existent method, service: MockService, method: mock_method_not_found")
}

func TestStreaming(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	kc := &kClient{
		opt:     client.NewOptions(newOpts(ctrl)),
		svcInfo: svcInfo,
	}
	mockRPCInfo := rpcinfo.NewRPCInfo(
		rpcinfo.NewEndpointInfo("mock_client", "mock_client_method", nil, nil),
		rpcinfo.NewEndpointInfo(
			"mock_server", "mockserver_method",
			utils.NewNetAddr(
				"mock_network", "mock_addr",
			), nil,
		),
		rpcinfo.NewInvocation("mock_service", "mock_method"),
		rpcinfo.NewRPCConfig(),
		rpcinfo.NewRPCStats(),
	)
	rpcinfo.AsMutableRPCConfig(mockRPCInfo.Config()).SetTransportProtocol(transport.GRPC)
	ctx = rpcinfo.NewCtxWithRPCInfo(context.Background(), mockRPCInfo)

	cliInfo := new(remote.ClientOption)
	cliInfo.SvcInfo = svcInfo
	conn := mocksnet.NewMockConn(ctrl)
	conn.EXPECT().Close().Return(nil).AnyTimes()
	connpool := mock_remote.NewMockConnPool(ctrl)
	connpool.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(conn, nil)
	cliInfo.ConnPool = connpool
	s, cr, _ := remotecli.NewStream(ctx, mockRPCInfo, new(mocks.MockCliTransHandler), cliInfo)
	stream := newStream(ctx,
		s, cr, kc, mockRPCInfo, serviceinfo.StreamingBidirectional,
		func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return stream.SendMsg(ctx, message)
		},
		func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return stream.RecvMsg(ctx, message)
		},
		func(stream streaming.Stream, message interface{}) (err error) {
			return stream.SendMsg(message)
		},
		func(stream streaming.Stream, message interface{}) (err error) {
			return stream.RecvMsg(message)
		},
	)

	for i := 0; i < 10; i++ {
		ch := make(chan struct{})
		// recv nil msg
		go func() {
			err := stream.RecvMsg(context.Background(), nil)
			test.Assert(t, err == nil, err)
			ch <- struct{}{}
		}()

		// send nil msg
		go func() {
			err := stream.SendMsg(context.Background(), nil)
			test.Assert(t, err == nil, err)
			ch <- struct{}{}
		}()
		<-ch
		<-ch
	}
}

func TestUninitClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	kc := &kClient{
		opt:     client.NewOptions(newOpts(ctrl)),
		svcInfo: svcInfo,
	}

	test.PanicAt(t, func() {
		_ = kc.Stream(ctx, "mock_method", req, resp)
	}, func(err interface{}) bool {
		return err.(string) == "client not initialized"
	})
}

func TestClosedClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()

	kc := &kClient{
		opt:     client.NewOptions(newOpts(ctrl)),
		svcInfo: svcInfo,
	}
	_ = kc.init()
	_ = kc.Close()

	test.PanicAt(t, func() {
		_ = kc.Stream(ctx, "mock_method", req, resp)
	}, func(err interface{}) bool {
		return err.(string) == "client is already closed"
	})
}

type mockStream struct {
	streaming.ClientStream
	ctx           context.Context
	close         func() error
	header        func() (streaming.Header, error)
	recv          func(ctx context.Context, msg interface{}) error
	send          func(ctx context.Context, msg interface{}) error
	cancelWithErr func(err error)
}

func (s *mockStream) Context() context.Context {
	return s.ctx
}

func (s *mockStream) Header() (streaming.Header, error) {
	return s.header()
}

func (s *mockStream) RecvMsg(ctx context.Context, msg any) error {
	return s.recv(ctx, msg)
}

func (s *mockStream) SendMsg(ctx context.Context, msg any) error {
	return s.send(ctx, msg)
}

func (s *mockStream) CloseSend(ctx context.Context) error {
	return s.close()
}

func (s *mockStream) CancelWithErr(err error) {
	s.cancelWithErr(err)
}

func (s *mockStream) Close() error {
	if s.close != nil {
		return s.close()
	}
	return nil
}

func Test_newStream(t *testing.T) {
	sendErr := errors.New("send error")
	recvErr := errors.New("recv error")
	ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	st := &mockStream{
		ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
	}
	kc := &kClient{
		opt: &client.Options{
			TracerCtl: &rpcinfo.TraceController{},
		},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cr := mock_remote.NewMockConnReleaser(ctrl)
	cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
	scr := remotecli.NewStreamConnManager(cr)
	s := newStream(ctx,
		st,
		scr,
		kc,
		ri,
		serviceinfo.StreamingClient,
		func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return sendErr
		}, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return recvErr
		},
		func(stream streaming.Stream, message interface{}) (err error) {
			return sendErr
		}, func(stream streaming.Stream, message interface{}) (err error) {
			return recvErr
		},
	)

	test.Assert(t, s.ClientStream == st)
	test.Assert(t, s.kc == kc)
	test.Assert(t, s.streamingMode == serviceinfo.StreamingClient)
	test.Assert(t, s.SendMsg(context.Background(), nil) == sendErr)
	test.Assert(t, s.RecvMsg(context.Background(), nil) == recvErr)
	test.Assert(t, s.recvTmCfg.Timeout == 0, s.recvTmCfg)
	test.Assert(t, !s.recvTmCfg.DisableCancelRemote, s.recvTmCfg)
}

type mockTracer struct {
	stats.Tracer
	start  func(ctx context.Context) context.Context
	finish func(ctx context.Context)
}

func (m *mockTracer) Start(ctx context.Context) context.Context {
	return m.start(ctx)
}

func (m *mockTracer) Finish(ctx context.Context) {
	m.finish(ctx)
}

func Test_stream_Header(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("no-error", func(t *testing.T) {
		headers := map[string]string{"k": "v"}
		st := &mockStream{
			header: func() (streaming.Header, error) {
				return headers, nil
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scr := remotecli.NewStreamConnManager(cr)
		s := newStream(ctx, st, scr, &kClient{}, rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil), serviceinfo.StreamingBidirectional, nil, nil, nil, nil)
		md, err := s.Header()

		test.Assert(t, err == nil)
		test.Assert(t, len(md) == 1, md)
		test.Assert(t, md["k"] == "v", md)
	})

	t.Run("error", func(t *testing.T) {
		headerErr := errors.New("header error")
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		st := &mockStream{
			header: func() (streaming.Header, error) {
				return nil, headerErr
			},
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
		}
		finishCalled := false
		tracer := &mockTracer{
			finish: func(ctx context.Context) {
				finishCalled = true
			},
		}
		ctl := &rpcinfo.TraceController{}
		ctl.Append(tracer)
		kc := &kClient{
			opt: &client.Options{
				TracerCtl: ctl,
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scr := remotecli.NewStreamConnManager(cr)
		s := newStream(ctx, st, scr, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil)
		md, err := s.Header()

		test.Assert(t, err == headerErr)
		test.Assert(t, md == nil)
		test.Assert(t, finishCalled)
	})
}

func Test_stream_RecvMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("no-error", func(t *testing.T) {
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scm := remotecli.NewStreamConnManager(cr)
		mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), rpcinfo.NewRPCConfig(), nil)
		kc := &kClient{
			opt: client.NewOptions(nil),
		}
		s := newStream(ctx, &mockStream{}, scm, kc, mockRPCInfo, serviceinfo.StreamingBidirectional, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return nil
		}, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				return nil
			},
		)

		err := s.RecvMsg(context.Background(), nil)

		test.Assert(t, err == nil)
	})

	t.Run("no-error-client-streaming", func(t *testing.T) {
		ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
		}
		finishCalled := false
		tracer := &mockTracer{
			finish: func(ctx context.Context) {
				finishCalled = true
			},
		}
		ctl := &rpcinfo.TraceController{}
		ctl.Append(tracer)
		kc := &kClient{
			opt: &client.Options{
				TracerCtl: ctl,
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		// client streaming should release connection after RecvMsg
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		s := newStream(ctx, st, scm, kc, ri, serviceinfo.StreamingClient, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return nil
		}, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				return nil
			},
		)
		err := s.RecvMsg(context.Background(), nil)

		test.Assert(t, err == nil)
		test.Assert(t, finishCalled)
	})

	t.Run("error", func(t *testing.T) {
		recvErr := errors.New("recv error")
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
		}
		finishCalled := false
		tracer := &mockTracer{
			finish: func(ctx context.Context) {
				finishCalled = true
			},
		}
		ctl := &rpcinfo.TraceController{}
		ctl.Append(tracer)
		kc := &kClient{
			opt: &client.Options{
				TracerCtl: ctl,
			},
		}

		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		s := newStream(ctx, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return recvErr
		}, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				return recvErr
			},
		)
		err := s.RecvMsg(context.Background(), nil)

		test.Assert(t, err == recvErr)
		test.Assert(t, finishCalled)
	})
}

func Test_stream_SendMsg(t *testing.T) {
	t.Run("no-error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scm := remotecli.NewStreamConnManager(cr)
		kc := &kClient{
			opt: client.NewOptions(nil),
		}
		s := newStream(ctx, &mockStream{}, scm, kc, rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil), serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return nil
		}, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				return nil
			},
			nil,
		)

		err := s.SendMsg(context.Background(), nil)

		test.Assert(t, err == nil)
	})

	t.Run("error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		sendErr := errors.New("recv error")
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
		}
		finishCalled := false
		tracer := &mockTracer{
			finish: func(ctx context.Context) {
				finishCalled = true
			},
		}
		ctl := &rpcinfo.TraceController{}
		ctl.Append(tracer)
		kc := &kClient{
			opt: &client.Options{
				TracerCtl: ctl,
			},
		}

		s := newStream(ctx, st, scm, kc, ri, serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return sendErr
		}, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				return sendErr
			},
			nil,
		)
		err := s.SendMsg(context.Background(), nil)

		test.Assert(t, err == sendErr)
		test.Assert(t, finishCalled)
	})
}

func Test_stream_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cr := mock_remote.NewMockConnReleaser(ctrl)
	cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
	scm := remotecli.NewStreamConnManager(cr)
	called := false
	s := newStream(ctx, &mockStream{
		close: func() error {
			called = true
			return nil
		},
	}, scm, &kClient{}, rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil), serviceinfo.StreamingBidirectional, nil, nil, nil, nil)

	err := s.CloseSend(context.Background())

	test.Assert(t, err == nil)
	test.Assert(t, called)
}

func Test_stream_DoFinish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("no-error", func(t *testing.T) {
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
		}
		tracer := &mockTracer{}
		ctl := &rpcinfo.TraceController{}
		ctl.Append(tracer)
		kc := &kClient{
			opt: &client.Options{
				TracerCtl: ctl,
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		s := newStream(ctx, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil)

		finishCalled := false
		err := errors.New("any err")
		tracer.finish = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			err = ri.Stats().Error()
			finishCalled = true
		}
		s.DoFinish(nil)
		test.Assert(t, finishCalled)
		test.Assert(t, err == nil)
	})

	t.Run("EOF", func(t *testing.T) {
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
		}
		tracer := &mockTracer{}
		ctl := &rpcinfo.TraceController{}
		ctl.Append(tracer)
		kc := &kClient{
			opt: &client.Options{
				TracerCtl: ctl,
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		s := newStream(st.ctx, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil)

		finishCalled := false
		err := errors.New("any err")
		tracer.finish = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			err = ri.Stats().Error()
			finishCalled = true
		}
		s.DoFinish(io.EOF)
		test.Assert(t, finishCalled)
		test.Assert(t, err == nil)
	})

	t.Run("biz-status-error", func(t *testing.T) {
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
		}
		tracer := &mockTracer{}
		ctl := &rpcinfo.TraceController{}
		ctl.Append(tracer)
		kc := &kClient{
			opt: &client.Options{
				TracerCtl: ctl,
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		s := newStream(ctx, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil)

		finishCalled := false
		var err error
		tracer.finish = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			err = ri.Stats().Error()
			finishCalled = true
		}
		s.DoFinish(kerrors.NewBizStatusError(100, "biz status error"))
		test.Assert(t, finishCalled)
		test.Assert(t, err == nil) // biz status error is not an rpc error
	})

	t.Run("error", func(t *testing.T) {
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
		}
		tracer := &mockTracer{}
		ctl := &rpcinfo.TraceController{}
		ctl.Append(tracer)
		kc := &kClient{
			opt: &client.Options{
				TracerCtl: ctl,
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		s := newStream(st.ctx, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil)

		finishCalled := false
		expectedErr := errors.New("error")
		var err error
		tracer.finish = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			err = ri.Stats().Error()
			finishCalled = true
		}
		s.DoFinish(expectedErr)
		test.Assert(t, finishCalled)
		test.Assert(t, err == expectedErr)
	})
}

func Test_isRPCError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		test.Assert(t, !isRPCError(nil))
	})
	t.Run("EOF", func(t *testing.T) {
		test.Assert(t, !isRPCError(io.EOF))
	})
	t.Run("biz status error", func(t *testing.T) {
		test.Assert(t, !isRPCError(kerrors.NewBizStatusError(100, "biz status error")))
	})
	t.Run("error", func(t *testing.T) {
		test.Assert(t, isRPCError(errors.New("error")))
	})
}

func TestContextFallback(t *testing.T) {
	mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), rpcinfo.NewRPCConfig(), nil)
	mockSt := &mockStream{
		recv: func(ctx context.Context, message interface{}) error {
			test.Assert(t, ctx == context.Background())
			return nil
		},
		send: func(ctx context.Context, message interface{}) error {
			test.Assert(t, ctx == context.Background())
			return nil
		},
	}
	kc := &kClient{
		opt: client.NewOptions(nil),
	}
	st := newStream(context.Background(), mockSt, nil, kc, mockRPCInfo, serviceinfo.StreamingBidirectional, sendEndpoint, recvEndpoint, nil, nil)
	err := st.RecvMsg(context.Background(), nil)
	test.Assert(t, err == nil)
	err = st.SendMsg(context.Background(), nil)
	test.Assert(t, err == nil)

	mockSt = &mockStream{
		recv: func(ctx context.Context, message interface{}) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			test.Assert(t, ri == mockRPCInfo)
			return nil
		},
		send: func(ctx context.Context, message interface{}) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			test.Assert(t, ri == mockRPCInfo)
			return nil
		},
	}
	st = newStream(context.Background(), mockSt, nil, kc, mockRPCInfo, serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
		ri := rpcinfo.GetRPCInfo(ctx)
		test.Assert(t, ri == mockRPCInfo)
		return sendEndpoint(ctx, stream, message)
	}, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
		ri := rpcinfo.GetRPCInfo(ctx)
		test.Assert(t, ri == mockRPCInfo)
		return recvEndpoint(ctx, stream, message)
	}, nil, nil)
	err = st.RecvMsg(context.Background(), nil)
	test.Assert(t, err == nil)
	err = st.SendMsg(context.Background(), nil)
	test.Assert(t, err == nil)
}

// createErrorMiddleware creates a streaming middleware that injects an error
func createErrorMiddleware(injectErr error) cep.StreamMiddleware {
	return func(next cep.StreamEndpoint) cep.StreamEndpoint {
		return func(ctx context.Context) (streaming.ClientStream, error) {
			return nil, injectErr
		}
	}
}

// TestStreamDoFinish tests if DoFinish is correctly called in Stream method
func TestStreamDoFinish(t *testing.T) {
	// Create test error
	testErr := errors.New("test stream error")

	// Create mock Tracer
	finishCalled := false
	tracer := &mockTracer{
		start: func(ctx context.Context) context.Context {
			return ctx
		},
		finish: func(ctx context.Context) {
			finishCalled = true
			fmt.Printf("finishCalled set to true\n")
		},
	}

	// Create client
	cli, err := NewClient(mocks.ServiceInfo(),
		WithTracer(tracer),
		WithTransportProtocol(transport.TTHeaderStreaming),
		WithDestService("MockService"),
		WithStreamOptions(
			WithStreamMiddleware(createErrorMiddleware(testErr)),
		),
	)
	test.Assert(t, err == nil, err)

	// Get Streaming interface
	streamingCli := cli.(Streaming)

	// Call Stream method
	result := &streaming.Result{}
	err = streamingCli.Stream(context.Background(), "mockStreaming", nil, result)

	// Check if test error is correctly returned
	test.Assert(t, err == testErr, err)

	// Check if DoFinish is called
	test.Assert(t, finishCalled, "DoFinish was not called")
	fmt.Printf("Final finishCalled status: %v\n", finishCalled)
}

// TestStreamXDoFinish tests if DoFinish is correctly called in StreamX method
func TestStreamXDoFinish(t *testing.T) {
	// Create test error
	testErr := errors.New("test streamX error")

	// Create mock Tracer
	finishCalled := false
	tracer := &mockTracer{
		start: func(ctx context.Context) context.Context {
			return ctx
		},
		finish: func(ctx context.Context) {
			finishCalled = true
			fmt.Printf("finishCalled set to true\n")
		},
	}
	ctl := &rpcinfo.TraceController{}
	ctl.Append(tracer)

	// Create client
	cli, err := NewClient(mocks.ServiceInfo(),
		WithTracer(tracer),
		WithTransportProtocol(transport.TTHeaderStreaming),
		WithDestService("MockService"),
		WithStreamOptions(
			WithStreamMiddleware(createErrorMiddleware(testErr)),
		),
	)
	test.Assert(t, err == nil, err)

	// Get Streaming interface
	streamingCli := cli.(Streaming)

	// Call StreamX method
	_, err = streamingCli.StreamX(context.Background(), "mockStreaming")
	test.Assert(t, err == testErr, err)

	// Check if DoFinish is called
	test.Assert(t, finishCalled, "DoFinish was not called")
	fmt.Printf("Final finishCalled status: %v\n", finishCalled)
}

// TestRecvTimeout tests recv timeout scenarios
func TestRecvTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("no timeout set", func(t *testing.T) {
		// no timeout, recv should not timeout even if it takes time
		recvCalled := false
		ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
			recv: func(ctx context.Context, msg interface{}) error {
				recvCalled = true
				time.Sleep(50 * time.Millisecond)
				return nil
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scm := remotecli.NewStreamConnManager(cr)
		kc := &kClient{
			opt: client.NewOptions(nil),
		}
		s := newStream(st.ctx, st, scm, kc, ri, serviceinfo.StreamingBidirectional,
			sendEndpoint, recvEndpoint, nil, nil)

		err := s.RecvMsg(context.Background(), nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, recvCalled)
	})

	t.Run("timeout set but finish normally", func(t *testing.T) {
		// timeout set to 100ms, but recv finishes in 20ms
		recvCalled := false
		cfg := rpcinfo.NewRPCConfig()
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeoutConfig(streaming.TimeoutConfig{
			Timeout:             100 * time.Millisecond,
			DisableCancelRemote: false,
		})
		ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
			recv: func(ctx context.Context, msg interface{}) error {
				recvCalled = true
				time.Sleep(20 * time.Millisecond)
				return nil
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scm := remotecli.NewStreamConnManager(cr)
		kc := &kClient{
			opt: client.NewOptions(nil),
		}
		s := newStream(st.ctx, st, scm, kc, ri, serviceinfo.StreamingBidirectional,
			sendEndpoint, recvEndpoint, nil, nil)

		err := s.RecvMsg(context.Background(), nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, recvCalled)
	})

	t.Run("timeout and cancel remote", func(t *testing.T) {
		// timeout set to 50ms, recv takes 200ms, should timeout and cancel remote
		var recvCalled, cancelCalled atomic.Bool
		cfg := rpcinfo.NewRPCConfig()
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeoutConfig(streaming.TimeoutConfig{
			Timeout:             50 * time.Millisecond,
			DisableCancelRemote: false,
		})
		ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
			recv: func(ctx context.Context, msg interface{}) error {
				recvCalled.Store(true)
				time.Sleep(200 * time.Millisecond)
				return nil
			},
			cancelWithErr: func(err error) {
				cancelCalled.Store(true)
				test.Assert(t, err != nil, "cancel error should not be nil")
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		kc := &kClient{
			opt: client.NewOptions(nil),
		}
		// create a grpc stream wrapper to enable timeout logic
		grpcInner := &mockGRPCInnerStream{
			recv: func(msg interface{}) error {
				// This will be called by the recv endpoint
				return st.recv(context.Background(), msg)
			},
			ctx: st.ctx,
		}
		grpcSt := &mockGRPCStreamWrapper{
			mockStream: st,
			grpcStream: grpcInner,
		}

		s := newStream(st.ctx, grpcSt, scm, kc, ri, serviceinfo.StreamingBidirectional,
			sendEndpoint, recvEndpoint,
			func(stream streaming.Stream, msg interface{}) error {
				return nil
			},
			func(stream streaming.Stream, msg interface{}) error {
				return stream.RecvMsg(msg)
			})

		err := s.RecvMsg(context.Background(), nil)
		test.Assert(t, err != nil)
		stat, ok := status.FromError(err)
		test.Assert(t, ok, err)
		test.Assert(t, stat.Code() == codes.RecvDeadlineExceeded, stat)
		test.Assert(t, strings.Contains(stat.Message(), "stream Recv timeout"), stat.Message())
		test.Assert(t, cancelCalled.Load())
		test.Assert(t, recvCalled.Load())
	})

	t.Run("timeout but no cancel remote", func(t *testing.T) {
		// timeout set to 50ms with DisableCancelRemote=true, recv takes 200ms
		// should timeout but NOT cancel remote
		var recvCalled, cancelCalled atomic.Bool
		cfg := rpcinfo.NewRPCConfig()
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeoutConfig(streaming.TimeoutConfig{
			Timeout:             50 * time.Millisecond,
			DisableCancelRemote: true,
		})
		ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
			recv: func(ctx context.Context, msg interface{}) error {
				recvCalled.Store(true)
				time.Sleep(200 * time.Millisecond)
				return nil
			},
			cancelWithErr: func(err error) {
				cancelCalled.Store(true)
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		kc := &kClient{
			opt: client.NewOptions(nil),
		}
		// create a grpc stream wrapper to enable timeout logic
		grpcInner := &mockGRPCInnerStream{
			recv: func(msg interface{}) error {
				// This will be called by the recv endpoint
				return st.recv(context.Background(), msg)
			},
			ctx: st.ctx,
		}
		grpcSt := &mockGRPCStreamWrapper{
			mockStream: st,
			grpcStream: grpcInner,
		}

		s := newStream(st.ctx, grpcSt, scm, kc, ri, serviceinfo.StreamingBidirectional,
			sendEndpoint, recvEndpoint,
			func(stream streaming.Stream, msg interface{}) error {
				return nil
			},
			func(stream streaming.Stream, msg interface{}) error {
				return stream.RecvMsg(msg)
			})

		err := s.RecvMsg(context.Background(), nil)
		stat, ok := status.FromError(err)
		test.Assert(t, ok, err)
		test.Assert(t, stat.Code() == codes.RecvDeadlineExceeded, stat)
		test.Assert(t, strings.Contains(stat.Message(), "stream Recv timeout"), stat.Message())
		test.Assert(t, err != nil)
		test.Assert(t, !cancelCalled.Load())
		test.Assert(t, recvCalled.Load())
	})

	t.Run("panic in recv and recovered", func(t *testing.T) {
		// recv panics, should be recovered and converted to error
		var recvCalled, cancelCalled atomic.Bool
		cfg := rpcinfo.NewRPCConfig()
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeoutConfig(streaming.TimeoutConfig{
			Timeout:             100 * time.Millisecond,
			DisableCancelRemote: false,
		})
		ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, rpcinfo.NewRPCStats())
		st := &mockStream{
			ctx: rpcinfo.NewCtxWithRPCInfo(context.Background(), ri),
			recv: func(ctx context.Context, msg interface{}) error {
				recvCalled.Store(true)
				panic("test panic in recv")
			},
			cancelWithErr: func(err error) {
				cancelCalled.Store(true)
				test.Assert(t, err != nil, "cancel error should not be nil")
			},
		}
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)
		scm := remotecli.NewStreamConnManager(cr)
		kc := &kClient{
			opt: client.NewOptions(nil),
		}
		// create a grpc stream wrapper to enable timeout logic
		grpcInner := &mockGRPCInnerStream{
			recv: func(msg interface{}) error {
				// This will be called by the recv endpoint
				return st.recv(context.Background(), msg)
			},
			ctx: st.ctx,
		}
		grpcSt := &mockGRPCStreamWrapper{
			mockStream: st,
			grpcStream: grpcInner,
		}

		s := newStream(st.ctx, grpcSt, scm, kc, ri, serviceinfo.StreamingBidirectional,
			sendEndpoint, recvEndpoint,
			func(stream streaming.Stream, msg interface{}) error {
				return nil
			},
			func(stream streaming.Stream, msg interface{}) error {
				return stream.RecvMsg(msg)
			})

		err := s.RecvMsg(context.Background(), nil)
		test.Assert(t, err != nil)
		stat, ok := status.FromError(err)
		test.Assert(t, ok, err)
		test.Assert(t, stat.Code() == codes.Internal, stat)
		test.Assert(t, strings.Contains(stat.Message(), "stream Recv panic"), stat.Message())
		test.Assert(t, strings.Contains(stat.Message(), "test panic in recv"), stat.Message())
		test.Assert(t, cancelCalled.Load())
		test.Assert(t, recvCalled.Load())
	})
}

// mockGRPCStreamWrapper wraps mockStream to implement GRPCStreamGetter interface
type mockGRPCStreamWrapper struct {
	*mockStream
	grpcStream *mockGRPCInnerStream
}

func (s *mockGRPCStreamWrapper) GetGRPCStream() streaming.Stream {
	return s.grpcStream
}

// mockGRPCInnerStream implements streaming.Stream interface for gRPC
type mockGRPCInnerStream struct {
	recv func(msg interface{}) error
	ctx  context.Context
}

func (s *mockGRPCInnerStream) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func (s *mockGRPCInnerStream) SetHeader(md metadata.MD) error {
	return nil
}

func (s *mockGRPCInnerStream) SendHeader(md metadata.MD) error {
	return nil
}

func (s *mockGRPCInnerStream) SetTrailer(md metadata.MD) {
}

func (s *mockGRPCInnerStream) Header() (metadata.MD, error) {
	return metadata.MD{}, nil
}

func (s *mockGRPCInnerStream) Trailer() metadata.MD {
	return metadata.MD{}
}

func (s *mockGRPCInnerStream) RecvMsg(m interface{}) error {
	if s.recv != nil {
		return s.recv(m)
	}
	return nil
}

func (s *mockGRPCInnerStream) SendMsg(m interface{}) error {
	return nil
}

func (s *mockGRPCInnerStream) Close() error {
	return nil
}
