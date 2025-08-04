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
	"io"
	"math"
	"sync"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

const (
	normalUnaryMethod           = "normal_unary"
	normalClientStreamingMethod = "normal_clientStreaming"
	normalServerStreamingMethod = "normal_serverStreaming"
	normalBidiStreamingMethod   = "normal_bidiStreaming"

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

func Test_trace(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		cliTraceCtl := &rpcinfo.TraceController{}
		cliTracer := &mockStreamTracer{}
		cliTraceCtl.Append(cliTracer)
		srvTraceCtl := &rpcinfo.TraceController{}
		srvTracer := &mockStreamTracer{}
		srvTraceCtl.Append(srvTracer)
		srv, cli := setUpWithOptions(t, 0,
			&ServerConfig{MaxStreams: math.MaxUint32, TraceController: srvTraceCtl}, fullStreamingMode,
			ConnectOptions{TraceController: cliTraceCtl},
		)
		defer srv.stop()
		t.Run("unary", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()
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
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamRecvHeader, stats.StreamRecvTrailer)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamSendHeader, stats.StreamSendTrailer)
		})
		t.Run("clientStreaming", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()
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
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamRecvHeader, stats.StreamRecvTrailer)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamSendHeader, stats.StreamSendTrailer)
		})
		t.Run("serverStreaming", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()
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
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamRecvHeader, stats.StreamRecvTrailer)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamSendHeader, stats.StreamSendTrailer)
		})
		t.Run("bidiStreaming", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()
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
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamRecvHeader, stats.StreamSendTrailer, stats.StreamRecvTrailer)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamSendHeader, stats.StreamRecvTrailer, stats.StreamSendTrailer)
		})
	})
	t.Run("cancel", func(t *testing.T) {
		cliTraceCtl := &rpcinfo.TraceController{}
		cliTracer := &mockStreamTracer{}
		cliTraceCtl.Append(cliTracer)
		srvTraceCtl := &rpcinfo.TraceController{}
		srvTracer := &mockStreamTracer{}
		srvTraceCtl.Append(srvTracer)
		srv, cli := setUpWithOptions(t, 0,
			&ServerConfig{MaxStreams: math.MaxUint32, TraceController: srvTraceCtl}, cancel,
			ConnectOptions{TraceController: cliTraceCtl},
		)
		defer srv.stop()
		t.Run("server_recv - header->rst", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelServerRecvHeaderRst,
			}
			ctx, cancelFunc := context.WithCancel(context.Background())
			_, err := cli.NewStream(ctx, callHdr)
			test.Assert(t, err == nil, err)
			cancelFunc()
			// wait for server finished
			<-srv.srvReady
			// Verify trace event
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamSendRst)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamRecvRst)
		})
		t.Run("server_recv - header->data->rst", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()
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
			cancelFunc()
			// wait for server finished
			<-srv.srvReady
			// Verify trace event
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamSendRst)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamRecvRst)
		})
		t.Run("server_recv - header->data->trailer->rst", func(t *testing.T) {
			cliTracer.SetT(t)
			srvTracer.SetT(t)
			defer cliTracer.Clean()
			defer srvTracer.Clean()
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
			cancelFunc()
			// wait for server finished
			<-srv.srvReady
			// Verify trace event
			cliTracer.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamSendRst)
			srvTracer.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamRecvRst)
		})
	})
}
