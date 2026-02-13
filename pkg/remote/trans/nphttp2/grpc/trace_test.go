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

package grpc

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

const (
	normalUnaryMethod           = "normal_unary"
	normalClientStreamingMethod = "normal_clientStreaming"
	normalServerStreamingMethod = "normal_serverStreaming"
	normalBidiStreamingMethod   = "normal_bidiStreaming"

	returnHandlerUnaryRecvHeaderTrailer               = "return_handler_unary_recv_header_trailer"
	returnHandlerUnaryRecvTrailer                     = "return_handler_unary_recv_trailer"
	returnHandlerClientStreamingRecvHeaderDataTrailer = "return_handler_clientStreaming_recv_header_data_trailer"
	returnHandlerClientStreamingRecvHeaderTrailer     = "return_handler_clientStreaming_recv_header_trailer"
	returnHandlerClientStreamingRecvTrailer           = "return_handler_clientStreaming_recv_trailer"
	returnHandlerServerStreamingRecvHeaderDataTrailer = "return_handler_serverStreaming_recv_header_data_trailer"
	returnHandlerServerStreamingRecvHeaderTrailer     = "return_handler_serverStreaming_recv_header_trailer"
	returnHandlerServerStreamingRecvTrailer           = "return_handler_serverStreaming_recv_trailer"
	returnHandlerBidiStreamingRecvHeaderDataTrailer   = "return_handler_bidiStreaming_recv_header_data_trailer"
	returnHandlerBidiStreamingRecvHeaderTrailer       = "return_handler_bidiStreaming_recv_header_trailer"
	returnHandlerBidiStreamingRecvTrailer             = "return_handler_bidiStreaming_recv_trailer"

	cancelClientRecvNone                 = "cancel_client_recv_none"
	cancelClientRecvHeader               = "cancel_client_recv_header"
	cancelClientRecvHeaderData           = "cancel_client_recv_header_data"
	cancelServerRecvHeaderRst            = "cancel_server_recv_header_rst"
	cancelServerRecvHeaderDataRst        = "cancel_server_recv_header_data_rst"
	cancelServerRecvHeaderDataTrailerRst = "cancel_server_recv_header_data_trailer_rst"
)

type mockStreamTracer struct {
	t        *testing.T
	eventBuf []stats.Event
	mu       sync.Mutex
}

func (tracer *mockStreamTracer) Start(ctx context.Context) context.Context {
	return ctx
}

func (tracer *mockStreamTracer) Finish(_ context.Context) {}

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
	tracer.eventBuf = tracer.eventBuf[:0]
}

