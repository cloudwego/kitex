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
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/remote/trans/ttstream"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
	streaming_types "github.com/cloudwego/kitex/pkg/streaming/types"
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
	stream := newStream(ctx, nil,
		s, cr, kc, mockRPCInfo, serviceinfo.StreamingBidirectional,
		func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return stream.SendMsg(ctx, message)
		},
		func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return stream.RecvMsg(ctx, message)
		},
		nil,
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
	ctx        context.Context
	close      func() error
	header     func() (streaming.Header, error)
	recv       func(ctx context.Context, msg interface{}) error
	send       func(ctx context.Context, msg interface{}) error
	cancel     func(err error)
	gRPCStream *mockGRPCStream
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

func (s *mockStream) Cancel(err error) {
	s.cancel(err)
}

func (s *mockStream) GetGRPCStream() streaming.Stream {
	return s.gRPCStream
}

type mockGRPCStream struct {
	streaming.Stream
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
	s := newStream(ctx, nil,
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
		nil,
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
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil)
		s := newStream(ctx, nil, st, scr, &kClient{}, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil, nil)
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
		s := newStream(ctx, nil, st, scr, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil, nil)
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
		s := newStream(ctx, nil, &mockStream{}, scm, &kClient{}, mockRPCInfo, serviceinfo.StreamingBidirectional, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return nil
		}, nil, nil,
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
		s := newStream(ctx, nil, st, scm, kc, ri, serviceinfo.StreamingClient, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return nil
		}, nil, nil,
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
		s := newStream(ctx, nil, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return recvErr
		}, nil, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				return recvErr
			},
		)
		err := s.RecvMsg(context.Background(), nil)

		test.Assert(t, err == recvErr)
		test.Assert(t, finishCalled)
	})

	t.Run("gRPC recv in time", func(t *testing.T) {
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scm := remotecli.NewStreamConnManager(cr)
		cfg := rpcinfo.NewRPCConfig()
		tm := 200 * time.Millisecond
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeout(tm)
		rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.GRPC)
		mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, nil)
		st := &mockStream{gRPCStream: &mockGRPCStream{}}
		s := newStream(ctx, nil, st, scm, &kClient{}, mockRPCInfo, serviceinfo.StreamingBidirectional, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			// mock recv time
			time.Sleep(100 * time.Millisecond)
			return nil
		}, nil, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				// mock recv time
				time.Sleep(100 * time.Millisecond)
				return nil
			},
		)

		err := s.RecvMsg(context.Background(), nil)
		test.Assert(t, err == nil)
		oldS := s.GetGRPCStream()
		err = oldS.RecvMsg(nil)
		test.Assert(t, err == nil)
	})

	t.Run("gRPC recv timeout", func(t *testing.T) {
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
		cfg := rpcinfo.NewRPCConfig()
		tm := 200 * time.Millisecond
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeout(tm)
		rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.GRPC)
		mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, rpcinfo.NewRPCStats())

		checkGRPCStatus := func(err error) {
			gRPCSt, ok := status.FromError(err)
			test.Assert(t, ok)
			test.Assert(t, gRPCSt != nil)
			test.Assert(t, gRPCSt.Code() == codes.DeadlineExceeded, gRPCSt.Code())
			test.Assert(t, strings.Contains(gRPCSt.Message(), "stream Recv timeout, timeout config="), gRPCSt.Message())
			test.Assert(t, gRPCSt.TimeoutType() == streaming_types.StreamRecvTimeout, gRPCSt.TimeoutType())
		}
		st := &mockStream{
			cancel: func(err error) {
				checkGRPCStatus(err)
			},
			gRPCStream: &mockGRPCStream{},
		}
		s := newStream(ctx, nil, st, scm, kc, mockRPCInfo, serviceinfo.StreamingBidirectional, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			// mock recv timeout
			time.Sleep(400 * time.Millisecond)
			return nil
		}, nil, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				// mock recv timeout
				time.Sleep(400 * time.Millisecond)
				return nil
			},
		)

		err := s.RecvMsg(context.Background(), nil)
		test.Assert(t, err != nil)
		checkGRPCStatus(err)
		oldS := s.GetGRPCStream()
		err = oldS.RecvMsg(nil)
		test.Assert(t, err != nil)
		checkGRPCStatus(err)
		test.Assert(t, finishCalled)
	})

	t.Run("ttstream recv in time", func(t *testing.T) {
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scm := remotecli.NewStreamConnManager(cr)
		cfg := rpcinfo.NewRPCConfig()
		tm := 200 * time.Millisecond
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeout(tm)
		rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.TTHeaderStreaming)
		mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, nil)
		s := newStream(ctx, nil, &mockStream{}, scm, &kClient{}, mockRPCInfo, serviceinfo.StreamingBidirectional, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			time.Sleep(100 * time.Millisecond)
			return nil
		}, nil, nil, nil)
		err := s.RecvMsg(context.Background(), nil)
		test.Assert(t, err == nil)
	})

	t.Run("ttstream recv timeout", func(t *testing.T) {
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
		cfg := rpcinfo.NewRPCConfig()
		tm := 200 * time.Millisecond
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeout(tm)
		rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.TTHeaderStreaming)
		mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, rpcinfo.NewRPCStats())
		checkTTStreamException := func(err error) {
			ttEx, ok := err.(*ttstream.Exception)
			test.Assert(t, ok, fmt.Sprintf("expected ttstream.Exception, got %T", err))
			test.Assert(t, ttEx != nil)
			test.Assert(t, errors.Is(err, kerrors.ErrStreamingTimeout))
			test.Assert(t, strings.Contains(err.Error(), "stream Recv timeout, timeout config="), err.Error())
		}
		st := &mockStream{
			cancel: func(err error) {
				checkTTStreamException(err)
			},
		}
		s := newStream(ctx, nil, st, scm, kc, mockRPCInfo, serviceinfo.StreamingBidirectional, nil, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			time.Sleep(400 * time.Millisecond)
			return nil
		}, nil, nil, nil)
		err := s.RecvMsg(context.Background(), nil)
		test.Assert(t, err != nil)
		checkTTStreamException(err)
		test.Assert(t, finishCalled)
	})
}

