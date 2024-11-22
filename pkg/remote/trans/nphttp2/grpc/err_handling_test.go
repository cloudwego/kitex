/*
 *
 * Copyright 2024 gRPC authors.
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
 *
 * This file may have been modified by CloudWeGo authors. All CloudWeGo
 * Modifications are Copyright 2021 CloudWeGo Authors.
 */

package grpc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/netpoll"
	"golang.org/x/net/http2/hpack"

	istatus "github.com/cloudwego/kitex/internal/remote/trans/grpc/status"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
)

const (
	testTypeKey                string = "testType"
	errBizCanceledVal          string = "errBizCanceledVal"
	errMiddleHeaderVal         string = "errMiddleHeaderVal"
	errDecodeHeaderVal         string = "errDecodeHeaderVal"
	errHTTP2StreamVal          string = "errHTTP2StreamVal"
	errClosedWithoutTrailerVal string = "errClosedWithoutTrailerVal"
	errRecvRstStreamVal        string = "errRecvRstStreamVal"
)

type expectedErrs map[string]struct {
	cliErr error
	srvErr error
}

func (errs expectedErrs) getClientExpectedErr(testType string) error {
	return errs[testType].cliErr
}

func (errs expectedErrs) getServerExpectedErr(testType string) error {
	return errs[testType].srvErr
}

