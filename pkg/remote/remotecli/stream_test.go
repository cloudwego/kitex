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

package remotecli

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/internal/mocks"
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	mock_remote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	kitex "github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/transport"
)

func TestNewStream(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	addr := utils.NewNetAddr("tcp", "to")
	ri := newMockRPCInfo(addr)
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

	hdlr := &mocks.MockCliTransHandler{}
	hdlr.NewStreamFunc = func(ctx context.Context, svcInfo *kitex.ServiceInfo, conn net.Conn, handler remote.TransReadWriter) (streaming.Stream, error) {
		return nphttp2.NewStream(ctx, svcInfo, conn, handler), nil
	}

	opt := newMockOption(ctrl, addr)
	opt.Option = remote.Option{
		StreamingMetaHandlers: []remote.StreamingMetaHandler{
			transmeta.ClientHTTP2Handler,
		},
	}

	_, _, err := NewStream(ctx, ri, hdlr, opt)
	test.Assert(t, err == nil, err)
}

func TestStreamConnManagerReleaseConn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cr := mock_remote.NewMockConnReleaser(ctrl)
	cr.EXPECT().ReleaseConn(gomock.Any(), gomock.Any()).Times(1)

	scr := NewStreamConnManager(cr)
	test.Assert(t, scr != nil)
	test.Assert(t, scr.ConnReleaser == cr)
	ri := rpcinfo.NewRPCInfo(nil, nil, nil, rpcinfo.NewRPCConfig(), nil)
	scr.ReleaseConn(nil, nil, ri)
}

type mockConnReleaser struct {
	releaseConn func(err error, ri rpcinfo.RPCInfo)
}

func (m *mockConnReleaser) ReleaseConn(err error, ri rpcinfo.RPCInfo) {
	if m.releaseConn != nil {
		m.releaseConn(err, ri)
	}
}

func TestStreamConnManager_AsyncCleanAndRelease(t *testing.T) {
	t.Run("finish-without-error", func(t *testing.T) {
		cleaner := &StreamCleaner{
			Async:   true,
			Timeout: time.Second,
			Clean: func(st streaming.Stream) error {
				return nil
			},
		}
		result := make(chan error, 1)
		scm := NewStreamConnManager(&mockConnReleaser{
			releaseConn: func(err error, ri rpcinfo.RPCInfo) {
				result <- err
			},
		})
		scm.AsyncCleanAndRelease(cleaner, nil, nil)
		got := <-result
		test.Assert(t, got == nil)
	})
	t.Run("finish-with-error", func(t *testing.T) {
		cleaner := &StreamCleaner{
			Async:   true,
			Timeout: time.Second,
			Clean: func(st streaming.Stream) error {
				return context.DeadlineExceeded
			},
		}
		result := make(chan error, 1)
		scm := NewStreamConnManager(&mockConnReleaser{
			releaseConn: func(err error, ri rpcinfo.RPCInfo) {
				result <- err
			},
		})
		scm.AsyncCleanAndRelease(cleaner, nil, nil)
		got := <-result
		test.Assert(t, got != nil)
	})
	t.Run("cleaner-timeout", func(t *testing.T) {
		cleaner := &StreamCleaner{
			Async:   true,
			Timeout: time.Millisecond * 10,
			Clean: func(st streaming.Stream) error {
				time.Sleep(time.Millisecond * 20)
				return nil
			},
		}
		result := make(chan error, 1)
		scm := NewStreamConnManager(&mockConnReleaser{
			releaseConn: func(err error, ri rpcinfo.RPCInfo) {
				result <- err
			},
		})
		scm.AsyncCleanAndRelease(cleaner, nil, nil)
		got := <-result
		test.Assert(t, got != nil)
	})
	t.Run("cleaner-panic", func(t *testing.T) {
		cleaner := &StreamCleaner{
			Async:   true,
			Timeout: time.Second,
			Clean: func(st streaming.Stream) error {
				panic("cleaner panic")
			},
		}
		result := make(chan error, 1)
		scm := NewStreamConnManager(&mockConnReleaser{
			releaseConn: func(err error, ri rpcinfo.RPCInfo) {
				result <- err
			},
		})
		scm.AsyncCleanAndRelease(cleaner, nil, nil)
		got := <-result
		test.Assert(t, got != nil)
	})
}

func TestStreamConnManager_ReleaseConn(t *testing.T) {
	t.Run("non-nil-error", func(t *testing.T) {
		err := errors.New("error")
		result := make(chan error, 1)
		scm := NewStreamConnManager(&mockConnReleaser{
			releaseConn: func(err error, ri rpcinfo.RPCInfo) {
				result <- err
			},
		})
		scm.ReleaseConn(nil, err, nil)
		got := <-result
		test.Assert(t, errors.Is(got, err))
	})
	t.Run("nil-error-without-registered-cleaner", func(t *testing.T) {
		result := make(chan error, 1)
		scm := NewStreamConnManager(&mockConnReleaser{
			releaseConn: func(err error, ri rpcinfo.RPCInfo) {
				result <- err
			},
		})
		cfg := rpcinfo.NewRPCConfig()
		_ = rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.Framed)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, cfg, nil)
		scm.ReleaseConn(nil, nil, ri)
		got := <-result
		test.Assert(t, got == nil)
	})
	t.Run("nil-error-with-registered-sync-cleaner", func(t *testing.T) {
		err := errors.New("error")
		result := make(chan error, 1)
		scm := NewStreamConnManager(&mockConnReleaser{
			releaseConn: func(err error, ri rpcinfo.RPCInfo) {
				result <- err
			},
		})
		cfg := rpcinfo.NewRPCConfig()
		_ = rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.Framed)
		RegisterCleaner(transport.Framed, &StreamCleaner{
			Async: false,
			Clean: func(st streaming.Stream) error {
				return err
			},
		})
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, cfg, nil)
		scm.ReleaseConn(nil, nil, ri)
		got := <-result
		test.Assert(t, errors.Is(got, err))
	})
	t.Run("nil-error-with-registered-async-cleaner", func(t *testing.T) {
		err := errors.New("error")
		result := make(chan error, 1)
		scm := NewStreamConnManager(&mockConnReleaser{
			releaseConn: func(err error, ri rpcinfo.RPCInfo) {
				result <- err
			},
		})
		cfg := rpcinfo.NewRPCConfig()
		_ = rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.Framed)
		RegisterCleaner(transport.Framed, &StreamCleaner{
			Async:   true,
			Timeout: time.Second,
			Clean: func(st streaming.Stream) error {
				return err
			},
		})
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, cfg, nil)
		scm.ReleaseConn(nil, nil, ri)
		got := <-result
		test.Assert(t, errors.Is(got, err))
	})
}
