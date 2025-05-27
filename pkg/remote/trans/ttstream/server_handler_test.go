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
	"testing"
	"time"

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

func TestOnStream(t *testing.T) {
	factory := NewSvrTransHandlerFactory()
	rawTransHdl, err := factory.NewTransHandler(&remote.ServerOption{
		SvcSearcher: mock_remote.NewDefaultSvcSearcher(),
		InitOrResetRPCInfoFunc: func(info rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
			return rpcinfo.NewRPCInfo(nil, nil,
				rpcinfo.NewInvocation(mocks.MockServiceName, mocks.MockStreamingMethod),
				rpcinfo.NewRPCConfig(),
				rpcinfo.NewRPCStats())
		},
		TracerCtl: &rpcinfo.TraceController{},
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
		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			return nil
		})
		err = EncodeFrame(context.Background(), wbuf, &Frame{
			streamFrame: streamFrame{
				sid:    1,
				method: mocks.MockStreamingMethod,
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		nctx, ss, err := transHdl.provider.OnStream(ctx, mockConn)
		test.Assert(t, err == nil, err)
		err = transHdl.OnStream(nctx, mockConn, ss)
		test.Assert(t, err == nil, err)
	})

	t.Run("invoking handler panic", func(t *testing.T) {
		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			defer func() {
				if handlerErr := recover(); handlerErr != nil {
					err = kerrors.ErrPanic.WithCauseAndStack(fmt.Errorf("[panic] %s", handlerErr), string(debug.Stack()))
				}
			}()
			panic("test")
		})
		err = EncodeFrame(context.Background(), wbuf, &Frame{
			streamFrame: streamFrame{
				sid:    1,
				method: mocks.MockStreamingMethod,
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		nctx, ss, err := transHdl.provider.OnStream(ctx, mockConn)
		test.Assert(t, err == nil, err)
		err = transHdl.OnStream(nctx, mockConn, ss)
		test.Assert(t, errors.Is(err, kerrors.ErrPanic), err)
		transHdl.OnError(ctx, err, mockConn)
	})

	t.Run("invoking handler throws biz error", func(t *testing.T) {
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
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		nctx, ss, err := transHdl.provider.OnStream(ctx, mockConn)
		test.Assert(t, err == nil, err)
		err = transHdl.OnStream(nctx, mockConn, ss)
		test.Assert(t, err == nil, err)
	})
}
