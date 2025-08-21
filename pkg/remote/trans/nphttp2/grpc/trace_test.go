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

type mockSpanKey struct{}

type mockSpan struct{}

func newCtxWithSpan(ctx context.Context) context.Context {
	return context.WithValue(ctx, mockSpanKey{}, &mockSpan{})
}

func getSpanFromCtx(ctx context.Context) *mockSpan {
	if span, ok := ctx.Value(mockSpanKey{}).(*mockSpan); ok {
		return span
	}
	return nil
}

type mockStreamEventHandler struct {
	t        *testing.T
	eventBuf []stats.Event
	mu       sync.Mutex
}

func (hdl *mockStreamEventHandler) SetT(t *testing.T) {
	hdl.mu.Lock()
	defer hdl.mu.Unlock()
	hdl.t = t
}

func (hdl *mockStreamEventHandler) Report(ctx context.Context, evt stats.Event, err error) {
	hdl.mu.Lock()
	defer hdl.mu.Unlock()
	t := hdl.t
	span := getSpanFromCtx(ctx)
	// span must be initialized
	test.Assert(t, span != nil)
	hdl.eventBuf = append(hdl.eventBuf, evt)
}

func (hdl *mockStreamEventHandler) Verify(expects ...stats.Event) {
	hdl.mu.Lock()
	defer hdl.mu.Unlock()
	t := hdl.t
	test.Assert(t, len(hdl.eventBuf) == len(expects), hdl.eventBuf)
	for i, e := range expects {
		test.Assert(t, e.Index() == hdl.eventBuf[i].Index(), hdl.eventBuf)
	}
}

func (hdl *mockStreamEventHandler) Clean() {
	hdl.mu.Lock()
	defer hdl.mu.Unlock()
	hdl.eventBuf = hdl.eventBuf[:0]
}

func Test_trace(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		cliEventHdl := &mockStreamEventHandler{}
		srvEventHdl := &mockStreamEventHandler{}
		srv, cli := setUpWithOptions(t, 0,
			&ServerConfig{MaxStreams: math.MaxUint32, StreamEventHandler: srvEventHdl.Report}, fullStreamingMode,
			ConnectOptions{StreamEventHandler: cliEventHdl.Report},
		)
		defer srv.stop()
		t.Run("unary", func(t *testing.T) {
			cliEventHdl.SetT(t)
			srvEventHdl.SetT(t)
			defer cliEventHdl.Clean()
			defer srvEventHdl.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: normalUnaryMethod,
			}
			ctx := newCtxWithSpan(context.Background())
			s, err := cli.NewStream(ctx, callHdr)
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
			cliEventHdl.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamRecvHeader, stats.StreamRecvTrailer)
			srvEventHdl.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamSendHeader, stats.StreamSendTrailer)
		})
		t.Run("clientStreaming", func(t *testing.T) {
			cliEventHdl.SetT(t)
			srvEventHdl.SetT(t)
			defer cliEventHdl.Clean()
			defer srvEventHdl.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: normalClientStreamingMethod,
			}
			ctx := newCtxWithSpan(context.Background())
			s, err := cli.NewStream(ctx, callHdr)
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
			cliEventHdl.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamRecvHeader, stats.StreamRecvTrailer)
			srvEventHdl.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamSendHeader, stats.StreamSendTrailer)
		})
		t.Run("serverStreaming", func(t *testing.T) {
			cliEventHdl.SetT(t)
			srvEventHdl.SetT(t)
			defer cliEventHdl.Clean()
			defer srvEventHdl.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: normalServerStreamingMethod,
			}
			ctx := newCtxWithSpan(context.Background())
			s, err := cli.NewStream(ctx, callHdr)
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
			cliEventHdl.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamRecvHeader, stats.StreamRecvTrailer)
			srvEventHdl.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamSendHeader, stats.StreamSendTrailer)
		})
		t.Run("bidiStreaming", func(t *testing.T) {
			cliEventHdl.SetT(t)
			srvEventHdl.SetT(t)
			defer cliEventHdl.Clean()
			defer srvEventHdl.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: normalBidiStreamingMethod,
			}
			ctx := newCtxWithSpan(context.Background())
			s, err := cli.NewStream(ctx, callHdr)
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
			cliEventHdl.Verify(stats.StreamSendHeader, stats.StreamRecvHeader, stats.StreamSendTrailer, stats.StreamRecvTrailer)
			srvEventHdl.Verify(stats.StreamRecvHeader, stats.StreamSendHeader, stats.StreamRecvTrailer, stats.StreamSendTrailer)
		})
	})
	t.Run("cancel", func(t *testing.T) {
		cliEventHdl := &mockStreamEventHandler{}
		srvEventHdl := &mockStreamEventHandler{}
		srv, cli := setUpWithOptions(t, 9999,
			&ServerConfig{MaxStreams: math.MaxUint32, StreamEventHandler: srvEventHdl.Report}, cancel,
			ConnectOptions{StreamEventHandler: cliEventHdl.Report},
		)
		defer srv.stop()
		t.Run("server_recv - header->rst", func(t *testing.T) {
			cliEventHdl.SetT(t)
			srvEventHdl.SetT(t)
			defer cliEventHdl.Clean()
			defer srvEventHdl.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelServerRecvHeaderRst,
			}
			ctx := newCtxWithSpan(context.Background())
			ctx, cancelFunc := context.WithCancel(ctx)
			_, err := cli.NewStream(ctx, callHdr)
			test.Assert(t, err == nil, err)
			cancelFunc()
			// wait for server finished
			<-srv.srvReady
			// Verify trace event
			cliEventHdl.Verify(stats.StreamSendHeader, stats.StreamSendRst)
			srvEventHdl.Verify(stats.StreamRecvHeader, stats.StreamRecvRst)
		})
		t.Run("server_recv - header->data->rst", func(t *testing.T) {
			cliEventHdl.SetT(t)
			srvEventHdl.SetT(t)
			defer cliEventHdl.Clean()
			defer srvEventHdl.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelServerRecvHeaderDataRst,
			}
			ctx := newCtxWithSpan(context.Background())
			ctx, cancelFunc := context.WithCancel(ctx)
			s, err := cli.NewStream(ctx, callHdr)
			test.Assert(t, err == nil, err)
			req := []byte("hello")
			err = cli.Write(s, nil, req, &Options{})
			test.Assert(t, err == nil, err)
			cancelFunc()
			// wait for server finished
			<-srv.srvReady
			// Verify trace event
			cliEventHdl.Verify(stats.StreamSendHeader, stats.StreamSendRst)
			srvEventHdl.Verify(stats.StreamRecvHeader, stats.StreamRecvRst)
		})
		t.Run("server_recv - header->data->trailer->rst", func(t *testing.T) {
			cliEventHdl.SetT(t)
			srvEventHdl.SetT(t)
			defer cliEventHdl.Clean()
			defer srvEventHdl.Clean()
			callHdr := &CallHdr{
				Host:   "localhost",
				Method: cancelServerRecvHeaderDataTrailerRst,
			}
			ctx := newCtxWithSpan(context.Background())
			ctx, cancelFunc := context.WithCancel(ctx)
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
			cliEventHdl.Verify(stats.StreamSendHeader, stats.StreamSendTrailer, stats.StreamSendRst)
			srvEventHdl.Verify(stats.StreamRecvHeader, stats.StreamRecvTrailer, stats.StreamRecvRst)
		})
	})
}