func TestErrorHandling(t *testing.T) {
	t.Run("close stream", func(t *testing.T) {
		testcases := []struct {
			desc                 string
			setup                func(t *testing.T)
			clean                func(t *testing.T)
			customRstCodeMapping map[uint32]error
			errs                 expectedErrs
		}{
			{
				desc: "normal RstCode",
				errs: expectedErrs{
					errBizCanceledVal:          {kerrors.ErrBizCanceled, errRecvUpstreamRstStream},
					errMiddleHeaderVal:         {errMiddleHeader, errRecvUpstreamRstStream},
					errDecodeHeaderVal:         {errDecodeHeader, errRecvUpstreamRstStream},
					errHTTP2StreamVal:          {errHTTP2Stream, errRecvUpstreamRstStream},
					errClosedWithoutTrailerVal: {errClosedWithoutTrailer, nil},
					errRecvRstStreamVal:        {errRecvDownStreamRstStream, nil},
				},
			},
			{
				desc:  "custom RstCode",
				setup: customSetup,
				clean: customClean,
				errs: expectedErrs{
					errBizCanceledVal:          {kerrors.ErrBizCanceled, kerrors.ErrBizCanceled},
					errMiddleHeaderVal:         {errMiddleHeader, errMiddleHeader},
					errDecodeHeaderVal:         {errDecodeHeader, errDecodeHeader},
					errHTTP2StreamVal:          {errHTTP2Stream, errHTTP2Stream},
					errClosedWithoutTrailerVal: {errClosedWithoutTrailer, nil},
					errRecvRstStreamVal:        {kerrors.ErrGracefulShutdown, nil},
				},
			},
		}

		for _, tc := range testcases {
			t.Run(tc.desc, func(t *testing.T) {
				if tc.setup != nil {
					tc.setup(t)
				}
				if tc.clean != nil {
					defer func() {
						tc.clean(t)
					}()
				}
				lis, err := netpoll.CreateListener("tcp", "localhost:0")
				test.Assert(t, err == nil, err)
				_, port, err := net.SplitHostPort(lis.Addr().String())
				test.Assert(t, err == nil, err)
				cfg := &ServerConfig{}
				var wg sync.WaitGroup
				var onConnect netpoll.OnConnect = func(ctx context.Context, conn netpoll.Connection) context.Context {
					rawSrv, err := newHTTP2Server(ctx, conn, cfg)
					srv := rawSrv.(*http2Server)
					test.Assert(t, err == nil, err)
					srv.HandleStreams(func(stream *Stream) {
						wg.Add(1)
						go func() {
							defer wg.Done()
							md, ok := metadata.FromIncomingContext(stream.Context())
							test.Assert(t, ok)
							vals := md.Get(testTypeKey)
							test.Assert(t, len(vals) == 1, md)
							testType := vals[0]
							expectedErr := tc.errs.getServerExpectedErr(testType)
							switch testType {
							case errMiddleHeaderVal:
								errMiddleHeaderHandler(t, srv, stream, expectedErr)
							case errDecodeHeaderVal:
								errDecodeHeaderHandler(t, srv, stream, expectedErr)
							case errHTTP2StreamVal:
								errHTTP2StreamHandler(t, srv, stream, expectedErr)
							case errBizCanceledVal:
								errBizCanceledHandler(t, srv, stream, expectedErr)
							case errClosedWithoutTrailerVal:
								errClosedWithoutTrailerHandler(t, srv, stream, expectedErr)
							case errRecvRstStreamVal:
								errRecvRstStreamHandler(t, srv, stream, expectedErr)
							}
						}()
					}, func(ctx context.Context, s string) context.Context {
						return ctx
					})
					return nil
				}
				eventloop, err := netpoll.NewEventLoop(nil,
					netpoll.WithOnConnect(onConnect),
					netpoll.WithIdleTimeout(10*time.Second),
				)
				test.Assert(t, err == nil, err)
				go func() {
					eventloop.Serve(lis)
				}()
				// create http2Client
				conn, dErr := netpoll.NewDialer().DialTimeout("tcp", "localhost:"+port, time.Second)
				test.Assert(t, dErr == nil, dErr)
				cli, cErr := newHTTP2Client(context.Background(), conn.(netpoll.Connection), ConnectOptions{}, "", func(GoAwayReason) {}, func() {})
				test.Assert(t, cErr == nil, cErr)
				defer func() {
					wg.Wait()
					cli.Close(istatus.Err(codes.Internal, "test"))
					eventloop.Shutdown(context.Background())
					<-cli.readerDone
					<-cli.writerDone
				}()
				callHdr := &CallHdr{
					Host:   "host",
					Method: "method",
				}
				buf := make([]byte, 1)
				t.Run("Headers Frame appeared in the middle of the stream", func(t *testing.T) {
					testType := errMiddleHeaderVal
					expectedErr := tc.errs.getClientExpectedErr(testType)
					ctx := metadata.AppendToOutgoingContext(context.Background(), testTypeKey, testType)
					stream, err := cli.NewStream(ctx, callHdr)
					test.Assert(t, err == nil, err)
					_, err = stream.Header()
					test.Assert(t, err == nil, err)
					_, recvErr := stream.Read(buf)
					test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
				})
				t.Run("Decode Headers Frame failed", func(t *testing.T) {
					testType := errDecodeHeaderVal
					expectedErr := tc.errs.getClientExpectedErr(testType)
					ctx := metadata.AppendToOutgoingContext(context.Background(), testTypeKey, errDecodeHeaderVal)
					stream, err := cli.NewStream(ctx, callHdr)
					test.Assert(t, err == nil, err)
					_, recvErr := stream.Read(buf)
					test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
				})
				t.Run("HTTP2Stream err when parsing frame", func(t *testing.T) {
					testType := errMiddleHeaderVal
					expectedErr := tc.errs.getClientExpectedErr(testType)
					ctx := metadata.AppendToOutgoingContext(context.Background(), testTypeKey, testType)
					stream, err := cli.NewStream(ctx, callHdr)
					test.Assert(t, err == nil, err)
					_, recvErr := stream.Read(buf)
					test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
				})
				t.Run("Biz context canceled", func(t *testing.T) {
					testType := errBizCanceledVal
					expectedErr := tc.errs.getClientExpectedErr(testType)
					ctx := metadata.AppendToOutgoingContext(context.Background(), testTypeKey, testType)
					ctx, cancel := context.WithCancel(ctx)
					stream, err := cli.NewStream(ctx, callHdr)
					test.Assert(t, err == nil, err)
					_, err = stream.Header()
					test.Assert(t, err == nil, err)
					cancel()
					_, recvErr := stream.Read(buf)
					test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
				})
				t.Run("Stream closed without trailer frame", func(t *testing.T) {
					testType := errMiddleHeaderVal
					expectedErr := tc.errs.getClientExpectedErr(testType)
					ctx := metadata.AppendToOutgoingContext(context.Background(), testTypeKey, testType)
					stream, err := cli.NewStream(ctx, callHdr)
					test.Assert(t, err == nil, err)
					_, err = stream.Header()
					test.Assert(t, err == nil, err)
					_, recvErr := stream.Read(buf)
					if errors.Is(recvErr, io.EOF) {
						recvErr = stream.Status().Err()
					}
					test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
				})
				t.Run("Receive RstStream Frame", func(t *testing.T) {
					testType := errRecvRstStreamVal
					expectedErr := tc.errs.getClientExpectedErr(testType)
					ctx := metadata.AppendToOutgoingContext(context.Background(), testTypeKey, testType)
					stream, err := cli.NewStream(ctx, callHdr)
					test.Assert(t, err == nil, err)
					_, err = stream.Header()
					test.Assert(t, err == nil, err)
					_, recvErr := stream.Read(buf)
					if errors.Is(recvErr, io.EOF) {
						recvErr = stream.Status().Err()
					}
					test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
				})
			})
		}
	})
}

