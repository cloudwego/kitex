//go:build !windows

/*
 * Copyright 2025 CloudWeGo Authors
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
	"fmt"
	"net"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/mocks"
	mock_remote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

type mockNetpollConn struct {
	mocks.Conn
	reader netpoll.Reader
	writer netpoll.Writer
}

func (m *mockNetpollConn) Reader() netpoll.Reader {
	return m.reader
}

func (m *mockNetpollConn) Writer() netpoll.Writer {
	return m.writer
}

func (m *mockNetpollConn) IsActive() bool {
	panic("implement me")
}

func (m *mockNetpollConn) SetReadTimeout(timeout time.Duration) error {
	return nil
}

func (m *mockNetpollConn) SetWriteTimeout(timeout time.Duration) error {
	return nil
}

func (m *mockNetpollConn) SetIdleTimeout(timeout time.Duration) error {
	return nil
}

func (m *mockNetpollConn) SetOnRequest(on netpoll.OnRequest) error {
	return nil
}

func (m *mockNetpollConn) AddCloseCallback(callback netpoll.CloseCallback) error {
	return nil
}

func (m *mockNetpollConn) WriteFrame(hdr, data []byte) (n int, err error) {
	return
}

func (m *mockNetpollConn) ReadFrame() (hdr, data []byte, err error) {
	return
}

func (m *mockNetpollConn) SetOnDisconnect(onDisconnect netpoll.OnDisconnect) error {
	return nil
}

type mockTracer struct {
	finishFunc func(ctx context.Context)
}

func (m *mockTracer) Start(ctx context.Context) context.Context {
	return ctx
}

func (m *mockTracer) Finish(ctx context.Context) {
	m.finishFunc(ctx)
}

type mockHeaderFrameReadHandler struct {
	ripTag string
}

func (m *mockHeaderFrameReadHandler) OnReadStream(ctx context.Context, ihd IntHeader, shd StrHeader) (context.Context, error) {
	ri := rpcinfo.GetRPCInfo(ctx)
	fi := rpcinfo.AsMutableEndpointInfo(ri.From())
	if rip, ok := shd[ttheader.HeaderTransRemoteAddr]; ok {
		fi.SetTag(m.ripTag, rip)
	}
	return ctx, nil
}

func TestOnStream(t *testing.T) {
	ripTag := "rip"
	tracer := &mockTracer{}
	traceCtl := &rpcinfo.TraceController{}
	traceCtl.Append(tracer)
	factory := NewSvrTransHandlerFactory()
	rawTransHdl, err := factory.NewTransHandler(&remote.ServerOption{
		SvcSearcher: mock_remote.NewDefaultSvcSearcher(),
		InitOrResetRPCInfoFunc: func(info rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
			return rpcinfo.NewRPCInfo(rpcinfo.NewEndpointInfo(
				mocks.MockService2Name, mocks.Mock2Method, nil, make(map[string]string)), nil,
				rpcinfo.NewInvocation(mocks.MockServiceName, mocks.MockStreamingMethod),
				rpcinfo.NewRPCConfig(),
				rpcinfo.NewRPCStats())
		},
		TracerCtl: traceCtl,
		TTHeaderStreamingOptions: remote.TTHeaderStreamingOptions{
			TransportOptions: []interface{}{
				WithServerHeaderFrameHandler(&mockHeaderFrameReadHandler{
					ripTag: ripTag,
				}),
			},
		},
	})
	test.Assert(t, err == nil, err)
	transHdl := rawTransHdl.(*svrTransHandler)

	rfd, wfd := netpoll.GetSysFdPairs()
	rconn, err := netpoll.NewFDConnection(rfd)
	test.Assert(t, err == nil, err)
	wconn, err := netpoll.NewFDConnection(wfd)
	test.Assert(t, err == nil, err)
	wbuf := newWriterBuffer(wconn.Writer())

	mockConn := &mockNetpollConn{
		Conn:   mocks.Conn{},
		reader: rconn.Reader(),
		writer: rconn.Writer(),
	}

	ctx, aerr := transHdl.OnActive(context.Background(), mockConn)
	test.Assert(t, aerr == nil, aerr)
	defer func() {
		wconn.Close()
		_, aerr = transHdl.provider.OnInactive(ctx, mockConn)
		test.Assert(t, aerr == nil, aerr)
	}()

	t.Run("invoking handler successfully", func(t *testing.T) {
		tracer.finishFunc = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			test.Assert(t, ri != nil, ri)
			rip, ok := ri.From().Tag(ripTag)
			test.Assert(t, ok)
			test.Assert(t, rip == "127.0.0.1:8888", rip)
		}
		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			return nil
		})
		err = EncodeFrame(context.Background(), wbuf, &Frame{
			streamFrame: streamFrame{
				sid:    1,
				method: mocks.MockStreamingMethod,
				header: map[string]string{
					ttheader.HeaderTransRemoteAddr: "127.0.0.1:8888",
				},
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		nctx, st, err := transHdl.provider.OnStream(ctx, mockConn)
		test.Assert(t, err == nil, err)
		err = transHdl.OnStream(nctx, mockConn, st)
		test.Assert(t, err == nil, err)
	})

	t.Run("invoking handler panic", func(t *testing.T) {
		tracer.finishFunc = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			test.Assert(t, ri != nil, ri)
			rip, ok := ri.From().Tag(ripTag)
			test.Assert(t, ok)
			test.Assert(t, rip == "127.0.0.1:8888", rip)
			ok, pErr := ri.Stats().Panicked()
			test.Assert(t, ok)
			test.Assert(t, errors.Is(pErr.(error), kerrors.ErrPanic), pErr)
		}
		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			defer func() {
				if handlerErr := recover(); handlerErr != nil {
					ri := rpcinfo.GetRPCInfo(ctx)
					err = kerrors.ErrPanic.WithCauseAndStack(fmt.Errorf("[panic] %s", handlerErr), string(debug.Stack()))
					rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats())
					rpcStats.SetPanicked(err)
				}
			}()
			panic("test")
		})
		err = EncodeFrame(context.Background(), wbuf, &Frame{
			streamFrame: streamFrame{
				sid:    1,
				method: mocks.MockStreamingMethod,
				header: map[string]string{
					ttheader.HeaderTransRemoteAddr: "127.0.0.1:8888",
				},
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		nctx, st, err := transHdl.provider.OnStream(ctx, mockConn)
		test.Assert(t, err == nil, err)
		err = transHdl.OnStream(nctx, mockConn, st)
		test.Assert(t, errors.Is(err, kerrors.ErrPanic), err)
		transHdl.OnError(ctx, err, mockConn)
	})

	t.Run("invoking handler throws biz error", func(t *testing.T) {
		tracer.finishFunc = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			test.Assert(t, ri != nil, ri)
			rip, ok := ri.From().Tag(ripTag)
			test.Assert(t, ok)
			test.Assert(t, rip == "127.0.0.1:8888", rip)
			bizErr := ri.Invocation().BizStatusErr()
			test.Assert(t, bizErr.BizStatusCode() == 10000, bizErr)
			test.Assert(t, strings.Contains(bizErr.BizMessage(), "biz-error test"), bizErr)
		}
		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			ri := rpcinfo.GetRPCInfo(ctx)
			defer func() {
				if bizErr, ok := kerrors.FromBizStatusError(err); ok {
					if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
						setter.SetBizStatusErr(bizErr)
						err = nil
					}
				}
			}()
			return kerrors.NewBizStatusError(10000, "biz-error test")
		})
		err = EncodeFrame(context.Background(), wbuf, &Frame{
			streamFrame: streamFrame{
				sid:    1,
				method: mocks.MockStreamingMethod,
				header: map[string]string{
					ttheader.HeaderTransRemoteAddr: "127.0.0.1:8888",
				},
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		nctx, st, err := transHdl.provider.OnStream(ctx, mockConn)
		test.Assert(t, err == nil, err)
		err = transHdl.OnStream(nctx, mockConn, st)
		test.Assert(t, err == nil, err)
	})
}