func Test_trace(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		cliTraceCtl := &rpcinfo.TraceController{}
		cliTracer := &mockStreamTracer{}
		cliTraceCtl.Append(cliTracer)
		cliTraceCtl.AppendClientStreamEventHandler(rpcinfo.ClientStreamEventHandler{
			HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
			HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
			HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
			HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
			HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
		})
		srv, cli := setUpWithOptions(t, 0,
			&ServerConfig{MaxStreams: math.MaxUint32}, fullStreamingMode,
			ConnectOptions{TraceController: cliTraceCtl},
		)
		defer srv.stop()
		t.Run("unary", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: normalUnaryMethod,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)
			req := []byte("hello")
			resp := make([]byte, len(req))
			err = cli.Write(s, nil, req, &Options{Last: true})
			test.Assert(t, err == nil, err)
			n, err := s.Read(resp)
			test.Assert(t, err == nil, err)
			test.Assert(t, n == len(resp), n)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)
			// wait for server finished
			<-srv.srvReady
			// Verify trace event
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("clientStreaming", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: normalClientStreamingMethod,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)
			req := []byte("hello")
			for i := 0; i < 10; i++ {
				err = cli.Write(s, nil, req, &Options{})
				test.Assert(t, err == nil, err)
			}
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == nil, err)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)
			// wait for server finished
			<-srv.srvReady
			// Verify trace event
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("serverStreaming", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: normalServerStreamingMethod,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)
			req := []byte("hello")
			err = cli.Write(s, nil, req, &Options{})
			test.Assert(t, err == nil, err)
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			for {
				_, err = s.Read(resp)
				if err == nil {
					continue
				}
				test.Assert(t, err == io.EOF, err)
				break
			}
			// wait for server finished
			<-srv.srvReady
			// Verify trace event
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("bidiStreaming", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: normalBidiStreamingMethod,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			for i := 0; i < 10; i++ {
				req := []byte("hello")
				err = cli.Write(s, nil, req, &Options{})
				test.Assert(t, err == nil, err)
				_, err = s.Read(resp)
				test.Assert(t, err == nil, err)
			}
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)
			// wait for server finished
			<-srv.srvReady
			// Verify trace event
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
	})
	t.Run("return handler", func(t *testing.T) {
		cliTraceCtl := &rpcinfo.TraceController{}
		cliTracer := &mockStreamTracer{}
		cliTraceCtl.Append(cliTracer)
		cliTraceCtl.AppendClientStreamEventHandler(rpcinfo.ClientStreamEventHandler{
			HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
			HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
			HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
			HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
			HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
		})
		srv, cli := setUpWithOptions(t, 0,
			&ServerConfig{MaxStreams: math.MaxUint32}, fullStreamingMode,
			ConnectOptions{TraceController: cliTraceCtl},
		)
		defer srv.stop()
		t.Run("unary - client_recv_frames - header->trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerUnaryRecvHeaderTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			_, err = s.Header()
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("unary - client_recv_frames - trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerUnaryRecvTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
		t.Run("clientStreaming - client_recv_frames - header->data->trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerClientStreamingRecvHeaderDataTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			for i := 0; i < 5; i++ {
				err = cli.Write(s, nil, req, &Options{})
				test.Assert(t, err == nil, err)
			}
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == nil, err)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("clientStreaming - client_recv_frames - header->trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerClientStreamingRecvHeaderTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			for i := 0; i < 5; i++ {
				err = cli.Write(s, nil, req, &Options{})
				test.Assert(t, err == nil, err)
			}
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			_, err = s.Header()
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("clientStreaming - client_recv_frames - trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerClientStreamingRecvTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			for i := 0; i < 5; i++ {
				err = cli.Write(s, nil, req, &Options{})
				test.Assert(t, err == nil, err)
			}
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
		t.Run("serverStreaming - client_recv_frames - header->data->trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerServerStreamingRecvHeaderDataTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			err = cli.Write(s, nil, req, &Options{})
			test.Assert(t, err == nil, err)
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			for i := 0; i < 3; i++ {
				_, err = s.Read(resp)
				test.Assert(t, err == nil, err)
			}
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("serverStreaming - client_recv_frames - header->trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerServerStreamingRecvHeaderTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			err = cli.Write(s, nil, req, &Options{})
			test.Assert(t, err == nil, err)
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			_, err = s.Header()
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("serverStreaming - client_recv_frames - trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerServerStreamingRecvTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			err = cli.Write(s, nil, req, &Options{})
			test.Assert(t, err == nil, err)
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
		t.Run("bidiStreaming - client_recv_frames - header->data->trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerBidiStreamingRecvHeaderDataTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			for i := 0; i < 5; i++ {
				err = cli.Write(s, nil, req, &Options{})
				test.Assert(t, err == nil, err)
			}
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			for i := 0; i < 3; i++ {
				_, err = s.Read(resp)
				test.Assert(t, err == nil, err)
			}
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("bidiStreaming - client_recv_frames - header->trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerBidiStreamingRecvHeaderTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			for i := 0; i < 5; i++ {
				err = cli.Write(s, nil, req, &Options{})
				test.Assert(t, err == nil, err)
			}
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			_, err = s.Header()
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("bidiStreaming - client_recv_frames - trailer", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: returnHandlerBidiStreamingRecvTrailer,
			}
			s, err := cli.NewStream(context.Background(), callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			for i := 0; i < 5; i++ {
				err = cli.Write(s, nil, req, &Options{})
				test.Assert(t, err == nil, err)
			}
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			resp := make([]byte, 5)
			_, err = s.Read(resp)
			test.Assert(t, err == io.EOF, err)

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
	})
	t.Run("cancel", func(t *testing.T) {
		cliTraceCtl := &rpcinfo.TraceController{}
		cliTracer := &mockStreamTracer{}
		cliTraceCtl.Append(cliTracer)
		cliTraceCtl.AppendClientStreamEventHandler(rpcinfo.ClientStreamEventHandler{
			HandleStreamStartEvent:      cliTracer.HandleStreamStartEvent,
			HandleStreamRecvEvent:       cliTracer.HandleStreamRecvEvent,
			HandleStreamSendEvent:       cliTracer.HandleStreamSendEvent,
			HandleStreamRecvHeaderEvent: cliTracer.HandleStreamRecvHeaderEvent,
			HandleStreamFinishEvent:     cliTracer.HandleStreamFinishEvent,
		})
		srv, cli := setUpWithOptions(t, 0,
			&ServerConfig{MaxStreams: math.MaxUint32}, cancel,
			ConnectOptions{TraceController: cliTraceCtl},
		)
		defer srv.stop()
		t.Run("client_recv_frames - none", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelClientRecvNone,
			}
			ctx, cancelFunc := context.WithCancel(context.Background())
			_, err := cli.NewStream(ctx, callHdr)
			test.Assert(t, err == nil, err)
			cancelFunc()

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
		t.Run("client_recv_frames - header", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelClientRecvHeader,
			}
			ctx, cancelFunc := context.WithCancel(context.Background())
			s, err := cli.NewStream(ctx, callHdr)
			test.Assert(t, err == nil, err)
			_, err = s.Header()
			test.Assert(t, err == nil, err)
			cancelFunc()

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("client_recv_frames - header->data", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelClientRecvHeaderData,
			}
			ctx, cancelFunc := context.WithCancel(context.Background())
			s, err := cli.NewStream(ctx, callHdr)
			test.Assert(t, err == nil, err)
			header := make([]byte, 5)
			_, err = s.Read(header)
			test.Assert(t, err == nil, err)
			sz := binary.BigEndian.Uint32(header[1:])
			msg := make([]byte, int(sz))
			_, err = s.Read(msg)
			test.Assert(t, err == nil, err)
			cancelFunc()

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamRecvHeader, stats.StreamFinish)
		})
		t.Run("server_recv_frames - header->rst", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelServerRecvHeaderRst,
			}
			ctx, cancelFunc := context.WithCancel(context.Background())
			_, err := cli.NewStream(ctx, callHdr)
			test.Assert(t, err == nil, err)

			cancelFunc()
			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
		t.Run("server_recv_frames - header->data->rst", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelServerRecvHeaderDataRst,
			}
			ctx, cancelFunc := context.WithCancel(context.Background())
			s, err := cli.NewStream(ctx, callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			err = cli.Write(s, nil, req, &Options{})
			test.Assert(t, err == nil, err)
			time.Sleep(50 * time.Millisecond)
			cancelFunc()

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
		t.Run("server_recv_frames - header->data->trailer->rst", func(t *testing.T) {
			cliTracer.SetT(t)
			defer cliTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelServerRecvHeaderDataTrailerRst,
			}
			ctx, cancelFunc := context.WithCancel(context.Background())
			s, err := cli.NewStream(ctx, callHdr)
			test.Assert(t, err == nil, err)

			req := []byte("hello")
			err = cli.Write(s, nil, req, &Options{})
			test.Assert(t, err == nil, err)
			err = cli.Write(s, nil, nil, &Options{Last: true})
			test.Assert(t, err == nil, err)
			time.Sleep(50 * time.Millisecond)
			cancelFunc()

			<-srv.srvReady
			cliTracer.Verify(stats.StreamStart, stats.StreamFinish)
		})
	})
}
