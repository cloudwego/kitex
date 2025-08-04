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
	"io"
	"net"
	"sync"
	"testing"

	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type mockStreamTracer struct {
	t        *testing.T
	eventBuf []stats.Event
	mu       sync.Mutex
}

func (tracer *mockStreamTracer) Start(ctx context.Context) context.Context {
	return ctx
}

func (tracer *mockStreamTracer) Finish(ctx context.Context) {}

func (tracer *mockStreamTracer) SetT(t *testing.T) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	tracer.t = t
}

func (tracer *mockStreamTracer) ReportStreamEvent(ctx context.Context, ri rpcinfo.RPCInfo, event rpcinfo.Event) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	tracer.eventBuf = append(tracer.eventBuf, event.Event())
}

func (tracer *mockStreamTracer) Verify(expects ...stats.Event) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	t := tracer.t
	test.Assert(t, len(tracer.eventBuf) == len(expects), tracer.eventBuf)
	for i, e := range expects {
		test.Assert(t, e.Index() == tracer.eventBuf[i].Index(), tracer.eventBuf)
	}
}

func (tracer *mockStreamTracer) Clean() {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	tracer.eventBuf = tracer.eventBuf[:0]
}

type mockServiceSearcher struct {
	si *serviceinfo.ServiceInfo
}

func (m *mockServiceSearcher) SearchService(svcName, methodName string, strict bool, codecType serviceinfo.PayloadCodec) *serviceinfo.ServiceInfo {
	return m.si
}

