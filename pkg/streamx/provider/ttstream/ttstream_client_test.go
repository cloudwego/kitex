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

package ttstream_test

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/streamxserver"
	"github.com/cloudwego/kitex/transport"
	"github.com/cloudwego/netpoll"
)

func init() {
	klog.SetLevel(klog.LevelDebug)
}

func TestTTHeaderStreaming(t *testing.T) {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	var addr = test.GetLocalAddress()
	ln, err := netpoll.CreateListener("tcp", addr)
	test.Assert(t, err == nil, err)
	defer ln.Close()

	// create server
	var serverStreamCount int32
	waitServerStreamDone := func() {
		for atomic.LoadInt32(&serverStreamCount) != 0 {
			t.Logf("waitServerStreamDone: %d", atomic.LoadInt32(&serverStreamCount))
			time.Sleep(time.Millisecond * 100)
		}
	}
	methodCount := map[string]int{}
	var serverRecvCount int32
	var serverSendCount int32
	svr := server.NewServer(server.WithListener(ln), server.WithExitWaitTime(time.Millisecond*10))
	// register pingpong service
	err = svr.RegisterService(pingpongServiceInfo, new(pingpongService))
	test.Assert(t, err == nil, err)
	// register streamingService as ttstreaam provider
	sp, err := ttstream.NewServerProvider(streamingServiceInfo)
	test.Assert(t, err == nil, err)
	err = svr.RegisterService(
		streamingServiceInfo,
		new(streamingService),
		streamxserver.WithProvider(sp),
		streamxserver.WithStreamRecvMiddleware(func(next streamx.StreamRecvEndpoint) streamx.StreamRecvEndpoint {
			return func(ctx context.Context, stream streamx.Stream, res any) (err error) {
				err = next(ctx, stream, res)
				if err == nil {
					atomic.AddInt32(&serverRecvCount, 1)
				} else {
					log.Printf("server recv middleware err=%v", err)
				}
				return err
			}
		}),
		streamxserver.WithStreamSendMiddleware(func(next streamx.StreamSendEndpoint) streamx.StreamSendEndpoint {
			return func(ctx context.Context, stream streamx.Stream, req any) (err error) {
				err = next(ctx, stream, req)
				if err == nil {
					atomic.AddInt32(&serverSendCount, 1)
				} else {
					log.Printf("server send middleware err=%v", err)
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
					test.Assert(t, ValidateMetadata(ctx))

					log.Printf("Server handler start")
					switch streamArgs.Stream().Mode() {
					case streamx.StreamingUnary:
						test.Assert(t, reqArgs.Req() != nil)
						test.Assert(t, resArgs.Res() == nil)
						err = next(ctx, streamArgs, reqArgs, resArgs)
						test.Assert(t, reqArgs.Req() != nil)
						test.Assert(t, resArgs.Res() != nil)
					case streamx.StreamingClient:
						test.Assert(t, reqArgs.Req() == nil)
						test.Assert(t, resArgs.Res() == nil)
						err = next(ctx, streamArgs, reqArgs, resArgs)
						test.Assert(t, reqArgs.Req() == nil)
						test.Assert(t, resArgs.Res() != nil)
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
					test.Assert(t, err == nil, err)
					methodCount[streamArgs.Stream().Method()]++
					log.Printf("Server handler end")

					log.Printf("Server middleware after next: reqArgs=%v resArgs=%v streamArgs=%v",
						reqArgs.Req(), resArgs.Res(), streamArgs.Stream())
					atomic.AddInt32(&serverStreamCount, 1)
					return nil
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
	cp, _ := ttstream.NewClientProvider(streamingServiceInfo, ttstream.WithClientLongConnPool(ttstream.DefaultLongConnConfig))
	streamClient, err := NewStreamingClient(
		"kitex.service.streaming",
		streamxclient.WithProvider(cp),
		streamxclient.WithHostPorts(addr),
		streamxclient.WithStreamRecvMiddleware(func(next streamx.StreamRecvEndpoint) streamx.StreamRecvEndpoint {
			return func(ctx context.Context, stream streamx.Stream, res any) (err error) {
				err = next(ctx, stream, res)
				log.Printf("Client recv middleware %v", res)
				return err
			}
		}),
		streamxclient.WithStreamSendMiddleware(func(next streamx.StreamSendEndpoint) streamx.StreamSendEndpoint {
			return func(ctx context.Context, stream streamx.Stream, req any) (err error) {
				err = next(ctx, stream, req)
				log.Printf("Client send middleware %v", req)
				return err
			}
		}),
		streamxclient.WithStreamMiddleware(func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
			return func(ctx context.Context, streamArgs streamx.StreamArgs, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs) (err error) {
				// validate ctx
				test.Assert(t, ValidateMetadata(ctx))

				log.Printf("Client middleware before next: reqArgs=%v resArgs=%v streamArgs=%v",
					reqArgs.Req(), resArgs.Res(), streamArgs.Stream())
				err = next(ctx, streamArgs, reqArgs, resArgs)
				test.Assert(t, err == nil, err)
				log.Printf("Client middleware after next: reqArgs=%v resArgs=%v streamArgs=%v",
					reqArgs.Req(), resArgs.Res(), streamArgs.Stream())

				test.Assert(t, streamArgs.Stream() != nil)
				switch streamArgs.Stream().Mode() {
				case streamx.StreamingUnary:
					test.Assert(t, reqArgs.Req() != nil)
					test.Assert(t, resArgs.Res() != nil)
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
	ctx := context.Background()
	ctx = SetMetadata(ctx)

	t.Logf("=== PingPong ===")
	req := new(Request)
	req.Message = "PingPong"
	res, err := pingpongClient.PingPong(ctx, req)
	test.Assert(t, err == nil, err)
	test.Assert(t, req.Message == res.Message, res)

	t.Logf("=== Unary ===")
	req = new(Request)
	req.Type = 10000
	req.Message = "Unary"
	res, err = streamClient.Unary(ctx, req)
	test.Assert(t, err == nil, err)
	test.Assert(t, req.Type == res.Type, res.Type)
	test.Assert(t, req.Message == res.Message, res.Message)
	test.Assert(t, serverRecvCount == 1, serverRecvCount)
	test.Assert(t, serverSendCount == 1, serverSendCount)
	atomic.AddInt32(&serverStreamCount, -1)
	waitServerStreamDone()
	serverRecvCount = 0
	serverSendCount = 0

	// client stream
	round := 5
	t.Logf("=== ClientStream ===")
	cs, err := streamClient.ClientStream(ctx)
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
	t.Logf("Client ClientStream CloseAndRecv: %v", res)
	atomic.AddInt32(&serverStreamCount, -1)
	waitServerStreamDone()
	test.Assert(t, serverRecvCount == int32(round), serverRecvCount)
	test.Assert(t, serverSendCount == 1, serverSendCount)
	cs = nil
	serverRecvCount = 0
	serverSendCount = 0
	runtime.GC()

	// server stream
	t.Logf("=== ServerStream ===")
	req = new(Request)
	req.Message = "ServerStream"
	ss, err := streamClient.ServerStream(ctx, req)
	test.Assert(t, err == nil, err)
	// server stream recv header
	hd, err := ss.Header()
	test.Assert(t, err == nil, err)
	t.Logf("Client ServerStream recv header: %v", hd)
	test.DeepEqual(t, hd["key1"], "val1")
	test.DeepEqual(t, hd["key2"], "val2")
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
	// server stream recv trailer
	tl, err := ss.Trailer()
	test.Assert(t, err == nil, err)
	t.Logf("Client ServerStream recv trailer: %v", tl)
	test.DeepEqual(t, tl["key1"], "val1")
	test.DeepEqual(t, tl["key2"], "val2")
	atomic.AddInt32(&serverStreamCount, -1)
	waitServerStreamDone()
	test.Assert(t, serverRecvCount == 1, serverRecvCount)
	test.Assert(t, serverSendCount == int32(received), serverSendCount)
	ss = nil
	serverRecvCount = 0
	serverSendCount = 0
	runtime.GC()

	// bidi stream
	t.Logf("=== BidiStream ===")
	concurrent := 32
	round = 5
	for c := 0; c < concurrent; c++ {
		atomic.AddInt32(&serverStreamCount, -1)
		go func() {
			bs, err := streamClient.BidiStream(ctx)
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
		}()
	}
	waitServerStreamDone()
	test.Assert(t, serverRecvCount == int32(concurrent*round), serverRecvCount)
	test.Assert(t, serverSendCount == int32(concurrent*round), serverSendCount)
	serverRecvCount = 0
	serverSendCount = 0
	runtime.GC()

	streamClient = nil
}

func TestTTHeaderStreamingLongConn(t *testing.T) {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	var addr = test.GetLocalAddress()
	ln, _ := netpoll.CreateListener("tcp", addr)
	defer ln.Close()

	// create server
	svr := server.NewServer(server.WithListener(ln), server.WithExitWaitTime(time.Millisecond*10))
	// register streamingService as ttstreaam provider
	sp, _ := ttstream.NewServerProvider(streamingServiceInfo)
	_ = svr.RegisterService(
		streamingServiceInfo,
		new(streamingService),
		streamxserver.WithProvider(sp),
	)
	go func() {
		_ = svr.Run()
	}()
	defer svr.Stop()
	test.WaitServerStart(addr)

	numGoroutine := runtime.NumGoroutine()
	cp, _ := ttstream.NewClientProvider(
		streamingServiceInfo,
		ttstream.WithClientLongConnPool(
			ttstream.LongConnConfig{MaxIdleTimeout: time.Second},
		),
	)
	streamClient, _ := NewStreamingClient(
		"kitex.service.streaming",
		streamxclient.WithHostPorts(addr),
		streamxclient.WithProvider(cp),
	)
	ctx := context.Background()

	msg := "BidiStream"
	streams := 500
	var wg sync.WaitGroup
	for i := 0; i < streams; i++ {
		wg.Add(1)
		bs, err := streamClient.BidiStream(ctx)
		test.Assert(t, err == nil, err)
		req := new(Request)
		req.Message = msg
		err = bs.Send(ctx, req)
		test.Assert(t, err == nil, err)
		go func() {
			res, err := bs.Recv(ctx)
			test.Assert(t, err == nil, err)
			err = bs.CloseSend(ctx)
			test.Assert(t, err == nil, err)
			test.Assert(t, res.Message == msg, res.Message)
			wg.Done()
		}()
	}
	wg.Wait()
	for {
		ng := runtime.NumGoroutine()
		if ng-numGoroutine < 100 {
			break
		}
		runtime.GC()
		time.Sleep(time.Second)
		t.Logf("current goroutines=%d, before =%d", ng, numGoroutine)
	}
}

func BenchmarkTTHeaderStreaming(b *testing.B) {
	klog.SetLevel(klog.LevelWarn)
	var addr = test.GetLocalAddress()
	ln, _ := netpoll.CreateListener("tcp", addr)
	defer ln.Close()

	// create server
	svr := server.NewServer(server.WithListener(ln), server.WithExitWaitTime(time.Millisecond*10))
	// register streamingService as ttstreaam provider
	sp, _ := ttstream.NewServerProvider(streamingServiceInfo)
	_ = svr.RegisterService(streamingServiceInfo, new(streamingService), streamxserver.WithProvider(sp))
	go func() {
		_ = svr.Run()
	}()
	defer svr.Stop()
	test.WaitServerStart(addr)

	streamClient, _ := NewStreamingClient("kitex.service.streaming", streamxclient.WithHostPorts(addr))
	ctx := context.Background()
	bs, err := streamClient.BidiStream(ctx)
	if err != nil {
		b.Fatal(err)
	}
	msg := "BidiStream"
	var wg sync.WaitGroup
	wg.Add(1)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := new(Request)
		req.Message = msg
		err := bs.Send(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
		res, err := bs.Recv(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		_ = res
	}
	err = bs.CloseSend(ctx)
}