func Test_stream_SendMsg(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("no-error", func(t *testing.T) {
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scm := remotecli.NewStreamConnManager(cr)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil)
		s := newStream(ctx, nil, &mockStream{}, scm, &kClient{}, ri, serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return nil
		}, nil, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				return nil
			},
			nil,
		)

		err := s.SendMsg(context.Background(), nil)

		test.Assert(t, err == nil)
	})

	t.Run("error", func(t *testing.T) {
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

		s := newStream(ctx, nil, st, scm, kc, ri, serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			return sendErr
		}, nil, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				return sendErr
			},
			nil,
		)
		err := s.SendMsg(context.Background(), nil)

		test.Assert(t, err == sendErr)
		test.Assert(t, finishCalled)
	})

	t.Run("gRPC send in time", func(t *testing.T) {
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scm := remotecli.NewStreamConnManager(cr)
		cfg := rpcinfo.NewRPCConfig()
		tm := 200 * time.Millisecond
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamSendTimeout(tm)
		rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.GRPC)
		mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, nil)
		st := &mockStream{gRPCStream: &mockGRPCStream{}}
		s := newStream(ctx, nil, st, scm, &kClient{}, mockRPCInfo, serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			time.Sleep(100 * time.Millisecond)
			return nil
		}, nil, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				time.Sleep(100 * time.Millisecond)
				return nil
			},
			nil,
		)
		err := s.SendMsg(context.Background(), nil)
		test.Assert(t, err == nil)
		oldS := s.GetGRPCStream()
		err = oldS.SendMsg(nil)
		test.Assert(t, err == nil)
	})

	t.Run("gRPC send timeout", func(t *testing.T) {
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
		cfg := rpcinfo.NewRPCConfig()
		tm := 200 * time.Millisecond
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamSendTimeout(tm)
		rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.GRPC)
		mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, rpcinfo.NewRPCStats())
		checkGRPCStatus := func(err error) {
			gRPCSt, ok := status.FromError(err)
			test.Assert(t, ok)
			test.Assert(t, gRPCSt != nil)
			test.Assert(t, gRPCSt.Code() == codes.DeadlineExceeded, gRPCSt.Code())
			test.Assert(t, strings.Contains(gRPCSt.Message(), "stream Send timeout, timeout config="), gRPCSt.Message())
			test.Assert(t, gRPCSt.TimeoutType() == streaming_types.StreamSendTimeout, gRPCSt.TimeoutType())
		}
		st := &mockStream{
			cancel: func(err error) {
				checkGRPCStatus(err)
			},
			gRPCStream: &mockGRPCStream{},
		}
		s := newStream(ctx, nil, st, scm, kc, mockRPCInfo, serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			time.Sleep(400 * time.Millisecond)
			return nil
		}, nil, nil,
			func(stream streaming.Stream, message interface{}) (err error) {
				time.Sleep(400 * time.Millisecond)
				return nil
			},
			nil,
		)
		err := s.SendMsg(context.Background(), nil)
		test.Assert(t, err != nil)
		checkGRPCStatus(err)
		oldS := s.GetGRPCStream()
		err = oldS.SendMsg(nil)
		test.Assert(t, err != nil)
		checkGRPCStatus(err)
		test.Assert(t, finishCalled)
	})

	t.Run("ttstream send in time", func(t *testing.T) {
		cr := mock_remote.NewMockConnReleaser(ctrl)
		cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(0)
		scm := remotecli.NewStreamConnManager(cr)
		cfg := rpcinfo.NewRPCConfig()
		tm := 200 * time.Millisecond
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamSendTimeout(tm)
		rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.TTHeaderStreaming)
		mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, nil)
		s := newStream(ctx, nil, &mockStream{}, scm, &kClient{}, mockRPCInfo, serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			time.Sleep(100 * time.Millisecond)
			return nil
		}, nil, nil, nil, nil)
		err := s.SendMsg(context.Background(), nil)
		test.Assert(t, err == nil)
	})

	t.Run("ttstream send timeout", func(t *testing.T) {
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
		cfg := rpcinfo.NewRPCConfig()
		tm := 200 * time.Millisecond
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamSendTimeout(tm)
		rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.TTHeaderStreaming)
		mockRPCInfo := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("mock_service", "mock_method"), cfg, rpcinfo.NewRPCStats())
		checkTTStreamException := func(err error) {
			ttEx, ok := err.(*ttstream.Exception)
			test.Assert(t, ok, fmt.Sprintf("expected ttstream.Exception, got %T", err))
			test.Assert(t, ttEx != nil)
			test.Assert(t, errors.Is(err, kerrors.ErrStreamingTimeout))
			test.Assert(t, strings.Contains(err.Error(), "stream Send timeout, timeout config="), err.Error())
		}
		st := &mockStream{
			cancel: func(err error) {
				checkTTStreamException(err)
			},
		}
		s := newStream(ctx, nil, st, scm, kc, mockRPCInfo, serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
			time.Sleep(400 * time.Millisecond)
			return nil
		}, nil, nil, nil, nil)
		err := s.SendMsg(context.Background(), nil)
		test.Assert(t, err != nil)
		checkTTStreamException(err)
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
	ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil)
	s := newStream(ctx, nil, &mockStream{
		close: func() error {
			called = true
			return nil
		},
	}, scm, &kClient{}, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil, nil)

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
		s := newStream(ctx, nil, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil, nil)

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
		s := newStream(ctx, nil, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil, nil)

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
		s := newStream(ctx, nil, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil, nil)

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
		s := newStream(st.ctx, nil, st, scm, kc, ri, serviceinfo.StreamingBidirectional, nil, nil, nil, nil, nil)

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
	st := newStream(context.Background(), nil, mockSt, nil, nil, mockRPCInfo, serviceinfo.StreamingBidirectional, sendEndpoint, recvEndpoint, nil, nil, nil)
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
	st = newStream(context.Background(), nil, mockSt, nil, nil, mockRPCInfo, serviceinfo.StreamingBidirectional, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
		ri := rpcinfo.GetRPCInfo(ctx)
		test.Assert(t, ri == mockRPCInfo)
		return sendEndpoint(ctx, stream, message)
	}, func(ctx context.Context, stream streaming.ClientStream, message interface{}) (err error) {
		ri := rpcinfo.GetRPCInfo(ctx)
		test.Assert(t, ri == mockRPCInfo)
		return recvEndpoint(ctx, stream, message)
	}, nil, nil, nil)
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
