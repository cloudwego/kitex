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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/mocks"
	mock_remote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/consts"
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
	if m.finishFunc != nil {
		m.finishFunc(ctx)
	}
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

func TestOnRead(t *testing.T) {
	prepare := func() (ctx context.Context, ripTag string, tracer *mockTracer, transHdl *svrTransHandler, wconn netpoll.Connection, wb *writerBuffer, mockConn *mockNetpollConn) {
		ripTag = "rip"
		tracer = &mockTracer{}
		traceCtl := &rpcinfo.TraceController{}
		traceCtl.Append(tracer)
		factory := NewSvrTransHandlerFactory()
		rawTransHdl, err := factory.NewTransHandler(&remote.ServerOption{
			SvcSearcher: mock_remote.NewDefaultSvcSearcher(),
			InitOrResetRPCInfoFunc: func(info rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
				return rpcinfo.NewRPCInfo(
					rpcinfo.EmptyEndpointInfo(),
					rpcinfo.EmptyEndpointInfo(),
					rpcinfo.NewServerInvocation(),
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
		transHdl = rawTransHdl.(*svrTransHandler)

		rfd, wfd := netpoll.GetSysFdPairs()
		rconn, err := netpoll.NewFDConnection(rfd)
		test.Assert(t, err == nil, err)
		wconn, err = netpoll.NewFDConnection(wfd)
		test.Assert(t, err == nil, err)
		wb = newWriterBuffer(wconn.Writer())

		mockConn = &mockNetpollConn{
			Conn:   mocks.Conn{},
			reader: rconn.Reader(),
			writer: rconn.Writer(),
		}

		ctx, err = transHdl.OnActive(context.Background(), mockConn)
		test.Assert(t, err == nil, err)
		return
	}

	t.Run("invoking handler successfully", func(t *testing.T) {
		ctx, ripTag, tracer, transHdl, wconn, wbuf, mockConn := prepare()
		tracer.finishFunc = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			test.Assert(t, ri != nil, ri)
			rip, ok := ri.From().Tag(ripTag)
			test.Assert(t, ok)
			test.Assert(t, rip == "127.0.0.1:8888", rip)
		}
		var invoked int32
		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			atomic.StoreInt32(&invoked, 1)
			return nil
		})
		err := EncodeFrame(context.Background(), wbuf, &Frame{
			streamFrame: streamFrame{
				sid:    1,
				method: mocks.MockStreamingMethod,
				header: map[string]string{
					ttheader.HeaderIDLServiceName:  mocks.MockServiceName,
					ttheader.HeaderTransRemoteAddr: "127.0.0.1:8888",
				},
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		go func() {
			time.Sleep(1 * time.Second)
			wconn.Close()
		}()
		err = transHdl.OnRead(ctx, mockConn)
		test.Assert(t, err == nil, err)
		test.Assert(t, atomic.LoadInt32(&invoked) == 1)
	})

	t.Run("invoking handler panic", func(t *testing.T) {
		ctx, ripTag, tracer, transHdl, wconn, wbuf, mockConn := prepare()
		tracer.finishFunc = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			test.Assert(t, ri != nil, ri)
			rip, ok := ri.From().Tag(ripTag)
			test.Assert(t, ok)
			test.Assert(t, rip == "127.0.0.1:8888", rip)
			ok, pErr := ri.Stats().Panicked()
			test.Assert(t, ok)
			test.Assert(t, errors.Is(pErr.(error), kerrors.ErrPanic), pErr)
			test.Assert(t, errors.Is(ri.Stats().Error(), kerrors.ErrPanic))
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
		err := EncodeFrame(context.Background(), wbuf, &Frame{
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
		go func() {
			time.Sleep(1 * time.Second)
			wconn.Close()
		}()
		err = transHdl.OnRead(ctx, mockConn)
		test.Assert(t, err == nil, err)
	})

	t.Run("invoking handler throws biz error", func(t *testing.T) {
		ctx, ripTag, tracer, transHdl, wconn, wbuf, mockConn := prepare()
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
		err := EncodeFrame(context.Background(), wbuf, &Frame{
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
		go func() {
			time.Sleep(1 * time.Second)
			wconn.Close()
		}()
		err = transHdl.OnRead(ctx, mockConn)
		test.Assert(t, err == nil, err)
	})

	t.Run("invoking handler and getting K_METHOD successfully", func(t *testing.T) {
		ctx, ripTag, tracer, transHdl, wconn, wbuf, mockConn := prepare()
		tracer.finishFunc = func(ctx context.Context) {
			ri := rpcinfo.GetRPCInfo(ctx)
			test.Assert(t, ri != nil, ri)
			rip, ok := ri.From().Tag(ripTag)
			test.Assert(t, ok)
			test.Assert(t, rip == "127.0.0.1:8888", rip)
			// retrieve TO method from rpcinfo
			mt := ri.To().Method()
			test.Assert(t, mt == mocks.MockStreamingMethod, mt)
			// retrieve K_METHOD from ctx
			kMt := ctx.Value(consts.CtxKeyMethod).(string)
			test.Assert(t, kMt == mocks.MockStreamingMethod, kMt)
		}
		var invoked int32
		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			atomic.StoreInt32(&invoked, 1)
			// retrieve K_METHOD from ctx
			kMt := ctx.Value(consts.CtxKeyMethod).(string)
			test.Assert(t, kMt == mocks.MockStreamingMethod, kMt)
			return nil
		})
		err := EncodeFrame(context.Background(), wbuf, &Frame{
			streamFrame: streamFrame{
				sid:    1,
				method: mocks.MockStreamingMethod,
				header: map[string]string{
					ttheader.HeaderIDLServiceName:  mocks.MockServiceName,
					ttheader.HeaderTransRemoteAddr: "127.0.0.1:8888",
				},
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		go func() {
			time.Sleep(1 * time.Second)
			wconn.Close()
		}()
		err = transHdl.OnRead(ctx, mockConn)
		test.Assert(t, err == nil, err)
		test.Assert(t, atomic.LoadInt32(&invoked) == 1)
	})

	t.Run("rpcinfo reuse disabled", func(t *testing.T) {
		var ri rpcinfo.RPCInfo
		ctx, _, _, transHdl, wconn, wbuf, mockConn := prepare()

		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			ri = rpcinfo.GetRPCInfo(ctx)
			return nil
		})
		err := EncodeFrame(context.Background(), wbuf, &Frame{
			streamFrame: streamFrame{
				sid:    1,
				method: mocks.MockStreamingMethod,
				header: map[string]string{
					ttheader.HeaderIDLServiceName:  mocks.MockServiceName,
					ttheader.HeaderTransRemoteAddr: "127.0.0.1:8888",
				},
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		go func() {
			time.Sleep(1 * time.Second)
			wconn.Close()
		}()
		err = transHdl.OnRead(ctx, mockConn)
		test.Assert(t, err == nil, err)
		test.Assert(t, ri.Invocation().MethodName() == mocks.MockStreamingMethod, ri)
	})

	t.Run("access rpcinfo asynchronously would not cause panic", func(t *testing.T) {
		ctx, _, _, transHdl, wconn, wbuf, mockConn := prepare()
		var wg sync.WaitGroup

		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func(c context.Context) {
					defer func() {
						if r := recover(); r != nil {
							t.Error(r)
						}
						wg.Done()
					}()
					time.Sleep(50 * time.Millisecond)
					ri := rpcinfo.GetRPCInfo(c)
					// access rpcinfo
					ri.From().Tag("key")
				}(ctx)
			}
			return nil
		})
		err := EncodeFrame(context.Background(), wbuf, &Frame{
			streamFrame: streamFrame{
				sid:    1,
				method: mocks.MockStreamingMethod,
				header: map[string]string{
					ttheader.HeaderIDLServiceName:  mocks.MockServiceName,
					ttheader.HeaderTransRemoteAddr: "127.0.0.1:8888",
				},
			},
			typ: headerFrameType,
		})
		test.Assert(t, err == nil, err)
		err = wbuf.Flush()
		test.Assert(t, err == nil, err)
		go func() {
			time.Sleep(1 * time.Second)
			wconn.Close()
		}()
		err = transHdl.OnRead(ctx, mockConn)
		test.Assert(t, err == nil, err)
		wg.Wait()
	})
}

func TestOnStreamFinish(t *testing.T) {
	hdl := &svrTransHandler{}

	t.Run("no error", func(t *testing.T) {
		var tl *Frame
		ss := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				if f.typ == trailerFrameType {
					tl = f
				}
				return nil
			},
		})
		err := hdl.OnStreamFinish(ss, nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, tl != nil)
		test.Assert(t, len(tl.payload) == 0, tl)
	})
	t.Run("biz status error without extra", func(t *testing.T) {
		var tl *Frame
		ss := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				if f.typ == trailerFrameType {
					tl = f
				}
				return nil
			},
		})
		err := hdl.OnStreamFinish(ss, kerrors.NewBizStatusError(10001, "biz error"))
		test.Assert(t, err == nil, err)
		test.Assert(t, tl != nil)
		test.Assert(t, tl.trailer["biz-status"] == "10001", tl.trailer)
		test.Assert(t, tl.trailer["biz-message"] == "biz error", tl.trailer)
		test.Assert(t, len(tl.payload) == 0, tl)
	})
	t.Run("biz status error with extra", func(t *testing.T) {
		var tl *Frame
		ss := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				if f.typ == trailerFrameType {
					tl = f
				}
				return nil
			},
		})
		err := hdl.OnStreamFinish(ss, kerrors.NewBizStatusErrorWithExtra(10002, "biz extra", map[string]string{"k": "v"}))
		test.Assert(t, err == nil, err)
		test.Assert(t, tl != nil)
		test.Assert(t, tl.trailer["biz-status"] == "10002", tl.trailer)
		test.Assert(t, tl.trailer["biz-message"] == "biz extra", tl.trailer)
		test.Assert(t, tl.trailer["biz-extra"] != "", tl.trailer)
		test.Assert(t, len(tl.payload) == 0, tl)
	})
	t.Run("thrift application exception", func(t *testing.T) {
		var tl *Frame
		ss := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				if f.typ == trailerFrameType {
					tl = f
				}
				return nil
			},
		})
		ss.method = "testMethod"
		err := hdl.OnStreamFinish(ss, thrift.NewApplicationException(thrift.UNKNOWN_METHOD, "unknown method"))
		test.Assert(t, err == nil, err)
		test.Assert(t, tl != nil)
		test.Assert(t, len(tl.payload) > 0, tl)
	})
	t.Run("ttstream exception", func(t *testing.T) {
		var tl *Frame
		ss := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				if f.typ == trailerFrameType {
					tl = f
				}
				return nil
			},
		})
		ss.method = "testMethod"
		err := hdl.OnStreamFinish(ss, errInternalCancel.newBuilder().withCause(errors.New("cancel")))
		test.Assert(t, err == nil, err)
		test.Assert(t, tl != nil)
		test.Assert(t, len(tl.payload) > 0, tl)
	})
	t.Run("generic error", func(t *testing.T) {
		var tl *Frame
		ss := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				if f.typ == trailerFrameType {
					tl = f
				}
				return nil
			},
		})
		ss.method = "testMethod"
		err := hdl.OnStreamFinish(ss, errors.New("some error"))
		test.Assert(t, err == nil, err)
		test.Assert(t, tl != nil)
		test.Assert(t, len(tl.payload) > 0, tl)
	})
	t.Run("CloseSend write error", func(t *testing.T) {
		writeErr := errors.New("write failed")
		ss := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				return writeErr
			},
		})
		err := hdl.OnStreamFinish(ss, nil)
		test.Assert(t, err != nil)
	})
	t.Run("finishTracer receives valid ctx when OnStreamFinish fails", func(t *testing.T) {
		tracer := &mockTracer{}
		traceCtl := &rpcinfo.TraceController{}
		traceCtl.Append(tracer)

		transHdl := &svrTransHandler{
			opt: &remote.ServerOption{
				SvcSearcher: mock_remote.NewDefaultSvcSearcher(),
				InitOrResetRPCInfoFunc: func(_ rpcinfo.RPCInfo, _ net.Addr) rpcinfo.RPCInfo {
					return rpcinfo.NewRPCInfo(
						rpcinfo.EmptyEndpointInfo(),
						rpcinfo.EmptyEndpointInfo(),
						rpcinfo.NewServerInvocation(),
						rpcinfo.NewRPCConfig(),
						rpcinfo.NewRPCStats())
				},
				TracerCtl: traceCtl,
			},
		}
		transHdl.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			ri.Invocation().(rpcinfo.InvocationSetter).SetBizStatusErr(kerrors.NewBizStatusError(10000, "test"))
			return nil
		})

		var tracerCtx context.Context
		tracer.finishFunc = func(ctx context.Context) {
			tracerCtx = ctx
		}

		ss := newTestServerStreamWithStreamWriter(mockStreamWriter{
			writeFrameFunc: func(f *Frame) error {
				return errors.New("write failed")
			},
		})
		ss.method = mocks.MockStreamingMethod
		ss.header = map[string]string{
			ttheader.HeaderIDLServiceName: mocks.MockServiceName,
		}

		err := transHdl.OnStream(context.Background(), &mocks.Conn{}, ss)
		test.Assert(t, err != nil)
		test.Assert(t, tracerCtx != nil)
		ri := rpcinfo.GetRPCInfo(tracerCtx)
		test.Assert(t, ri != nil, tracerCtx)
	})
}
