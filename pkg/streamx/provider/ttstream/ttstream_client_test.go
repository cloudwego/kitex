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
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/streamxserver"
	"github.com/cloudwego/kitex/transport"
	"github.com/cloudwego/netpoll"
)

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
	serverRecvCount := map[string]int{}
	serverSendCount := map[string]int{}
	svr := server.NewServer(server.WithListener(ln))
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
					serverRecvCount[stream.Method()]++
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
					serverSendCount[stream.Method()]++
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
	streamClient, err := NewStreamingClient(
		"kitex.service.streaming",
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
	test.Assert(t, serverRecvCount["Unary"] == 1, serverRecvCount)
	test.Assert(t, serverSendCount["Unary"] == 1, serverSendCount)
	atomic.AddInt32(&serverStreamCount, -1)
	waitServerStreamDone()

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
	test.Assert(t, serverRecvCount["ClientStream"] == round, serverRecvCount)
	test.Assert(t, serverSendCount["ClientStream"] == 1, serverSendCount)
	cs = nil
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
	test.Assert(t, serverRecvCount["ServerStream"] == 1, serverRecvCount)
	test.Assert(t, serverSendCount["ServerStream"] == received, serverSendCount)
	ss = nil
	runtime.GC()

	// bidi stream
	round = 5
	t.Logf("=== BidiStream ===")
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
	wg.Wait()
	atomic.AddInt32(&serverStreamCount, -1)
	waitServerStreamDone()
	test.Assert(t, serverRecvCount["BidiStream"] == round, serverRecvCount)
	test.Assert(t, serverSendCount["BidiStream"] == round, serverSendCount)
	bs = nil
	runtime.GC()

	streamClient = nil
}
