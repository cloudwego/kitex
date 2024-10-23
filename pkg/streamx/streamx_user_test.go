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

package streamx_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/streamxserver"
	"github.com/cloudwego/kitex/transport"
)

var providerTestCases []testCase

type testCase struct {
	Name           string
	ClientProvider streamx.ClientProvider
	ServerProvider streamx.ServerProvider
}

func init() {
	sp, _ := ttstream.NewServerProvider(streamingServiceInfo)
	cp, _ := ttstream.NewClientProvider(streamingServiceInfo, ttstream.WithClientLongConnPool(ttstream.LongConnConfig{MaxIdleTimeout: time.Millisecond * 100}))
	providerTestCases = append(providerTestCases, testCase{Name: "TTHeader_LongConn", ClientProvider: cp, ServerProvider: sp})
	cp, _ = ttstream.NewClientProvider(streamingServiceInfo, ttstream.WithClientShortConnPool())
	providerTestCases = append(providerTestCases, testCase{Name: "TTHeader_ShortConn", ClientProvider: cp, ServerProvider: sp})
	cp, _ = ttstream.NewClientProvider(streamingServiceInfo, ttstream.WithClientMuxConnPool())
	providerTestCases = append(providerTestCases, testCase{Name: "TTHeader_Mux", ClientProvider: cp, ServerProvider: sp})
}

func TestMain(m *testing.M) {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	// klog.SetLevel(klog.LevelDebug)
	m.Run()
}

