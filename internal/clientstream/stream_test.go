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

package clientstream

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// mockClientStream is a controllable streaming.ClientStream for tests.
type mockClientStream struct {
	sendFn        func(ctx context.Context, m interface{}) error
	recvFn        func(ctx context.Context, m interface{}) error
	cancelWithErr func(err error)
}

func (m *mockClientStream) SendMsg(ctx context.Context, msg any) error {
	if m.sendFn != nil {
		return m.sendFn(ctx, msg)
	}
	return nil
}

func (m *mockClientStream) RecvMsg(ctx context.Context, msg any) error {
	if m.recvFn != nil {
		return m.recvFn(ctx, msg)
	}
	return nil
}

func (m *mockClientStream) Header() (streaming.Header, error)   { return nil, nil }
func (m *mockClientStream) Trailer() (streaming.Trailer, error) { return nil, nil }
func (m *mockClientStream) CloseSend(ctx context.Context) error { return nil }
func (m *mockClientStream) Context() context.Context            { return context.Background() }

// CancelWithErr makes mockClientStream implement CancelableClientStream.
func (m *mockClientStream) CancelWithErr(err error) {
	if m.cancelWithErr != nil {
		m.cancelWithErr(err)
	}
}

func newTestRPCInfo() rpcinfo.RPCInfo {
	ink := rpcinfo.NewInvocation("svc", "method")
	return rpcinfo.NewRPCInfo(nil, nil, ink, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
}

func newTestCommonStream(cs streaming.ClientStream, mode serviceinfo.StreamingMode, opts Options) (*CommonStream, rpcinfo.RPCInfo) {
	ri := newTestRPCInfo()
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	return New(ctx, cs, &rpcinfo.TraceController{}, ri, mode, opts), ri
}

func TestCommonStreamDefaultDelegates(t *testing.T) {
	var sent, recvd bool
	mcs := &mockClientStream{
		sendFn: func(ctx context.Context, m interface{}) error { sent = true; return nil },
		recvFn: func(ctx context.Context, m interface{}) error { recvd = true; return nil },
	}
	s, _ := newTestCommonStream(mcs, serviceinfo.StreamingBidirectional, Options{})

	// With no custom Send/Recv, fixCtx flags must be false.
	test.Assert(t, !s.sendFixCtx)
	test.Assert(t, !s.recvFixCtx)

	test.Assert(t, s.SendMsg(context.Background(), nil) == nil)
	test.Assert(t, s.RecvMsg(context.Background(), nil) == nil)
	test.Assert(t, sent)
	test.Assert(t, recvd)
}

func TestCommonStreamFixCtxInjectsRPCInfo(t *testing.T) {
	var sawSendRI, sawRecvRI rpcinfo.RPCInfo
	customSend := func(ctx context.Context, cs streaming.ClientStream, m interface{}) error {
		sawSendRI = rpcinfo.GetRPCInfo(ctx)
		return nil
	}
	customRecv := func(ctx context.Context, cs streaming.ClientStream, m interface{}) error {
		sawRecvRI = rpcinfo.GetRPCInfo(ctx)
		return nil
	}
	s, ri := newTestCommonStream(&mockClientStream{}, serviceinfo.StreamingBidirectional, Options{
		Send: customSend,
		Recv: customRecv,
	})
	test.Assert(t, s.sendFixCtx)
	test.Assert(t, s.recvFixCtx)

	// Caller passes a ctx WITHOUT the stream's rpcinfo; the stream must re-inject it.
	test.Assert(t, s.SendMsg(context.Background(), nil) == nil)
	test.Assert(t, sawSendRI == ri, sawSendRI)
	test.Assert(t, s.RecvMsg(context.Background(), nil) == nil)
	test.Assert(t, sawRecvRI == ri, sawRecvRI)
}

func TestCommonStreamDoFinishIdempotentAndFilters(t *testing.T) {
	t.Run("only-once", func(t *testing.T) {
		var count int
		s, _ := newTestCommonStream(&mockClientStream{}, serviceinfo.StreamingBidirectional, Options{
			OnFinish: func(err error, ri rpcinfo.RPCInfo) { count++ },
		})
		s.DoFinish(errors.New("rpc error"))
		s.DoFinish(errors.New("rpc error"))
		test.Assert(t, count == 1, count)
	})

	t.Run("eof-filtered-to-nil", func(t *testing.T) {
		var gotErr error
		called := false
		s, _ := newTestCommonStream(&mockClientStream{}, serviceinfo.StreamingBidirectional, Options{
			OnFinish: func(err error, ri rpcinfo.RPCInfo) { called = true; gotErr = err },
		})
		s.DoFinish(io.EOF)
		test.Assert(t, called)
		test.Assert(t, gotErr == nil, gotErr)
	})

	t.Run("biz-error-filtered-to-nil", func(t *testing.T) {
		var gotErr error
		s, _ := newTestCommonStream(&mockClientStream{}, serviceinfo.StreamingBidirectional, Options{
			OnFinish: func(err error, ri rpcinfo.RPCInfo) { gotErr = err },
		})
		s.DoFinish(kerrors.NewBizStatusError(100, "biz"))
		test.Assert(t, gotErr == nil, gotErr)
	})
}

func TestCommonStreamFinishHandlerPreferredOverOnFinish(t *testing.T) {
	handlerCalled := false
	onFinishCalled := false
	fh := finishHandlerFunc(func(err error, ri rpcinfo.RPCInfo) { handlerCalled = true })
	s, _ := newTestCommonStream(&mockClientStream{}, serviceinfo.StreamingBidirectional, Options{
		OnFinish:      func(err error, ri rpcinfo.RPCInfo) { onFinishCalled = true },
		FinishHandler: fh,
	})
	s.DoFinish(errors.New("rpc error"))
	test.Assert(t, handlerCalled)
	test.Assert(t, !onFinishCalled)
}

type finishHandlerFunc func(err error, ri rpcinfo.RPCInfo)

func (f finishHandlerFunc) OnFinish(err error, ri rpcinfo.RPCInfo) { f(err, ri) }

func TestCommonStreamRecvSurfacesBizStatusErr(t *testing.T) {
	s, ri := newTestCommonStream(&mockClientStream{
		recvFn: func(ctx context.Context, m interface{}) error { return nil },
	}, serviceinfo.StreamingBidirectional, Options{})
	setter := ri.Invocation().(rpcinfo.InvocationSetter)
	setter.SetBizStatusErr(kerrors.NewBizStatusError(100, "biz"))

	err := s.RecvMsg(context.Background(), nil)
	bizErr, ok := kerrors.FromBizStatusError(err)
	test.Assert(t, ok, err)
	test.Assert(t, bizErr.BizStatusCode() == 100, bizErr)
}

func TestCommonStreamClientStreamingRecvFinishes(t *testing.T) {
	finished := false
	s, _ := newTestCommonStream(&mockClientStream{}, serviceinfo.StreamingClient, Options{
		OnFinish: func(err error, ri rpcinfo.RPCInfo) { finished = true },
	})
	// A successful recv on a client-streaming stream triggers DoFinish.
	test.Assert(t, s.RecvMsg(context.Background(), nil) == nil)
	test.Assert(t, finished)
}

func TestCommonStreamSendErrorFinishes(t *testing.T) {
	sendErr := errors.New("send failed")
	finished := false
	s, _ := newTestCommonStream(&mockClientStream{
		sendFn: func(ctx context.Context, m interface{}) error { return sendErr },
	}, serviceinfo.StreamingBidirectional, Options{
		OnFinish: func(err error, ri rpcinfo.RPCInfo) { finished = true },
	})
	test.Assert(t, s.SendMsg(context.Background(), nil) == sendErr)
	test.Assert(t, finished)
}

func TestCommonStreamRecvTimeout(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		s, _ := newTestCommonStream(&mockClientStream{}, serviceinfo.StreamingBidirectional, Options{})
		test.Assert(t, !s.RecvTimeoutEnabled())
	})

	t.Run("enabled-timeout-cancels", func(t *testing.T) {
		var canceled bool
		mcs := &mockClientStream{
			recvFn: func(ctx context.Context, m interface{}) error {
				time.Sleep(200 * time.Millisecond)
				return nil
			},
			cancelWithErr: func(err error) { canceled = true },
		}
		s, _ := newTestCommonStream(mcs, serviceinfo.StreamingBidirectional, Options{
			EnableRecvTimeout: true,
			RecvTmCfg:         streaming.TimeoutConfig{Timeout: 30 * time.Millisecond},
		})
		test.Assert(t, s.RecvTimeoutEnabled())
		err := s.RecvMsg(context.Background(), nil)
		test.Assert(t, err != nil)
		test.Assert(t, canceled)
	})
}

