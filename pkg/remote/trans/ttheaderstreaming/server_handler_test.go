/*
 * Copyright 2024 CloudWeGo Authors
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

package ttheaderstreaming

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec"
	"github.com/cloudwego/kitex/pkg/remote/codec/ttheader"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/transport"
)

func Test_ttheaderStreamingTransHandler_ProtocolMatch(t *testing.T) {
	t.Run("peek-err", func(t *testing.T) {
		opt := &remote.ServerOption{}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		err := svrHandler.ProtocolMatch(context.Background(), &mockConn{
			readBuf: make([]byte, 0),
		})
		test.Assert(t, err != nil, err)
	})
	t.Run("match", func(t *testing.T) {
		opt := &remote.ServerOption{}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		buf := []byte{0, 0, 0, 0, 0x10, 0x00, 0x00, 0x02}
		err := svrHandler.ProtocolMatch(context.Background(), &mockConn{
			readBuf: buf,
		})
		test.Assert(t, err == nil, err)
	})
	t.Run("unmatched", func(t *testing.T) {
		opt := &remote.ServerOption{}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		buf := []byte{0, 0, 0, 0, 0x10, 0x00, 0x00, 0x00}
		err := svrHandler.ProtocolMatch(context.Background(), &mockConn{
			readBuf: buf,
		})
		test.Assert(t, errors.Is(err, ErrNotTTHeaderStreaming), err)
	})
}

func Test_getServerMetaHandler(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		h := getServerFrameMetaHandler(nil)
		_, ok := h.(*serverTTHeaderFrameHandler)
		test.Assert(t, ok, h)
	})
	t.Run("non-nil", func(t *testing.T) {
		h := getServerFrameMetaHandler(&mockFrameMetaHandler{})
		_, ok := h.(*mockFrameMetaHandler)
		test.Assert(t, ok, h)
	})
}

type mockTracer struct {
	start  func(ctx context.Context) context.Context
	finish func(ctx context.Context)
}

func (m *mockTracer) Start(ctx context.Context) context.Context {
	if m.start != nil {
		return m.start(ctx)
	}
	return ctx
}

func (m *mockTracer) Finish(ctx context.Context) {
	if m.finish != nil {
		m.finish(ctx)
	}
}

func Test_ttheaderStreamingTransHandler_finishTracer(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		called := false
		ctrl := &rpcinfo.TraceController{}
		ctrl.Append(&mockTracer{
			finish: func(ctx context.Context) {
				ri := rpcinfo.GetRPCInfo(ctx)
				event := ri.Stats().GetEvent(stats.RPCFinish)
				test.Assert(t, event != nil && event.Event() == stats.RPCFinish, event)
				called = true
			},
		})
		h, _ := NewSvrTransHandler(&remote.ServerOption{
			TracerCtl: ctrl,
		}, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelBase)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		svrHandler.finishTracer(ctx, ri, nil, nil)
		test.Assert(t, called, called)
	})
	t.Run("err", func(t *testing.T) {
		err := errors.New("err")
		called := false
		ctrl := &rpcinfo.TraceController{}
		ctrl.Append(&mockTracer{
			finish: func(ctx context.Context) {
				ri := rpcinfo.GetRPCInfo(ctx)
				event := ri.Stats().GetEvent(stats.RPCFinish)
				test.Assert(t, event != nil && event.Event() == stats.RPCFinish, event)
				test.Assert(t, event.Info() == err.Error(), event)
				called = true
			},
		})
		h, _ := NewSvrTransHandler(&remote.ServerOption{
			TracerCtl: ctrl,
		}, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelBase)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		svrHandler.finishTracer(ctx, ri, err, nil)
		test.Assert(t, called, called)
	})
	t.Run("panic", func(t *testing.T) {
		called := false
		ctrl := &rpcinfo.TraceController{}
		ctrl.Append(&mockTracer{
			finish: func(ctx context.Context) {
				ri := rpcinfo.GetRPCInfo(ctx)
				event := ri.Stats().GetEvent(stats.RPCFinish)
				test.Assert(t, event != nil && event.Event() == stats.RPCFinish, event)
				panicked, info := ri.Stats().Panicked()
				test.Assert(t, panicked && info == "panic", panicked, info)
				called = true
			},
		})
		h, _ := NewSvrTransHandler(&remote.ServerOption{
			TracerCtl: ctrl,
		}, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelBase)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		svrHandler.finishTracer(ctx, ri, nil, "panic")
		test.Assert(t, called, called)
	})

	t.Run("no-rpcStats", func(t *testing.T) {
		called := false
		ctrl := &rpcinfo.TraceController{}
		ctrl.Append(&mockTracer{
			finish: func(ctx context.Context) {
				called = true
			},
		})
		h, _ := NewSvrTransHandler(&remote.ServerOption{
			TracerCtl: ctrl,
		}, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, nil)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
		svrHandler.finishTracer(ctx, ri, nil, nil)
		test.Assert(t, !called, called)
	})
}

func Test_ttheaderStreamingTransHandler_retrieveServiceMethodInfo(t *testing.T) {
	h, _ := NewSvrTransHandler(&remote.ServerOption{
		SvcSearchMap: map[string]*serviceinfo.ServiceInfo{
			remote.BuildMultiServiceKey("service", "method"): {
				ServiceName: "service",
				Methods: map[string]serviceinfo.MethodInfo{
					"method": serviceinfo.NewMethodInfo(nil, nil, nil, false),
				},
			},
		},
	}, nil)
	svrHandler := h.(*ttheaderStreamingTransHandler)
	ivk := rpcinfo.NewInvocation("service", "method")
	ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, nil)
	svcInfo, methodInfo := svrHandler.retrieveServiceMethodInfo(ri)
	test.Assert(t, svcInfo != nil, svcInfo)
	test.Assert(t, methodInfo != nil, methodInfo)
}

func Test_ttheaderStreamingTransHandler_unknownMethodWithRecover(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		called := false
		opt := &remote.ServerOption{
			GRPCUnknownServiceHandler: func(ctx context.Context, method string, stream streaming.Stream) error {
				test.Assert(t, method == "method", method)
				called = true
				return nil
			},
		}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		ivk := rpcinfo.NewInvocation("service", "method")
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelDetailed)
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, stat)
		err := svrHandler.unknownMethodWithRecover(context.Background(), ri, nil)
		test.Assert(t, err == nil, err)
		test.Assert(t, called, called)
		test.Assert(t, stat.GetEvent(stats.ServerHandleStart) != nil, stat)
		event := stat.GetEvent(stats.ServerHandleFinish)
		test.Assert(t, event.Status() == stats.StatusInfo, event)
		panicked, _ := stat.Panicked()
		test.Assert(t, !panicked, panicked)
	})
	t.Run("panic", func(t *testing.T) {
		called := false
		opt := &remote.ServerOption{
			GRPCUnknownServiceHandler: func(ctx context.Context, method string, stream streaming.Stream) error {
				called = true
				panic("panic")
			},
		}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		ivk := rpcinfo.NewInvocation("service", "method")
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelDetailed)
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, stat)
		err := svrHandler.unknownMethodWithRecover(context.Background(), ri, nil)
		test.Assert(t, err != nil, err)
		test.Assert(t, called, called)
		event := stat.GetEvent(stats.ServerHandleFinish)
		test.Assert(t, event.Status() == stats.StatusError, event)
		panicked, panicInfo := stat.Panicked()
		test.Assert(t, panicked && panicInfo == "panic", panicked, panicInfo)
	})
}

func Test_ttheaderStreamingTransHandler_invokeUnknownMethod(t *testing.T) {
	t.Run("unknown-method:no-err", func(t *testing.T) {
		opt := &remote.ServerOption{
			GRPCUnknownServiceHandler: func(ctx context.Context, method string, stream streaming.Stream) error {
				return nil
			},
		}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		ivk := rpcinfo.NewInvocation("service", "method")
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelDetailed)
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		err := svrHandler.invokeUnknownMethod(ctx, ri, nil, nil)

		test.Assert(t, err == nil, err)
	})
	t.Run("unknown-method:normal-err", func(t *testing.T) {
		handlerErr := errors.New("error")
		opt := &remote.ServerOption{
			GRPCUnknownServiceHandler: func(ctx context.Context, method string, stream streaming.Stream) error {
				return handlerErr
			},
		}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		ivk := rpcinfo.NewInvocation("service", "method")
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelDetailed)
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		err := svrHandler.invokeUnknownMethod(ctx, ri, nil, nil)

		var detailedErr *kerrors.DetailedError
		test.Assert(t, errors.As(err, &detailedErr), detailedErr)
		test.Assert(t, detailedErr.Is(kerrors.ErrBiz))
		test.Assert(t, errors.Is(handlerErr, detailedErr.Unwrap()))
	})

	t.Run("unknown-method:biz-status-err", func(t *testing.T) {
		handlerErr := kerrors.NewBizStatusError(100, "error")
		opt := &remote.ServerOption{
			GRPCUnknownServiceHandler: func(ctx context.Context, method string, stream streaming.Stream) error {
				return handlerErr
			},
		}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		ivk := rpcinfo.NewInvocation("service", "method")
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelDetailed)
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		err := svrHandler.invokeUnknownMethod(ctx, ri, nil, nil)

		test.Assert(t, err == nil, err)
		bizErr := ri.Invocation().BizStatusErr()
		test.Assert(t, bizErr != nil, bizErr)
		test.Assert(t, bizErr.BizStatusCode() == handlerErr.BizStatusCode())
		test.Assert(t, bizErr.BizMessage() == handlerErr.BizMessage())
	})

	t.Run("no-unknown-method,no-svc-info", func(t *testing.T) {
		opt := &remote.ServerOption{
			GRPCUnknownServiceHandler: nil,
		}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		ivk := rpcinfo.NewInvocation("service", "method")
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelDetailed)
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		err := svrHandler.invokeUnknownMethod(ctx, ri, nil, nil)

		test.Assert(t, err != nil, err)
		var transErr *remote.TransError
		errors.As(err, &transErr)
		test.Assert(t, transErr.TypeID() == remote.UnknownService, transErr.TypeID())
	})

	t.Run("no-unknown-method,has-svc-info", func(t *testing.T) {
		opt := &remote.ServerOption{
			GRPCUnknownServiceHandler: nil,
		}
		h, _ := NewSvrTransHandler(opt, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		ivk := rpcinfo.NewInvocation("service", "method")
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelDetailed)
		ri := rpcinfo.NewRPCInfo(nil, nil, ivk, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		err := svrHandler.invokeUnknownMethod(ctx, ri, nil, &serviceinfo.ServiceInfo{})

		test.Assert(t, err != nil, err)
		var transErr *remote.TransError
		errors.As(err, &transErr)
		test.Assert(t, transErr.TypeID() == remote.UnknownMethod, transErr.TypeID())
	})
}

func Test_ttheaderStreamingTransHandler_newCtxWithRPCInfo(t *testing.T) {
	expectedAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8888")
	opt := &remote.ServerOption{
		InitOrResetRPCInfoFunc: func(info rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
			test.Assert(t, reflect.DeepEqual(addr, expectedAddr), addr)
			from := rpcinfo.NewEndpointInfo("from-service", "from-method", addr, nil)
			cfg := rpcinfo.NewRPCConfig()
			ri := rpcinfo.NewRPCInfo(from, nil, nil, cfg, nil)
			return ri
		},
	}
	h, _ := NewSvrTransHandler(opt, nil)
	svrHandler := h.(*ttheaderStreamingTransHandler)
	conn := &mockConn{
		remoteAddr: expectedAddr,
	}
	ctx, ri := svrHandler.newCtxWithRPCInfo(context.Background(), conn)
	test.Assert(t, rpcinfo.GetRPCInfo(ctx) != nil)
	test.Assert(t, ri.Config().TransportProtocol() == transport.TTHeader, ri)
}

func Test_ttheaderStreamingTransHandler_onEndStream(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8888")
	conn := &mockConn{
		remoteAddr: addr,
	}

	t.Run("normal", func(t *testing.T) {
		ctrl := &rpcinfo.TraceController{}
		h, _ := NewSvrTransHandler(&remote.ServerOption{
			TracerCtl: ctrl,
		}, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelBase)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		gotErr := errors.New("got error")
		st := &mockStream{
			doCloseSend: func(err error) error {
				gotErr = err
				return nil
			},
		}
		err := svrHandler.onEndStream(ctx, conn, ri, nil, nil, st)
		test.Assert(t, err == nil, err)
		test.Assert(t, gotErr == nil, gotErr)
	})

	t.Run("err", func(t *testing.T) {
		ctrl := &rpcinfo.TraceController{}
		h, _ := NewSvrTransHandler(&remote.ServerOption{
			TracerCtl: ctrl,
		}, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelBase)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		invokeErr := errors.New("invokeErr")
		gotErr := errors.New("got error")
		st := &mockStream{
			doCloseSend: func(err error) error {
				gotErr = err
				return nil
			},
		}
		err := svrHandler.onEndStream(ctx, conn, ri, nil, invokeErr, st)
		test.Assert(t, errors.Is(err, invokeErr), err)
		test.Assert(t, errors.Is(gotErr, invokeErr), gotErr)
	})

	t.Run("panic", func(t *testing.T) {
		ctrl := &rpcinfo.TraceController{}
		h, _ := NewSvrTransHandler(&remote.ServerOption{
			TracerCtl: ctrl,
		}, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelBase)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		gotErr := errors.New("got error")
		st := &mockStream{
			doCloseSend: func(err error) error {
				gotErr = err
				return nil
			},
		}
		err := svrHandler.onEndStream(ctx, conn, ri, "panic", nil, st)
		test.Assert(t, strings.Contains(err.Error(), "panic"), err)
		test.Assert(t, strings.Contains(gotErr.Error(), "panic"), gotErr)
	})

	t.Run("normal+closeSendErr", func(t *testing.T) {
		ctrl := &rpcinfo.TraceController{}
		h, _ := NewSvrTransHandler(&remote.ServerOption{
			TracerCtl: ctrl,
		}, nil)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		stat := rpcinfo.NewRPCStats()
		rpcinfo.AsMutableRPCStats(stat).SetLevel(stats.LevelBase)
		ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, stat)
		ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)

		closeSendErr := errors.New("closeSendErr")
		st := &mockStream{
			doCloseSend: func(err error) error {
				return closeSendErr
			},
		}
		err := svrHandler.onEndStream(ctx, conn, ri, nil, nil, st)
		test.Assert(t, errors.Is(err, closeSendErr), err)
	})
}

func Test_ttheaderStreamingTransHandler_OnRead(t *testing.T) {
	idlServiceName := "idl-service"
	toMethodName := "to-method"
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8888")
	svcInfo := &serviceinfo.ServiceInfo{
		ServiceName:  "xxx",
		PayloadCodec: serviceinfo.Protobuf,
		Methods: map[string]serviceinfo.MethodInfo{
			toMethodName: serviceinfo.NewMethodInfo(
				nil, nil, nil, false,
				serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
			),
		},
	}

	t.Run("normal", func(t *testing.T) {
		opt := &remote.ServerOption{
			Option: remote.Option{},
			InitOrResetRPCInfoFunc: func(info rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
				from := rpcinfo.NewEndpointInfo("from-service", "from-method", addr, nil)
				to := rpcinfo.NewMutableEndpointInfo("to-service", toMethodName, addr, nil)
				ivk := rpcinfo.NewInvocation(idlServiceName, toMethodName)
				return rpcinfo.NewRPCInfo(from, to.ImmutableView(), ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
			},
			TracerCtl: &rpcinfo.TraceController{},
			SvcSearchMap: map[string]*serviceinfo.ServiceInfo{
				remote.BuildMultiServiceKey(idlServiceName, toMethodName): svcInfo,
			},
		}
		ext := &mockExtension{}
		h, err := NewSvrTransHandler(opt, ext)
		test.Assert(t, err == nil, err)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		invokeHandlerCalled := false
		svrHandler.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			invokeHandlerCalled = true
			return nil
		})

		writeCnt := int32(0)

		conn := &mockConn{
			readBuf: func() []byte {
				// create a client HeaderFrame for read
				ivk := rpcinfo.NewInvocation(idlServiceName, toMethodName)
				ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
				msg := remote.NewMessage(nil, svcInfo, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = ivk.MethodName()
				msg.TransInfo().TransStrInfo()[transmeta.HeaderIDLServiceName] = ivk.ServiceName()
				out := remote.NewWriterBuffer(1024)
				ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
				if err = ttheader.NewStreamCodec().Encode(ctx, msg, out); err != nil {
					panic(err)
				}
				buf, _ := out.Bytes()
				return buf
			}(),
			write: func(b []byte) (int, error) {
				ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, nil)
				msg := remote.NewMessage(nil, svcInfo, ri, remote.Stream, remote.Server)
				in := remote.NewReaderBuffer(b)
				if err := ttheader.NewStreamCodec().Decode(context.Background(), msg, in); err != nil {
					panic(err)
				}
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader)
				} else if writeCnt == 1 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer)
				} else {
					test.Assert(t, false, "writeCnt should be 0 or 1")
				}
				writeCnt += 1
				return len(b), nil
			},
			remoteAddr: addr,
		}

		ctx := context.Background()
		err = svrHandler.OnRead(ctx, conn)
		test.Assert(t, err == nil, err)
		test.Assert(t, invokeHandlerCalled, invokeHandlerCalled)
		test.Assert(t, writeCnt == 2, writeCnt)
	})

	t.Run("dirty-frame:ignore", func(t *testing.T) {
		opt := &remote.ServerOption{
			Option: remote.Option{},
			InitOrResetRPCInfoFunc: func(info rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
				from := rpcinfo.NewEndpointInfo("from-service", "from-method", addr, nil)
				to := rpcinfo.NewMutableEndpointInfo("to-service", toMethodName, addr, nil)
				ivk := rpcinfo.NewInvocation(idlServiceName, toMethodName)
				return rpcinfo.NewRPCInfo(from, to.ImmutableView(), ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
			},
			TracerCtl: &rpcinfo.TraceController{},
			SvcSearchMap: map[string]*serviceinfo.ServiceInfo{
				remote.BuildMultiServiceKey(idlServiceName, toMethodName): svcInfo,
			},
		}
		ext := &mockExtension{}
		h, err := NewSvrTransHandler(opt, ext)
		test.Assert(t, err == nil, err)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		invokeHandlerCalled := false
		svrHandler.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			invokeHandlerCalled = true
			return nil
		})

		conn := &mockConn{
			readBuf: func() []byte {
				// create a client HeaderFrame for read
				ivk := rpcinfo.NewInvocation(idlServiceName, toMethodName)
				ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
				msg := remote.NewMessage(nil, svcInfo, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeTrailer // Dirty Frame
				msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = ivk.MethodName()
				msg.TransInfo().TransStrInfo()[transmeta.HeaderIDLServiceName] = ivk.ServiceName()
				out := remote.NewWriterBuffer(1024)
				ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
				if err = ttheader.NewStreamCodec().Encode(ctx, msg, out); err != nil {
					panic(err)
				}
				buf, _ := out.Bytes()
				return buf
			}(),
			remoteAddr: addr,
		}

		ctx := context.Background()
		err = svrHandler.OnRead(ctx, conn)
		test.Assert(t, err == nil, err)
		test.Assert(t, !invokeHandlerCalled, invokeHandlerCalled)
	})

	t.Run("read-header-failed", func(t *testing.T) {
		opt := &remote.ServerOption{
			Option: remote.Option{},
			InitOrResetRPCInfoFunc: func(info rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
				from := rpcinfo.NewEndpointInfo("from-service", "from-method", addr, nil)
				to := rpcinfo.NewMutableEndpointInfo("to-service", toMethodName, addr, nil)
				ivk := rpcinfo.NewInvocation(idlServiceName, toMethodName)
				return rpcinfo.NewRPCInfo(from, to.ImmutableView(), ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
			},
			TracerCtl: &rpcinfo.TraceController{},
			SvcSearchMap: map[string]*serviceinfo.ServiceInfo{
				remote.BuildMultiServiceKey(idlServiceName, toMethodName): svcInfo,
			},
		}
		ext := &mockExtension{}
		h, err := NewSvrTransHandler(opt, ext)
		test.Assert(t, err == nil, err)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		invokeHandlerCalled := false
		svrHandler.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			invokeHandlerCalled = true
			return nil
		})

		conn := &mockConn{
			remoteAddr: addr,
			readBuf: func() []byte {
				// create a client HeaderFrame for read
				ivk := rpcinfo.NewInvocation(idlServiceName, toMethodName)
				ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
				msg := remote.NewMessage(nil, svcInfo, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = ivk.MethodName()
				msg.TransInfo().TransStrInfo()[transmeta.HeaderIDLServiceName] = ivk.ServiceName()
				out := remote.NewWriterBuffer(1024)
				ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
				if err = ttheader.NewStreamCodec().Encode(ctx, msg, out); err != nil {
					panic(err)
				}
				buf, _ := out.Bytes()
				return buf[0 : len(buf)-1] // short read
			}(),
		}

		ctx := context.Background()
		err = svrHandler.OnRead(ctx, conn)
		test.Assert(t, strings.HasSuffix(err.Error(), "error=EOF"), err.Error())
		test.Assert(t, !invokeHandlerCalled, invokeHandlerCalled)
	})

	t.Run("unknown-method", func(t *testing.T) {
		unknownMethodCalled := false
		opt := &remote.ServerOption{
			Option: remote.Option{},
			InitOrResetRPCInfoFunc: func(info rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
				from := rpcinfo.NewEndpointInfo("from-service", "from-method", addr, nil)
				to := rpcinfo.NewMutableEndpointInfo("to-service", toMethodName, addr, nil)
				ivk := rpcinfo.NewInvocation(idlServiceName, toMethodName)
				return rpcinfo.NewRPCInfo(from, to.ImmutableView(), ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
			},
			TracerCtl: &rpcinfo.TraceController{},
			SvcSearchMap: map[string]*serviceinfo.ServiceInfo{
				remote.BuildMultiServiceKey(idlServiceName, toMethodName): svcInfo,
			},
			GRPCUnknownServiceHandler: func(ctx context.Context, method string, stream streaming.Stream) error {
				unknownMethodCalled = true
				return nil
			},
		}
		ext := &mockExtension{}
		h, err := NewSvrTransHandler(opt, ext)
		test.Assert(t, err == nil, err)
		svrHandler := h.(*ttheaderStreamingTransHandler)
		invokeHandlerCalled := false
		svrHandler.SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
			invokeHandlerCalled = true
			return nil
		})

		writeCnt := int32(0)

		conn := &mockConn{
			readBuf: func() []byte {
				// create a client HeaderFrame for read
				ivk := rpcinfo.NewInvocation(idlServiceName, "unknown-method")
				ri := rpcinfo.NewRPCInfo(nil, nil, ivk, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
				msg := remote.NewMessage(nil, svcInfo, ri, remote.Stream, remote.Client)
				msg.TransInfo().TransIntInfo()[transmeta.FrameType] = codec.FrameTypeHeader
				msg.TransInfo().TransIntInfo()[transmeta.ToMethod] = ivk.MethodName()
				msg.TransInfo().TransStrInfo()[transmeta.HeaderIDLServiceName] = ivk.ServiceName()
				out := remote.NewWriterBuffer(1024)
				ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
				if err = ttheader.NewStreamCodec().Encode(ctx, msg, out); err != nil {
					panic(err)
				}
				buf, _ := out.Bytes()
				return buf
			}(),
			write: func(b []byte) (int, error) {
				ri := rpcinfo.NewRPCInfo(nil, nil, nil, nil, nil)
				msg := remote.NewMessage(nil, svcInfo, ri, remote.Stream, remote.Server)
				in := remote.NewReaderBuffer(b)
				if err := ttheader.NewStreamCodec().Decode(context.Background(), msg, in); err != nil {
					panic(err)
				}
				if writeCnt == 0 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeHeader)
				} else if writeCnt == 1 {
					test.Assert(t, codec.MessageFrameType(msg) == codec.FrameTypeTrailer)
				} else {
					test.Assert(t, false, "writeCnt should be 0 or 1")
				}
				writeCnt += 1
				return len(b), nil
			},
			remoteAddr: addr,
		}

		ctx := context.Background()
		err = svrHandler.OnRead(ctx, conn)
		test.Assert(t, err == nil, err)
		test.Assert(t, !invokeHandlerCalled, invokeHandlerCalled)
		test.Assert(t, unknownMethodCalled, unknownMethodCalled)
		test.Assert(t, writeCnt == 2, writeCnt)
	})
}