func errMiddleHeaderHandler(t *testing.T, srv *http2Server, stream *Stream, expectedErr error) {
	var err error
	buf := make([]byte, 5)
	err = stream.SendHeader(nil)
	test.Assert(t, err == nil, err)
	err = srv.controlBuf.put(&headerFrame{
		streamID:  stream.id,
		endStream: false,
	})
	test.Assert(t, err == nil, err)
	_, recvErr := stream.Read(buf)
	test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
}

func errDecodeHeaderHandler(t *testing.T, srv *http2Server, stream *Stream, expectedErr error) {
	var err error
	buf := make([]byte, 5)
	err = srv.controlBuf.put(&headerFrame{
		streamID: stream.id,
	})
	test.Assert(t, err == nil, err)
	_, recvErr := stream.Read(buf)
	test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
}

func errHTTP2StreamHandler(t *testing.T, srv *http2Server, stream *Stream, expectedErr error) {
	var err error
	buf := make([]byte, 5)
	err = srv.controlBuf.put(&headerFrame{
		streamID: stream.id,
		// regular header field is previous to pseudo header field
		hf: []hpack.HeaderField{
			{Name: "key", Value: "val"},
			{Name: ":status", Value: "200"},
		},
	})
	test.Assert(t, err == nil, err)
	_, recvErr := stream.Read(buf)
	test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
}

func errBizCanceledHandler(t *testing.T, srv *http2Server, stream *Stream, expectedErr error) {
	var err error
	buf := make([]byte, 5)
	err = stream.SendHeader(nil)
	test.Assert(t, err == nil, err)
	_, recvErr := stream.Read(buf)
	test.Assert(t, errors.Is(recvErr, expectedErr), recvErr)
}

func errClosedWithoutTrailerHandler(t *testing.T, srv *http2Server, stream *Stream, expectedErr error) {
	var err error
	err = stream.SendHeader(nil)
	test.Assert(t, err == nil, err)
	err = srv.controlBuf.put(&dataFrame{
		streamID:  stream.id,
		endStream: true,
	})
	test.Assert(t, err == nil, err)
}

func errRecvRstStreamHandler(t *testing.T, srv *http2Server, stream *Stream, expectedErr error) {
	err := stream.SendHeader(nil)
	test.Assert(t, err == nil, err)
	err = srv.controlBuf.put(&cleanupStream{
		streamID: stream.id,
		rst:      true,
		rstCode:  getRstCode(newStatus(codes.Unavailable, kerrors.ErrGracefulShutdown, "test").Err()),
		onWrite:  func() {},
	})
	test.Assert(t, err == nil, err)
}