func TestStreamingBasic(t *testing.T) {
	for _, tc := range providerTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			// === prepare test environment ===
			addr := test.GetLocalAddress()
			ln, err := netpoll.CreateListener("tcp", addr)
			test.Assert(t, err == nil, err)
			defer ln.Close()
			// create server
			var serverStreamCount int32
			waitServerStreamDone := func() {
				for atomic.LoadInt32(&serverStreamCount) != 0 {
					t.Logf("waitServerStreamDone: %d", atomic.LoadInt32(&serverStreamCount))
					time.Sleep(time.Millisecond * 10)
				}
			}
			var serverRecvCount int32
			var serverSendCount int32
			svr := server.NewServer(server.WithListener(ln), server.WithExitWaitTime(time.Millisecond*10))
			// register pingpong service
			err = svr.RegisterService(pingpongServiceInfo, new(pingpongService))
			test.Assert(t, err == nil, err)
			// register streamingService as ttstreaam provider
			err = svr.RegisterService(
				streamingServiceInfo,
				new(streamingService),
				streamxserver.WithProvider(tc.ServerProvider),
				streamxserver.WithStreamRecvMiddleware(func(next streamx.StreamRecvEndpoint) streamx.StreamRecvEndpoint {
					return func(ctx context.Context, stream streamx.Stream, res any) (err error) {
						err = next(ctx, stream, res)
						if err == nil {
							atomic.AddInt32(&serverRecvCount, 1)
						}
						return err
					}
				}),
				streamxserver.WithStreamSendMiddleware(func(next streamx.StreamSendEndpoint) streamx.StreamSendEndpoint {
					return func(ctx context.Context, stream streamx.Stream, req any) (err error) {
						err = next(ctx, stream, req)
						if err == nil {
							atomic.AddInt32(&serverSendCount, 1)
						}
						return err
					}
				}),
				streamxserver.WithStreamMiddleware(
					// middleware example: server streaming mode
					func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
						return func(ctx context.Context, streamArgs streamx.StreamArgs, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs) (err error) {
							log.Printf("Server middleware before next: reqArgs=%v resArgs=%v streamArgs=%v",
								reqArgs.Req(), resArgs.Res(), streamArgs)
							test.Assert(t, streamArgs.Stream() != nil)
							test.Assert(t, validateMetadata(ctx))

							switch streamArgs.Stream().Mode() {
							case streamx.StreamingUnary:
								test.Assert(t, reqArgs.Req() != nil)
								test.Assert(t, resArgs.Res() == nil)
								err = next(ctx, streamArgs, reqArgs, resArgs)
								test.Assert(t, reqArgs.Req() != nil)
								test.Assert(t, resArgs.Res() != nil || err != nil)
							case streamx.StreamingClient:
								test.Assert(t, reqArgs.Req() == nil)
								test.Assert(t, resArgs.Res() == nil)
								err = next(ctx, streamArgs, reqArgs, resArgs)
								test.Assert(t, reqArgs.Req() == nil)
								test.Assert(t, resArgs.Res() != nil || err != nil)
							case streamx.StreamingServer:
								test.Assert(t, reqArgs.Req() != nil)
								test.Assert(t, resArgs.Res() == nil)
								err = next(ctx, streamArgs, reqArgs, resArgs)
								test.Assert(t, reqArgs.Req() != nil)
								test.Assert(t, resArgs.Res() == nil)
							case streamx.StreamingBidirectional:
								test.Assert(t, reqArgs.Req() == nil)
								test.Assert(t, resArgs.Res() == nil)
								err = next(ctx, streamArgs, reqArgs, resArgs)
								test.Assert(t, reqArgs.Req() == nil)
								test.Assert(t, resArgs.Res() == nil)
							}

							log.Printf("Server middleware after next: reqArgs=%v resArgs=%v streamArgs=%v err=%v",
								reqArgs.Req(), resArgs.Res(), streamArgs.Stream(), err)
							atomic.AddInt32(&serverStreamCount, 1)
							return err
						}
					},
				),
			)
			test.Assert(t, err == nil, err)
			go func() {
				err := svr.Run()
				test.Assert(t, err == nil, err)
			}()
			defer svr.Stop()
			test.WaitServerStart(addr)

			// create client
			pingpongClient, err := NewPingPongClient(
				"kitex.service.pingpong",
				client.WithHostPorts(addr),
				client.WithTransportProtocol(transport.TTHeaderFramed),
				client.WithPayloadCodec(thrift.NewThriftCodecWithConfig(thrift.FastRead|thrift.FastWrite|thrift.EnableSkipDecoder)),
			)
			test.Assert(t, err == nil, err)
			// create streaming client
			streamClient, err := NewStreamingClient(
				"kitex.service.streaming",
				streamxclient.WithProvider(tc.ClientProvider),
				streamxclient.WithHostPorts(addr),
				streamxclient.WithStreamRecvMiddleware(func(next streamx.StreamRecvEndpoint) streamx.StreamRecvEndpoint {
					return func(ctx context.Context, stream streamx.Stream, res any) (err error) {
						err = next(ctx, stream, res)
						return err
					}
				}),
				streamxclient.WithStreamSendMiddleware(func(next streamx.StreamSendEndpoint) streamx.StreamSendEndpoint {
					return func(ctx context.Context, stream streamx.Stream, req any) (err error) {
						err = next(ctx, stream, req)
						return err
					}
				}),
				streamxclient.WithStreamMiddleware(func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
					return func(ctx context.Context, streamArgs streamx.StreamArgs, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs) (err error) {
						// validate ctx
						test.Assert(t, validateMetadata(ctx))

						err = next(ctx, streamArgs, reqArgs, resArgs)

						test.Assert(t, streamArgs.Stream() != nil)
						switch streamArgs.Stream().Mode() {
						case streamx.StreamingUnary:
							test.Assert(t, reqArgs.Req() != nil)
							test.Assert(t, resArgs.Res() != nil || err != nil)
						case streamx.StreamingClient:
							test.Assert(t, reqArgs.Req() == nil, reqArgs.Req())
							test.Assert(t, resArgs.Res() == nil)
						case streamx.StreamingServer:
							test.Assert(t, reqArgs.Req() != nil)
							test.Assert(t, resArgs.Res() == nil)
						case streamx.StreamingBidirectional:
							test.Assert(t, reqArgs.Req() == nil)
							test.Assert(t, resArgs.Res() == nil)
						}
						return err
					}
				}),
			)
			test.Assert(t, err == nil, err)

			// prepare metainfo
			octx := setMetadata(context.Background())

			t.Logf("=== PingPong ===")
			req := new(Request)
			req.Message = "PingPong"
			res, err := pingpongClient.PingPong(octx, req)
			test.Assert(t, err == nil, err)
			test.Assert(t, req.Message == res.Message, res)

			t.Logf("=== Unary ===")
			req = new(Request)
			req.Type = 10000
			req.Message = "Unary"
			res, err = streamClient.Unary(octx, req)
			test.Assert(t, err == nil, err)
			test.Assert(t, req.Type == res.Type, res.Type)
			test.Assert(t, req.Message == res.Message, res.Message)
			atomic.AddInt32(&serverStreamCount, -1)
			waitServerStreamDone()
			test.DeepEqual(t, atomic.LoadInt32(&serverRecvCount), int32(1))
			test.DeepEqual(t, atomic.LoadInt32(&serverSendCount), int32(1))
			atomic.StoreInt32(&serverRecvCount, 0)
			atomic.StoreInt32(&serverSendCount, 0)

			// client stream
			round := 5
			t.Logf("=== ClientStream ===")
			ctx, cs, err := streamClient.ClientStream(octx)
			test.Assert(t, err == nil, err)
			for i := 0; i < round; i++ {
				req := new(Request)
				req.Type = int32(i)
				req.Message = "ClientStream"
				err = cs.Send(ctx, req)
				test.Assert(t, err == nil, err)
			}
			res, err = cs.CloseAndRecv(ctx)
			test.Assert(t, err == nil, err)
			test.Assert(t, res.Message == "ClientStream", res.Message)
			atomic.AddInt32(&serverStreamCount, -1)
			waitServerStreamDone()
			testHeaderAndTrailer(t, cs)
			test.DeepEqual(t, atomic.LoadInt32(&serverRecvCount), int32(round))
			test.DeepEqual(t, atomic.LoadInt32(&serverSendCount), int32(1))
			atomic.StoreInt32(&serverRecvCount, 0)
			atomic.StoreInt32(&serverSendCount, 0)
			cs = nil
			runtime.GC()

			// server stream
			t.Logf("=== ServerStream ===")
			req = new(Request)
			req.Message = "ServerStream"
			ctx, ss, err := streamClient.ServerStream(octx, req)
			test.Assert(t, err == nil, err)
			received := 0
			for {
				res, err := ss.Recv(ctx)
				if errors.Is(err, io.EOF) {
					break
				}
				test.Assert(t, err == nil, err)
				received++
				t.Logf("Client ServerStream recv: %v", res)
			}
			err = ss.CloseSend(ctx)
			test.Assert(t, err == nil, err)
			atomic.AddInt32(&serverStreamCount, -1)
			waitServerStreamDone()
			testHeaderAndTrailer(t, ss)
			test.DeepEqual(t, atomic.LoadInt32(&serverRecvCount), int32(1))
			test.DeepEqual(t, atomic.LoadInt32(&serverSendCount), int32(received))
			atomic.StoreInt32(&serverRecvCount, 0)
			atomic.StoreInt32(&serverSendCount, 0)
			ss = nil
			runtime.GC()

			// bidi stream
			t.Logf("=== BidiStream ===")
			concurrent := 32
			round = 5
			for c := 0; c < concurrent; c++ {
				atomic.AddInt32(&serverStreamCount, -1)
				go func() {
					ctx, bs, err := streamClient.BidiStream(octx)
					test.Assert(t, err == nil, err)
					msg := "BidiStream"
					var wg sync.WaitGroup
					wg.Add(2)
					go func() {
						defer wg.Done()
						for i := 0; i < round; i++ {
							req := new(Request)
							req.Message = msg
							err := bs.Send(ctx, req)
							test.Assert(t, err == nil, err)
						}
						err = bs.CloseSend(ctx)
						test.Assert(t, err == nil, err)
					}()
					go func() {
						defer wg.Done()
						i := 0
						for {
							res, err := bs.Recv(ctx)
							if errors.Is(err, io.EOF) {
								break
							}
							i++
							test.Assert(t, err == nil, err)
							test.Assert(t, msg == res.Message, res.Message)
						}
						test.Assert(t, i == round, i)
					}()
					testHeaderAndTrailer(t, bs)
				}()
			}
			waitServerStreamDone()
			test.DeepEqual(t, atomic.LoadInt32(&serverRecvCount), int32(concurrent*round))
			test.DeepEqual(t, atomic.LoadInt32(&serverSendCount), int32(concurrent*round))
			atomic.StoreInt32(&serverRecvCount, 0)
			atomic.StoreInt32(&serverSendCount, 0)
			runtime.GC()

			t.Logf("=== UnaryWithErr normalErr ===")
			req = new(Request)
			req.Type = normalErr
			res, err = streamClient.UnaryWithErr(ctx, req)
			test.Assert(t, res == nil, res)
			test.Assert(t, err != nil, err)
			assertNormalErr(t, err)

			t.Logf("=== UnaryWithErr bizErr ===")
			req = new(Request)
			req.Type = bizErr
			res, err = streamClient.UnaryWithErr(ctx, req)
			test.Assert(t, res == nil, res)
			test.Assert(t, err != nil, err)
			assertBizErr(t, err)

			t.Logf("=== ClientStreamWithErr normalErr ===")
			ctx, cliStream, err := streamClient.ClientStreamWithErr(octx)
			test.Assert(t, err == nil, err)
			test.Assert(t, cliStream != nil, cliStream)
			req = new(Request)
			req.Type = normalErr
			err = cliStream.Send(ctx, req)
			test.Assert(t, err == nil, err)
			res, err = cliStream.CloseAndRecv(ctx)
			test.Assert(t, res == nil, res)
			test.Assert(t, err != nil, err)
			assertNormalErr(t, err)

			t.Logf("=== ClientStreamWithErr bizErr ===")
			ctx, cliStream, err = streamClient.ClientStreamWithErr(octx)
			test.Assert(t, err == nil, err)
			test.Assert(t, cliStream != nil, cliStream)
			req = new(Request)
			req.Type = bizErr
			err = cliStream.Send(ctx, req)
			test.Assert(t, err == nil, err)
			res, err = cliStream.CloseAndRecv(ctx)
			test.Assert(t, res == nil, res)
			test.Assert(t, err != nil, err)
			assertBizErr(t, err)

			t.Logf("=== ServerStreamWithErr normalErr ===")
			req = new(Request)
			req.Type = normalErr
			ctx, svrStream, err := streamClient.ServerStreamWithErr(octx, req)
			test.Assert(t, err == nil, err)
			test.Assert(t, svrStream != nil, svrStream)
			res, err = svrStream.Recv(ctx)
			test.Assert(t, res == nil, res)
			test.Assert(t, err != nil, err)
			assertNormalErr(t, err)

			t.Logf("=== ServerStreamWithErr bizErr ===")
			req = new(Request)
			req.Type = bizErr
			ctx, svrStream, err = streamClient.ServerStreamWithErr(octx, req)
			test.Assert(t, err == nil, err)
			test.Assert(t, svrStream != nil, svrStream)
			res, err = svrStream.Recv(ctx)
			test.Assert(t, res == nil, res)
			test.Assert(t, err != nil, err)
			assertBizErr(t, err)

			t.Logf("=== BidiStreamWithErr normalErr ===")
			ctx, bidiStream, err := streamClient.BidiStreamWithErr(octx)
			test.Assert(t, err == nil, err)
			test.Assert(t, bidiStream != nil, bidiStream)
			req = new(Request)
			req.Type = normalErr
			err = bidiStream.Send(ctx, req)
			test.Assert(t, err == nil, err)
			res, err = bidiStream.Recv(ctx)
			test.Assert(t, res == nil, res)
			test.Assert(t, err != nil, err)
			assertNormalErr(t, err)

			t.Logf("=== BidiStreamWithErr bizErr ===")
			ctx, bidiStream, err = streamClient.BidiStreamWithErr(octx)
			test.Assert(t, err == nil, err)
			test.Assert(t, bidiStream != nil, bidiStream)
			req = new(Request)
			req.Type = bizErr
			err = bidiStream.Send(ctx, req)
			test.Assert(t, err == nil, err)
			res, err = bidiStream.Recv(ctx)
			test.Assert(t, res == nil, res)
			test.Assert(t, err != nil, err)
			assertBizErr(t, err)

			t.Logf("=== Timeout by Ctx ===")
			ctx, bs, err := streamClient.BidiStream(octx)
			test.Assert(t, err == nil, err)
			req = new(Request)
			req.Message = string(make([]byte, 1024))
			err = bs.Send(ctx, req)
			test.Assert(t, err == nil, err)
			nctx, cancel := context.WithCancel(ctx)
			cancel()
			_, err = bs.Recv(nctx)
			test.Assert(t, err != nil, err)
			t.Logf("recv timeout error: %v", err)
			err = bs.CloseSend(ctx)
			test.Assert(t, err == nil, err)

			// timeout by client WithRecvTimeout
			t.Logf("=== Timeout by WithRecvTimeout ===")
			streamClient, _ = NewStreamingClient(
				"kitex.service.streaming",
				streamxclient.WithHostPorts(addr),
				streamxclient.WithProvider(tc.ClientProvider),
				streamxclient.WithRecvTimeout(time.Nanosecond),
			)
			ctx, bs, err = streamClient.BidiStream(octx)
			test.Assert(t, err == nil, err)
			req = new(Request)
			req.Message = string(make([]byte, 1024))
			err = bs.Send(ctx, req)
			test.Assert(t, err == nil, err)
			_, err = bs.Recv(ctx)
			test.Assert(t, err != nil, err)
			t.Logf("recv timeout error: %v", err)
			err = bs.CloseSend(ctx)
			test.Assert(t, err == nil, err)

			streamClient = nil
		})
	}
}

