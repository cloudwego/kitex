//go:build !windows

/*
 * Copyright 2026 CloudWeGo Authors
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

package ttstream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/utils"
)

// mockTransPool implements transPool for testing clientTransHandler.NewStream
type mockTransPool struct {
	getTrans *clientTransport
	getErr   error
}

func (p *mockTransPool) Get(network, addr string) (*clientTransport, error) {
	return p.getTrans, p.getErr
}

func (p *mockTransPool) Put(trans *clientTransport) {}

// mockHeaderFrameWriteHandler implements HeaderFrameWriteHandler for testing
type mockHeaderFrameWriteHandler struct {
	intHeader IntHeader
	strHeader streaming.Header
	err       error
}

func (h *mockHeaderFrameWriteHandler) OnWriteStream(ctx context.Context) (IntHeader, StrHeader, error) {
	return h.intHeader, h.strHeader, h.err
}

func newTestRPCInfoCtx(cfg rpcinfo.RPCConfig) (context.Context, rpcinfo.RPCInfo) {
	if cfg == nil {
		cfg = rpcinfo.NewRPCConfig()
	}
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.NewEndpointInfo("clientSvc", "testMethod", nil, nil),
		rpcinfo.NewEndpointInfo("serverSvc", "testMethod", utils.NewNetAddr("tcp", "127.0.0.1:8888"), nil),
		rpcinfo.NewInvocation("serverSvc", "testMethod"),
		cfg,
		nil,
	)
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	return ctx, ri
}

func TestNewStream(t *testing.T) {
	t.Run("no dest address", func(t *testing.T) {
		handler := &clientTransHandler{
			transPool: &mockTransPool{},
		}
		ri := rpcinfo.NewRPCInfo(
			nil,
			rpcinfo.NewEndpointInfo("serverSvc", "testMethod", nil, nil), // addr = nil
			rpcinfo.NewInvocation("serverSvc", "testMethod"),
			rpcinfo.NewRPCConfig(), nil,
		)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		_, err := handler.NewStream(ctx, ri)
		test.Assert(t, errors.Is(err, kerrors.ErrNoDestAddress), err)
	})
	t.Run("headerHandler.OnWriteStream() fails", func(t *testing.T) {
		hdlErr := errors.New("header handler error")
		handler := &clientTransHandler{
			transPool:     &mockTransPool{},
			headerHandler: &mockHeaderFrameWriteHandler{err: hdlErr},
		}
		ctx, ri := newTestRPCInfoCtx(nil)
		_, err := handler.NewStream(ctx, ri)
		test.Assert(t, err == hdlErr, err)
	})
	t.Run("transPool.Get() fails", func(t *testing.T) {
		getErr := errors.New("pool get error")
		handler := &clientTransHandler{
			transPool: &mockTransPool{getErr: getErr},
		}
		ctx, ri := newTestRPCInfoCtx(nil)
		_, err := handler.NewStream(ctx, ri)
		test.Assert(t, err == getErr, err)
	})
	t.Run("success with StreamRecvTimeout", func(t *testing.T) {
		cconn, sconn := newTestConnectionPipe(t)
		defer cconn.Close()
		defer sconn.Close()

		ctrans := newClientTransport(cconn, nil)
		handler := &clientTransHandler{
			transPool: &mockTransPool{getTrans: ctrans},
		}
		cfg := rpcinfo.NewRPCConfig()
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeout(100 * time.Millisecond)
		ctx, ri := newTestRPCInfoCtx(cfg)

		cs, err := handler.NewStream(ctx, ri)
		test.Assert(t, err == nil, err)
		test.Assert(t, cs != nil)

		cliSt := cs.(*clientStream)
		test.Assert(t, cliSt.recvTimeoutConfig.Timeout == 100*time.Millisecond, cliSt.recvTimeoutConfig)
		test.Assert(t, !cliSt.recvTimeoutConfig.DisableCancelRemote, cliSt.recvTimeoutConfig)
		test.Assert(t, cliSt.stream.recvTimeoutCallback != nil)
	})
	t.Run("success with StreamRecvTimeoutConfig", func(t *testing.T) {
		cconn, sconn := newTestConnectionPipe(t)
		defer cconn.Close()
		defer sconn.Close()

		ctrans := newClientTransport(cconn, nil)
		handler := &clientTransHandler{
			transPool: &mockTransPool{getTrans: ctrans},
		}
		cfg := rpcinfo.NewRPCConfig()
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeoutConfig(streaming.TimeoutConfig{
			Timeout:             100 * time.Millisecond,
			DisableCancelRemote: true,
		})
		ctx, ri := newTestRPCInfoCtx(cfg)

		cs, err := handler.NewStream(ctx, ri)
		test.Assert(t, err == nil, err)
		test.Assert(t, cs != nil)

		cliSt := cs.(*clientStream)
		test.Assert(t, cliSt.recvTimeoutConfig.Timeout == 100*time.Millisecond, cliSt.recvTimeoutConfig)
		test.Assert(t, cliSt.recvTimeoutConfig.DisableCancelRemote, cliSt.recvTimeoutConfig)
		test.Assert(t, cliSt.stream.recvTimeoutCallback != nil)
	})
	t.Run("both set, StreamRecvTimeoutConfig takes priority", func(t *testing.T) {
		cconn, sconn := newTestConnectionPipe(t)
		defer cconn.Close()
		defer sconn.Close()

		ctrans := newClientTransport(cconn, nil)
		handler := &clientTransHandler{
			transPool: &mockTransPool{getTrans: ctrans},
		}
		cfg := rpcinfo.NewRPCConfig()
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeout(500 * time.Millisecond)
		rpcinfo.AsMutableRPCConfig(cfg).SetStreamRecvTimeoutConfig(streaming.TimeoutConfig{
			Timeout:             100 * time.Millisecond,
			DisableCancelRemote: true,
		})
		ctx, ri := newTestRPCInfoCtx(cfg)

		cs, err := handler.NewStream(ctx, ri)
		test.Assert(t, err == nil, err)
		test.Assert(t, cs != nil)

		cliSt := cs.(*clientStream)
		test.Assert(t, cliSt.recvTimeoutConfig.Timeout == 100*time.Millisecond, cliSt.recvTimeoutConfig)
		test.Assert(t, cliSt.recvTimeoutConfig.DisableCancelRemote, cliSt.recvTimeoutConfig)
		test.Assert(t, cliSt.stream.recvTimeoutCallback != nil)
	})
	t.Run("stream ctx deadline sets streamTimeout", func(t *testing.T) {
		cconn, sconn := newTestConnectionPipe(t)
		defer cconn.Close()
		defer sconn.Close()

		ctrans := newClientTransport(cconn, nil)
		handler := &clientTransHandler{
			transPool: &mockTransPool{getTrans: ctrans},
		}
		ctx, ri := newTestRPCInfoCtx(nil)
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()

		cs, err := handler.NewStream(ctx, ri)
		test.Assert(t, err == nil, err)

		cliSt := cs.(*clientStream)
		test.Assert(t, cliSt.streamTimeout > 0 && cliSt.streamTimeout <= 500*time.Millisecond, cliSt.streamTimeout)
	})
	t.Run("headerHandler provides custom headers", func(t *testing.T) {
		cconn, sconn := newTestConnectionPipe(t)
		defer cconn.Close()
		defer sconn.Close()

		ctrans := newClientTransport(cconn, nil)
		handler := &clientTransHandler{
			transPool: &mockTransPool{getTrans: ctrans},
			headerHandler: &mockHeaderFrameWriteHandler{
				intHeader: IntHeader{1: "val1"},
				strHeader: streaming.Header{"custom-key": "custom-val"},
			},
		}
		ctx, ri := newTestRPCInfoCtx(nil)

		cs, err := handler.NewStream(ctx, ri)
		test.Assert(t, err == nil, err)
		test.Assert(t, cs != nil)
	})
}
