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
	"sync"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/cloudwego/kitex/pkg/streaming"
)

type mockStreamTracer struct {
	t        *testing.T
	eventBuf []stats.Event
	mu       sync.Mutex
	finishCh chan struct{}
}

func newMockStreamTracer() *mockStreamTracer {
	return &mockStreamTracer{
		finishCh: make(chan struct{}, 1),
	}
}

func (tracer *mockStreamTracer) Start(ctx context.Context) context.Context {
	return ctx
}

func (tracer *mockStreamTracer) Finish(ctx context.Context) {
	tracer.finishCh <- struct{}{}
}

func (tracer *mockStreamTracer) SetT(t *testing.T) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	tracer.t = t
}

func (tracer *mockStreamTracer) HandleStreamStartEvent(_ context.Context, _ rpcinfo.RPCInfo, _ rpcinfo.StreamStartEvent) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	tracer.eventBuf = append(tracer.eventBuf, stats.StreamStart)
}

func (tracer *mockStreamTracer) HandleStreamRecvEvent(_ context.Context, _ rpcinfo.RPCInfo, _ rpcinfo.StreamRecvEvent) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	tracer.eventBuf = append(tracer.eventBuf, stats.StreamRecv)
}

func (tracer *mockStreamTracer) HandleStreamSendEvent(_ context.Context, _ rpcinfo.RPCInfo, _ rpcinfo.StreamSendEvent) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	tracer.eventBuf = append(tracer.eventBuf, stats.StreamSend)
}

func (tracer *mockStreamTracer) HandleStreamRecvHeaderEvent(_ context.Context, _ rpcinfo.RPCInfo, _ rpcinfo.StreamRecvHeaderEvent) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	tracer.eventBuf = append(tracer.eventBuf, stats.StreamRecvHeader)
}

func (tracer *mockStreamTracer) HandleStreamFinishEvent(_ context.Context, _ rpcinfo.RPCInfo, _ rpcinfo.StreamFinishEvent) {
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	tracer.eventBuf = append(tracer.eventBuf, stats.StreamFinish)
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
	// wait for Finish invoked
	<-tracer.finishCh
	tracer.eventBuf = tracer.eventBuf[:0]
}