func TestStreamingGoroutineLeak(t *testing.T) {
	for _, tc := range providerTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			addr := test.GetLocalAddress()
			ln, _ := netpoll.CreateListener("tcp", addr)
			defer ln.Close()

			// create server
			svr := server.NewServer(server.WithListener(ln), server.WithExitWaitTime(time.Millisecond*10))
			var streamStarted int32
			waitStreamStarted := func(streamWaited int) {
				for {
					stated, waited := atomic.LoadInt32(&streamStarted), int32(streamWaited)
					if stated >= waited {
						return
					}
					t.Logf("streamStarted=%d < streamWaited=%d", stated, waited)
					time.Sleep(time.Millisecond * 10)
				}
			}
			_ = svr.RegisterService(
				streamingServiceInfo, new(streamingService),
				streamxserver.WithProvider(tc.ServerProvider),
				streamxserver.WithStreamMiddleware(func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
					return func(ctx context.Context, streamArgs streamx.StreamArgs, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs) (err error) {
						atomic.AddInt32(&streamStarted, 1)
						return next(ctx, streamArgs, reqArgs, resArgs)
					}
				}),
			)
			go func() {
				_ = svr.Run()
			}()
			defer svr.Stop()
			test.WaitServerStart(addr)

			streamClient, _ := NewStreamingClient(
				"kitex.service.streaming",
				streamxclient.WithHostPorts(addr),
				streamxclient.WithProvider(tc.ClientProvider),
			)
			octx := context.Background()
			msg := "BidiStream"

			t.Logf("=== Checking only one connection be reused ===")
			var wg sync.WaitGroup
			for i := 0; i < 12; i++ {
				wg.Add(1)
				ctx, bs, err := streamClient.BidiStream(octx)
				test.Assert(t, err == nil, err)
				req := new(Request)
				req.Message = string(make([]byte, 1024))
				err = bs.Send(ctx, req)
				test.Assert(t, err == nil, err)
				res, err := bs.Recv(ctx)
				test.Assert(t, err == nil, err)
				err = bs.CloseSend(ctx)
				test.Assert(t, err == nil, err)
				test.Assert(t, res.Message == req.Message, res.Message)
				runtime.SetFinalizer(bs, func(_ any) {
					wg.Done()
				})
				bs = nil
				runtime.GC()
				wg.Wait()
			}

			t.Logf("=== Checking streams GCed ===")
			streams := 100
			streamList := make([]streamx.ServerStream, streams)
			atomic.StoreInt32(&streamStarted, 0)
			for i := 0; i < streams; i++ {
				_, bs, err := streamClient.BidiStream(octx)
				test.Assert(t, err == nil, err)
				streamList[i] = bs
			}
			waitStreamStarted(streams)
			// before GC
			ngBefore := runtime.NumGoroutine()
			test.Assert(t, runtime.NumGoroutine() > streams, runtime.NumGoroutine())
			// after GC
			for i := 0; i < streams; i++ {
				streamList[i] = nil
			}
			for runtime.NumGoroutine() > ngBefore {
				t.Logf("ngCurrent=%d > ngBefore=%d", runtime.NumGoroutine(), ngBefore)
				runtime.GC()
				time.Sleep(time.Millisecond * 50)
			}

			t.Logf("=== Checking Streams Called and GCed ===")
			streams = 100
			for i := 0; i < streams; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					ctx, bs, err := streamClient.BidiStream(octx)
					test.Assert(t, err == nil, err)
					req := new(Request)
					req.Message = msg
					err = bs.Send(ctx, req)
					test.Assert(t, err == nil, err)

					res, err := bs.Recv(ctx)
					test.Assert(t, err == nil, err)
					err = bs.CloseSend(ctx)
					test.Assert(t, err == nil, err)
					test.Assert(t, res.Message == msg, res.Message)

					testHeaderAndTrailer(t, bs)
				}()
			}
			wg.Wait()
		})
	}
}

