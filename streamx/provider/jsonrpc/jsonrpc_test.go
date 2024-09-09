package jsonrpc_test

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/server/streamxserver"
	"github.com/cloudwego/kitex/streamx"
	"github.com/cloudwego/netpoll"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const testAddr = "127.0.0.1:12345"

func TestJSONRPC(t *testing.T) {
	ln, err := netpoll.CreateListener("tcp", testAddr)
	test.Assert(t, err == nil, err)

	// create server
	var serverStreamCount int32
	waitServerStreamDone := func() {
		for atomic.LoadInt32(&serverStreamCount) != 0 {
			t.Logf("waitServerStreamDone: %d", atomic.LoadInt32(&serverStreamCount))
			time.Sleep(time.Millisecond * 100)
		}
	}
	methodCount := map[string]int{}
	svr, err := NewServer(
		new(serviceImpl),
		streamxserver.WithListener(ln),
		streamxserver.WithStreamMiddleware(
			// middleware example: server streaming mode
			func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
				return func(ctx context.Context, streamArgs streamx.StreamArgs, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs) (err error) {
					log.Printf("Server middleware before next: reqArgs=%v resArgs=%v streamArgs=%v",
						reqArgs.Req(), resArgs.Res(), streamArgs)
					test.Assert(t, streamArgs.Stream() != nil)

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
	time.Sleep(time.Millisecond * 100)

	// create client
	ctx := context.Background()
	cli, err := NewClient(
		"a.b.c",
		streamxclient.WithHostPorts(testAddr),
		//streamxclient.WithStreamRecvMiddleware(func(next streamx.StreamRecvEndpoint) streamx.StreamRecvEndpoint {
		//	return func(ctx context.Context, stream streamx.Stream, res any) (err error) {
		//		log.Println("xxx")
		//		return next(ctx, stream, res)
		//	}
		//}),
		streamxclient.WithStreamMiddleware(func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
			return func(ctx context.Context, streamArgs streamx.StreamArgs, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs) (err error) {
				log.Printf("Client middleware before next: reqArgs=%v resArgs=%v streamArgs=%v",
					reqArgs.Req(), resArgs.Res(), streamArgs.Stream())
				err = next(ctx, streamArgs, reqArgs, resArgs)
				log.Printf("Client middleware after next: reqArgs=%v resArgs=%v streamArgs=%v",
					reqArgs.Req(), resArgs.Res(), streamArgs.Stream())

				test.Assert(t, streamArgs.Stream() != nil)
				switch streamArgs.Stream().Mode() {
				case streamx.StreamingUnary:
					test.Assert(t, reqArgs.Req() != nil)
					test.Assert(t, resArgs.Res() != nil)
				case streamx.StreamingClient:
					test.Assert(t, reqArgs.Req() == nil)
					test.Assert(t, resArgs.Res() == nil)
				case streamx.StreamingServer:
					test.Assert(t, reqArgs.Req() != nil)
					test.Assert(t, resArgs.Res() == nil)
				case streamx.StreamingBidirectional:
					test.Assert(t, reqArgs.Req() == nil)
					test.Assert(t, resArgs.Res() == nil)
				}
				test.Assert(t, err == nil, err)
				return err
			}
		}),
	)
	test.Assert(t, err == nil, err)

	t.Logf("=== Unary ===")
	req := new(Request)
	req.Message = "Unary"
	res, err := cli.Unary(ctx, req)
	test.Assert(t, err == nil, err)
	test.Assert(t, req.Message == res.Message, res.Message)
	atomic.AddInt32(&serverStreamCount, -1)
	waitServerStreamDone()

	// client stream
	t.Logf("=== ClientStream ===")
	cs, err := cli.ClientStream(ctx)
	test.Assert(t, err == nil, err)
	for i := 0; i < 3; i++ {
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

	// server stream
	t.Logf("=== ServerStream ===")
	req = new(Request)
	req.Message = "ServerStream"
	ss, err := cli.ServerStream(ctx, req)
	test.Assert(t, err == nil, err)
	for {
		res, err := ss.Recv(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		test.Assert(t, err == nil, err)
		t.Logf("Client ServerStream recv: %v", res)
	}
	//err = ss.CloseSend(ctx)
	//test.Assert(t, err == nil, err)
	atomic.AddInt32(&serverStreamCount, -1)
	waitServerStreamDone()

	// bidi stream
	t.Logf("=== BidiStream ===")
	bs, err := cli.BidiStream(ctx)
	test.Assert(t, err == nil, err)
	round := 5
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
}