func TestTrace_trace(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		t.Run("clientStreaming", func(t *testing.T) {
			cliTraceCtl := &rpcinfo.TraceController{}
			cliTracer := newMockStreamTracer()
			cliTraceCtl.Append(cliTracer)
			cliTraceCtl.AppendStreamEventHandler(rpcinfo.StreamEventHandler{
				HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
				HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
				HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
				HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
				HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
			})
			cliTracer.SetT(t)
			defer cliTracer.Clean()

			cconn, sconn := newTestConnectionPipe(t)
			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)
			ctrans := newClientTransport(cconn, nil)
			defer ctrans.Close(nil)
			ctx := context.Background()
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "ClientStreaming"})
			cs.setTraceController(cliTraceCtl)
			cs.RegisterCloseCallback(func(err error) {
				cliTracer.Finish(cs.ctx)
			})
			err := ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)
			cliTraceCtl.HandleStreamStartEvent(ctx, cs.rpcInfo, rpcinfo.StreamStartEvent{})

			strans := newServerTransport(sconn)
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				req := new(testRequest)
				req.B = "hello"
				for i := 0; i < 3; i++ {
					sErr := cs.SendMsg(context.Background(), req)
					test.Assert(t, sErr == nil, sErr)
				}
				sErr := cs.CloseSend(context.Background())
				test.Assert(t, sErr == nil, sErr)

				res := new(testResponse)
				rErr := cs.RecvMsg(context.Background(), res)
				test.Assert(t, rErr == nil, rErr)
				res = new(testResponse)
				rErr = cs.RecvMsg(context.Background(), res)
				test.Assert(t, rErr == io.EOF, rErr)
			}()

			// server
			req := new(testRequest)
			for i := 0; i < 3; i++ {
				rErr := ss.RecvMsg(context.Background(), req)
				test.Assert(t, rErr == nil, rErr)
			}
			rErr := ss.RecvMsg(context.Background(), req)
			test.Assert(t, rErr == io.EOF, rErr)
			res := new(testResponse)
			res.B = req.B
			sErr := ss.SendMsg(context.Background(), res)
			test.Assert(t, sErr == nil, sErr)
			sErr = ss.CloseSend(nil)
			test.Assert(t, sErr == nil, sErr)

			wg.Wait()
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("serverStreaming", func(t *testing.T) {
			cliTraceCtl := &rpcinfo.TraceController{}
			cliTracer := newMockStreamTracer()
			cliTraceCtl.Append(cliTracer)
			cliTraceCtl.AppendStreamEventHandler(rpcinfo.StreamEventHandler{
				HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
				HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
				HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
				HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
				HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
			})
			cliTracer.SetT(t)
			defer cliTracer.Clean()

			cconn, sconn := newTestConnectionPipe(t)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)

			ctrans := newClientTransport(cconn, nil)
			defer ctrans.Close(nil)
			ctx := context.Background()
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "ServerStreaming"})
			cs.setTraceController(cliTraceCtl)
			cs.RegisterCloseCallback(func(err error) {
				cliTracer.Finish(cs.ctx)
			})

			err := ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)

			// Manually report StreamStart
			cliTraceCtl.HandleStreamStartEvent(ctx, cs.rpcInfo, rpcinfo.StreamStartEvent{})

			strans := newServerTransport(sconn)
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			// client
			go func() {
				defer wg.Done()
				req := new(testRequest)
				req.B = "hello"
				sErr := cs.SendMsg(context.Background(), req)
				test.Assert(t, sErr == nil, sErr)

				sErr = cs.CloseSend(context.Background())
				test.Assert(t, sErr == nil, sErr)

				// Receive multiple responses
				res := new(testResponse)
				for {
					rErr := cs.RecvMsg(context.Background(), res)
					if rErr != nil {
						test.Assert(t, rErr == io.EOF, rErr)
						break
					}
				}
			}()

			// server
			req := new(testRequest)
			rErr := ss.RecvMsg(context.Background(), req)
			test.Assert(t, rErr == nil, rErr)
			rErr = ss.RecvMsg(context.Background(), req)
			test.Assert(t, rErr == io.EOF, rErr)

			// Send multiple responses
			res := new(testResponse)
			res.B = req.B
			for i := 0; i < 3; i++ {
				sErr := ss.SendMsg(context.Background(), res)
				test.Assert(t, sErr == nil, sErr)
			}
			sErr := ss.CloseSend(nil)
			test.Assert(t, sErr == nil, sErr)

			wg.Wait()

			// Verify trace events
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("bidiStreaming", func(t *testing.T) {
			cliTraceCtl := &rpcinfo.TraceController{}
			cliTracer := newMockStreamTracer()
			cliTraceCtl.Append(cliTracer)
			cliTraceCtl.AppendStreamEventHandler(rpcinfo.StreamEventHandler{
				HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
				HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
				HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
				HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
				HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
			})
			cliTracer.SetT(t)
			defer cliTracer.Clean()

			cconn, sconn := newTestConnectionPipe(t)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)

			ctrans := newClientTransport(cconn, nil)
			defer ctrans.Close(nil)
			ctx := context.Background()
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
			cs.setTraceController(cliTraceCtl)
			cs.RegisterCloseCallback(func(err error) {
				cliTracer.Finish(cs.ctx)
			})

			err := ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)

			// Manually report StreamStart
			cliTraceCtl.HandleStreamStartEvent(ctx, cs.rpcInfo, rpcinfo.StreamStartEvent{})

			strans := newServerTransport(sconn)
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(2)
			// client
			go func() {
				defer wg.Done()
				req := new(testRequest)
				req.B = "hello"
				sErr := cs.SendMsg(context.Background(), req)
				test.Assert(t, sErr == nil, sErr)

				sErr = cs.CloseSend(context.Background())
				test.Assert(t, sErr == nil, sErr)
			}()

			// client receive
			go func() {
				defer wg.Done()
				res := new(testResponse)
				rErr := cs.RecvMsg(context.Background(), res)
				test.Assert(t, rErr == nil, rErr)
				rErr = cs.RecvMsg(context.Background(), res)
				test.Assert(t, rErr == io.EOF, rErr)
			}()

			// server
			req := new(testRequest)
			rErr := ss.RecvMsg(context.Background(), req)
			test.Assert(t, rErr == nil, rErr)
			rErr = ss.RecvMsg(context.Background(), req)
			test.Assert(t, rErr == io.EOF, rErr)
			res := new(testResponse)
			res.B = req.B
			sErr := ss.SendMsg(context.Background(), res)
			test.Assert(t, sErr == nil, sErr)
			sErr = ss.CloseSend(nil)
			test.Assert(t, sErr == nil, sErr)

			wg.Wait()

			// Verify trace events
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
	})
	t.Run("return handler", func(t *testing.T) {
		t.Run("server returns immediately with no data", func(t *testing.T) {
			cliTraceCtl := &rpcinfo.TraceController{}
			cliTracer := newMockStreamTracer()
			cliTraceCtl.Append(cliTracer)
			cliTraceCtl.AppendStreamEventHandler(rpcinfo.StreamEventHandler{
				HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
				HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
				HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
				HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
				HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
			})
			cliTracer.SetT(t)
			defer cliTracer.Clean()

			cconn, sconn := newTestConnectionPipe(t)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)

			ctrans := newClientTransport(cconn, nil)
			defer ctrans.Close(nil)
			ctx := context.Background()
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Test"})
			cs.setTraceController(cliTraceCtl)
			cs.RegisterCloseCallback(func(err error) {
				cliTracer.Finish(cs.ctx)
			})

			err := ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)

			// Manually report StreamStart
			cliTraceCtl.HandleStreamStartEvent(ctx, cs.rpcInfo, rpcinfo.StreamStartEvent{})

			strans := newServerTransport(sconn)
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			// Server closes immediately with no data
			sErr := ss.CloseSend(nil)
			test.Assert(t, sErr == nil, sErr)

			// Client receives EOF
			res := new(testResponse)
			rErr := cs.RecvMsg(context.Background(), res)
			test.Assert(t, rErr == io.EOF, rErr)

			// Verify trace events: StreamStart -> StreamRecvHeader -> StreamFinish
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("server sends header then closes", func(t *testing.T) {
			cliTraceCtl := &rpcinfo.TraceController{}
			cliTracer := newMockStreamTracer()
			cliTraceCtl.Append(cliTracer)
			cliTraceCtl.AppendStreamEventHandler(rpcinfo.StreamEventHandler{
				HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
				HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
				HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
				HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
				HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
			})
			cliTracer.SetT(t)
			defer cliTracer.Clean()

			cconn, sconn := newTestConnectionPipe(t)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)

			ctrans := newClientTransport(cconn, nil)
			defer ctrans.Close(nil)
			ctx := context.Background()
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Test"})
			cs.setTraceController(cliTraceCtl)
			cs.RegisterCloseCallback(func(err error) {
				cliTracer.Finish(cs.ctx)
			})

			err := ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)

			// Manually report StreamStart
			cliTraceCtl.HandleStreamStartEvent(ctx, cs.rpcInfo, rpcinfo.StreamStartEvent{})

			strans := newServerTransport(sconn)
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			// Server sends header
			header := streaming.Header{"key": "value"}
			sErr := ss.SetHeader(header)
			test.Assert(t, sErr == nil, sErr)

			// Server closes without sending data
			sErr = ss.CloseSend(nil)
			test.Assert(t, sErr == nil, sErr)

			// Client receives EOF
			res := new(testResponse)
			rErr := cs.RecvMsg(context.Background(), res)
			test.Assert(t, rErr == io.EOF, rErr)

			// Verify trace events: StreamStart -> StreamRecvHeader -> StreamFinish
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("server sends partial data then closes", func(t *testing.T) {
			cliTraceCtl := &rpcinfo.TraceController{}
			cliTracer := newMockStreamTracer()
			cliTraceCtl.Append(cliTracer)
			cliTraceCtl.AppendStreamEventHandler(rpcinfo.StreamEventHandler{
				HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
				HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
				HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
				HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
				HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
			})
			cliTracer.SetT(t)
			defer cliTracer.Clean()

			cconn, sconn := newTestConnectionPipe(t)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)

			ctrans := newClientTransport(cconn, nil)
			defer ctrans.Close(nil)
			ctx := context.Background()
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Test"})
			cs.setTraceController(cliTraceCtl)
			cs.RegisterCloseCallback(func(err error) {
				cliTracer.Finish(cs.ctx)
			})

			err := ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)

			// Manually report StreamStart
			cliTraceCtl.HandleStreamStartEvent(ctx, cs.rpcInfo, rpcinfo.StreamStartEvent{})

			strans := newServerTransport(sconn)
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			// Server sends one message
			res := new(testResponse)
			res.B = "partial response"
			sErr := ss.SendMsg(context.Background(), res)
			test.Assert(t, sErr == nil, sErr)

			// Client receives the message
			clientRes := new(testResponse)
			rErr := cs.RecvMsg(context.Background(), clientRes)
			test.Assert(t, rErr == nil, rErr)
			test.Assert(t, clientRes.B == "partial response", clientRes.B)

			// Server closes after sending partial data
			sErr = ss.CloseSend(nil)
			test.Assert(t, sErr == nil, sErr)

			// Client receives EOF on next recv
			rErr = cs.RecvMsg(context.Background(), clientRes)
			test.Assert(t, rErr == io.EOF, rErr)

			// Verify trace events: StreamStart -> StreamRecvHeader -> StreamFinish
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
	})
	t.Run("cancel", func(t *testing.T) {
		t.Run("cancel after stream creation", func(t *testing.T) {
			cliTraceCtl := &rpcinfo.TraceController{}
			cliTracer := newMockStreamTracer()
			cliTraceCtl.Append(cliTracer)
			cliTraceCtl.AppendStreamEventHandler(rpcinfo.StreamEventHandler{
				HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
				HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
				HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
				HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
				HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
			})
			cliTracer.SetT(t)
			defer cliTracer.Clean()

			cconn, sconn := newTestConnectionPipe(t)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)

			ctrans := newClientTransport(cconn, nil)
			defer ctrans.Close(nil)
			ctx, cancel := context.WithCancel(context.Background())
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Test"})
			cs.setTraceController(cliTraceCtl)
			// Initialize rpcInfo for cancel handling
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo("client", "Test", nil, nil), nil, nil, nil, nil)
			cs.RegisterCloseCallback(func(err error) {
				cliTracer.Finish(cs.ctx)
			})

			err := ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)

			// Manually report StreamStart
			cliTraceCtl.HandleStreamStartEvent(ctx, cs.rpcInfo, rpcinfo.StreamStartEvent{})

			strans := newServerTransport(sconn)
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			// Cancel immediately after stream creation
			cancel()

			// Wait for cancellation to propagate
			res := new(testResponse)
			rErr := cs.RecvMsg(context.Background(), res)
			test.Assert(t, rErr != nil, rErr)

			// Server should receive RST frame
			req := new(testRequest)
			err = ss.RecvMsg(context.Background(), req)
			test.Assert(t, err != nil, err)

			// Verify trace events: StreamStart -> StreamFinish
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
		t.Run("cancel after sending data", func(t *testing.T) {
			cliTraceCtl := &rpcinfo.TraceController{}
			cliTracer := newMockStreamTracer()
			cliTraceCtl.Append(cliTracer)
			cliTraceCtl.AppendStreamEventHandler(rpcinfo.StreamEventHandler{
				HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
				HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
				HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
				HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
				HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
			})
			cliTracer.SetT(t)
			defer cliTracer.Clean()

			cconn, sconn := newTestConnectionPipe(t)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)

			ctrans := newClientTransport(cconn, nil)
			defer ctrans.Close(nil)
			ctx, cancel := context.WithCancel(context.Background())
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Test"})
			cs.setTraceController(cliTraceCtl)
			// Initialize rpcInfo for cancel handling
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo("client", "Test", nil, nil), nil, nil, nil, nil)
			cs.RegisterCloseCallback(func(err error) {
				cliTracer.Finish(cs.ctx)
			})

			err := ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)

			// Manually report StreamStart
			cliTraceCtl.HandleStreamStartEvent(ctx, cs.rpcInfo, rpcinfo.StreamStartEvent{})

			strans := newServerTransport(sconn)
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			// Send one message
			req := new(testRequest)
			req.B = "hello"
			sErr := cs.SendMsg(context.Background(), req)
			test.Assert(t, sErr == nil, sErr)

			// Server receives the message
			serverReq := new(testRequest)
			rErr := ss.RecvMsg(context.Background(), serverReq)
			test.Assert(t, rErr == nil, rErr)

			// Cancel after sending data
			cancel()

			// Wait for cancellation to propagate
			res := new(testResponse)
			rErr = cs.RecvMsg(context.Background(), res)
			test.Assert(t, rErr != nil, rErr)

			// Verify trace events: StreamStart -> StreamFinish
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
		t.Run("cancel after receiving data", func(t *testing.T) {
			cliTraceCtl := &rpcinfo.TraceController{}
			cliTracer := newMockStreamTracer()
			cliTraceCtl.Append(cliTracer)
			cliTraceCtl.AppendStreamEventHandler(rpcinfo.StreamEventHandler{
				HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
				HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
				HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
				HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
				HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
			})
			cliTracer.SetT(t)
			defer cliTracer.Clean()

			cconn, sconn := newTestConnectionPipe(t)

			intHeader := make(IntHeader)
			strHeader := make(streaming.Header)

			ctrans := newClientTransport(cconn, nil)
			defer ctrans.Close(nil)
			ctx, cancel := context.WithCancel(context.Background())
			cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Test"})
			cs.setTraceController(cliTraceCtl)
			// Initialize rpcInfo for cancel handling
			cs.rpcInfo = rpcinfo.NewRPCInfo(
				rpcinfo.NewEndpointInfo("client", "Test", nil, nil), nil, nil, nil, nil)
			cs.RegisterCloseCallback(func(err error) {
				cliTracer.Finish(cs.ctx)
			})

			err := ctrans.WriteStream(ctx, cs, intHeader, strHeader)
			test.Assert(t, err == nil, err)

			// Manually report StreamStart
			cliTraceCtl.HandleStreamStartEvent(ctx, cs.rpcInfo, rpcinfo.StreamStartEvent{})

			strans := newServerTransport(sconn)
			ss, err := strans.ReadStream(context.Background())
			test.Assert(t, err == nil, err)

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Server sends one response
				res := new(testResponse)
				res.B = "response"
				sErr := ss.SendMsg(context.Background(), res)
				test.Assert(t, sErr == nil, sErr)
			}()

			// Client receives one message
			res := new(testResponse)
			rErr := cs.RecvMsg(context.Background(), res)
			test.Assert(t, rErr == nil, rErr)

			// Cancel after receiving data
			cancel()

			// Try to receive again, should fail
			rErr = cs.RecvMsg(context.Background(), res)
			test.Assert(t, rErr != nil, rErr)

			wg.Wait()

			// Verify trace events: StreamStart -> StreamRecvHeader -> StreamFinish
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
	})
}