func TestStreamingException(t *testing.T) {
	for _, tc := range providerTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			addr := test.GetLocalAddress()
			ln, _ := netpoll.CreateListener("tcp", addr)
			defer ln.Close()

			// create server
			svr := server.NewServer(server.WithListener(ln), server.WithExitWaitTime(time.Millisecond*10))
			_ = svr.RegisterService(
				streamingServiceInfo, new(streamingService),
				streamxserver.WithProvider(tc.ServerProvider),
			)
			go func() {
				_ = svr.Run()
			}()
			defer svr.Stop()
			test.WaitServerStart(addr)

			var circuitBreaker int32
			circuitBreakerErr := fmt.Errorf("circuitBreaker on")
			streamClient, _ := NewStreamingClient(
				"kitex.service.streaming",
				streamxclient.WithHostPorts(addr),
				streamxclient.WithProvider(tc.ClientProvider),
				streamxclient.WithStreamMiddleware(func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
					return func(ctx context.Context, streamArgs streamx.StreamArgs, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs) (err error) {
						ri := rpcinfo.GetRPCInfo(ctx)
						test.Assert(t, ri.To().Address() != nil)
						if atomic.LoadInt32(&circuitBreaker) > 0 {
							return circuitBreakerErr
						}
						return next(ctx, streamArgs, reqArgs, resArgs)
					}
				}),
			)
			octx := context.Background()

			//assert circuitBreaker error
			atomic.StoreInt32(&circuitBreaker, 1)
			ctx, bs, err := streamClient.BidiStream(octx)
			test.Assert(t, errors.Is(err, circuitBreakerErr), err)
			atomic.StoreInt32(&circuitBreaker, 0)

			// assert context deadline error
			ctx, cancel := context.WithTimeout(octx, time.Millisecond)
			ctx, bs, err = streamClient.BidiStream(ctx)
			test.Assert(t, err == nil, err)
			res, err := bs.Recv(ctx)
			cancel()
			test.Assert(t, res == nil && err != nil, res, err)
			test.Assert(t, errors.Is(err, ctx.Err()), err)
			test.Assert(t, errors.Is(err, context.DeadlineExceeded), err)
		})
	}
}