func TestIsRPCError(t *testing.T) {
	test.Assert(t, !IsRPCError(nil))
	test.Assert(t, !IsRPCError(io.EOF))
	test.Assert(t, !IsRPCError(kerrors.NewBizStatusError(100, "biz")))
	test.Assert(t, IsRPCError(errors.New("real error")))
}

func TestCommonStreamGetSetGRPCStream(t *testing.T) {
	s, _ := newTestCommonStream(&mockClientStream{}, serviceinfo.StreamingBidirectional, Options{})
	test.Assert(t, s.GetGRPCStream() == nil)

	fwd := NewGRPCForwardStream(nil, s)
	s.SetGRPCStream(fwd)
	test.Assert(t, s.GetGRPCStream() == streaming.Stream(fwd))
}

func TestGRPCForwardStreamForwards(t *testing.T) {
	var sent, recvd bool
	mcs := &mockClientStream{
		sendFn: func(ctx context.Context, m interface{}) error { sent = true; return nil },
		recvFn: func(ctx context.Context, m interface{}) error { recvd = true; return nil },
	}
	s, _ := newTestCommonStream(mcs, serviceinfo.StreamingBidirectional, Options{})
	fwd := NewGRPCForwardStream(nil, s)

	test.Assert(t, fwd.SendMsg(nil) == nil)
	test.Assert(t, sent)
	test.Assert(t, fwd.RecvMsg(nil) == nil)
	test.Assert(t, recvd)
}

func TestGRPCForwardStreamDoFinish(t *testing.T) {
	finished := false
	s, _ := newTestCommonStream(&mockClientStream{}, serviceinfo.StreamingBidirectional, Options{
		OnFinish: func(err error, ri rpcinfo.RPCInfo) { finished = true },
	})
	fwd := NewGRPCForwardStream(nil, s)
	fwd.DoFinish(errors.New("rpc error"))
	test.Assert(t, finished)
}