func Test_trace(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		cfd, sfd := netpoll.GetSysFdPairs()
		cconn, err := netpoll.NewFDConnection(cfd)
		test.Assert(t, err == nil, err)
		sconn, err := netpoll.NewFDConnection(sfd)
		test.Assert(t, err == nil, err)
		defer cconn.Close()
		defer sconn.Close()

		cliTraceCtl := &rpcinfo.TraceController{}
		cliTracer := &mockStreamTracer{}
		cliTraceCtl.Append(cliTracer)
		srvTraceCtl := &rpcinfo.TraceController{}
		srvTracer := &mockStreamTracer{}
		srvTraceCtl.Append(srvTracer)

		ctrans := newTransport(clientTransport, cconn, nil)

		srvOpt := &remote.ServerOption{
			TTHeaderStreamingOptions: remote.TTHeaderStreamingOptions{
				TransportOptions: []interface{}{
					WithServerTraceController(srvTraceCtl),
				},
			},
			InitOrResetRPCInfoFunc: func(info rpcinfo.RPCInfo, addr net.Addr) rpcinfo.RPCInfo {
				ri := rpcinfo.NewRPCInfo(
					rpcinfo.EmptyEndpointInfo(),
					rpcinfo.EmptyEndpointInfo(),
					rpcinfo.NewServerInvocation(),
					rpcinfo.NewRPCConfig(),
					rpcinfo.NewRPCStats(),
				)
				rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(addr)
				return ri
			},
			TracerCtl: srvTraceCtl,
			SvcSearcher: &mockServiceSearcher{
				si: &serviceinfo.ServiceInfo{
					ServiceName: "traceTest",
					HandlerType: nil,
					Methods: map[string]serviceinfo.MethodInfo{
						"ClientStreaming": serviceinfo.NewMethodInfo(nil, nil, nil, false),
						"ServerStreaming": serviceinfo.NewMethodInfo(nil, nil, nil, false),
						"BidiStreaming":   serviceinfo.NewMethodInfo(nil, nil, nil, false),
					},
				},
			},
		}
		srvHdl, err := NewSvrTransHandlerFactory().NewTransHandler(srvOpt)
		test.Assert(t, err == nil, err)
		srvHdl.(remote.InvokeHandleFuncSetter).SetInvokeHandleFunc(func(ctx context.Context, rawReq, rawResp interface{}) (err error) {
			args, ok := rawReq.(*streaming.Args)
			test.Assert(t, ok)
			ss := args.ServerStream
			ri := rpcinfo.GetRPCInfo(ctx)
			switch ri.Invocation().MethodName() {
			case "ClientStreaming":
				for {
					req := new(testRequest)
					rErr := ss.RecvMsg(ctx, req)
					if rErr != nil {
						test.Assert(t, rErr == io.EOF)
						break
					}
				}
				err = ss.SendMsg(ctx, &testResponse{B: "ClientStreaming"})
				test.Assert(t, err == nil, err)
			case "ServerStreaming":
				req := new(testRequest)
				rErr := ss.RecvMsg(ctx, req)
				test.Assert(t, rErr == nil, rErr)
				rErr = ss.RecvMsg(ctx, req)
				test.Assert(t, rErr == io.EOF, rErr)
				for i := 0; i < 10; i++ {
					sErr := ss.SendMsg(ctx, &testResponse{B: "ServerStreaming"})
					test.Assert(t, sErr == nil)
				}
			case "BidiStreaming":
				for {
					req := new(testRequest)
					rErr := ss.RecvMsg(ctx, req)
					if rErr != nil {
						test.Assert(t, rErr == io.EOF, rErr)
						break
					}
					sErr := ss.SendMsg(ctx, &testResponse{B: "BidiStreaming"})
					test.Assert(t, sErr == nil)
				}
			default:
				t.Fatalf("unknown method: %s", ri.Invocation().MethodName())
			}
			return nil
		})
		sCtx, err := srvHdl.OnActive(context.Background(), sconn)
		test.Assert(t, err == nil, err)
		go func() {
			srvHdl.OnRead(sCtx, sconn)
		}()

		test.Assert(t, err == nil, err)
		t.Run("ClientStreaming", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctx := context.Background()
			rawClientStream := newStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "ClientStreaming"})
			rawClientStream.setTraceController(cliTraceCtl)
			rawClientStream.setTraceContext(ctx)
			err = ctrans.WriteStream(ctx, rawClientStream, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			cs := newClientStream(rawClientStream)

			req := new(testRequest)
			req.B = "ClientStreaming"
			for i := 0; i < 10; i++ {
				err = cs.SendMsg(cs.Context(), req)
				test.Assert(t, err == nil, err)
			}
			err = cs.CloseSend(cs.Context())
			test.Assert(t, err == nil, err)
			resp := new(testResponse)
			err = cs.RecvMsg(cs.Context(), resp)
			test.Assert(t, err == nil, err)
			err = cs.RecvMsg(cs.Context(), resp)
			test.Assert(t, err == io.EOF, err)

			// wait for server finished
			_, err = cs.Trailer()
			test.Assert(t, err == nil, err)
			// Verify trace event
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamRecvHeader, stats.StreamRecvTrailer)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamSendHeader, stats.StreamSendTrailer)
		})
		t.Run("ServerStreaming", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctx := context.Background()
			rawClientStream := newStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "ServerStreaming"})
			rawClientStream.setTraceController(cliTraceCtl)
			rawClientStream.setTraceContext(ctx)
			err = ctrans.WriteStream(ctx, rawClientStream, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			cs := newClientStream(rawClientStream)

			req := new(testRequest)
			req.B = "ServerStreaming"
			err = cs.SendMsg(cs.Context(), req)
			test.Assert(t, err == nil, err)
			err = cs.CloseSend(cs.Context())
			test.Assert(t, err == nil, err)
			for {
				resp := new(testResponse)
				err = cs.RecvMsg(cs.Context(), resp)
				if err != nil {
					test.Assert(t, err == io.EOF, err)
					break
				}
			}

			// wait for server finished
			_, err = cs.Trailer()
			test.Assert(t, err == nil, err)
			// Verify trace event
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamRecvHeader, stats.StreamRecvTrailer)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamSendHeader, stats.StreamSendTrailer)
		})
		t.Run("BidiStreaming", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctx := context.Background()
			rawClientStream := newStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "BidiStreaming"})
			rawClientStream.setTraceController(cliTraceCtl)
			rawClientStream.setTraceContext(ctx)
			err = ctrans.WriteStream(ctx, rawClientStream, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			cs := newClientStream(rawClientStream)

			for i := 0; i < 10; i++ {
				req := new(testRequest)
				req.B = "BidiStreaming"
				err = cs.SendMsg(cs.Context(), req)
				test.Assert(t, err == nil, err)
				resp := new(testResponse)
				err = cs.RecvMsg(cs.Context(), resp)
				test.Assert(t, err == nil, err)
			}

			err = cs.CloseSend(cs.Context())
			test.Assert(t, err == nil, err)
			resp := new(testResponse)
			err = cs.RecvMsg(cs.Context(), resp)
			test.Assert(t, err == io.EOF, err)
			// wait for server finished
			_, err = cs.Trailer()
			test.Assert(t, err == nil, err)
			// Verify trace event
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamRecvHeader, stats.StreamSendTrailer, stats.StreamRecvTrailer)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamSendHeader, stats.StreamRecvTrailer, stats.StreamSendTrailer)
		})
	})
}
