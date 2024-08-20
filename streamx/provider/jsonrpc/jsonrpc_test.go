package jsonrpc_test

import (
	"context"
	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/server/streamxserver"
	"github.com/cloudwego/kitex/streamx"
	"github.com/cloudwego/netpoll"
	"log"
	"runtime"
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
			runtime.Gosched()
		}
	}
	svr, err := NewServer(
		new(serviceImpl),
		streamxserver.WithListener(ln),
		streamxserver.WithMiddleware(
			// middleware example: server streaming mode
			func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
				return func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
					log.Printf("Server middleware before next: reqArgs=%v resArgs=%v streamArgs=%v",
						reqArgs.Req(), resArgs.Res(), streamArgs)
					err = next(ctx, reqArgs, resArgs, streamArgs)
					log.Printf("Server middleware after next: reqArgs=%v resArgs=%v streamArgs=%v",
						reqArgs.Req(), resArgs.Res(), streamArgs)
					if err != nil {
						return err
					}
					atomic.AddInt32(&serverStreamCount, 1)
					stream := streamArgs.Stream()
					_ = stream
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
		streamxclient.WithMiddleware(func(next streamx.StreamEndpoint) streamx.StreamEndpoint {
			return func(ctx context.Context, reqArgs streamx.StreamReqArgs, resArgs streamx.StreamResArgs, streamArgs streamx.StreamArgs) (err error) {
				log.Printf("Client middleware before next: reqArgs=%v resArgs=%v streamArgs=%v",
					reqArgs.Req(), resArgs.Res(), streamArgs)
				err = next(ctx, reqArgs, resArgs, streamArgs)
				log.Printf("Client middleware after next: reqArgs=%v resArgs=%v streamArgs=%v",
					reqArgs.Req(), resArgs.Res(), streamArgs)
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
	err = cs.CloseSend(ctx)
	test.Assert(t, err == nil, err)
	atomic.AddInt32(&serverStreamCount, -1)
	waitServerStreamDone()

	// server stream
	//t.Logf("=== ServerStream ===")
	//req = new(Request)
	//req.Message = "ServerStream"
	//ss, err := cli.ServerStream(ctx, req)
	//test.Assert(t, err == nil, err)
	//for {
	//	res, err := ss.Recv(ctx)
	//	if errors.Is(err, io.EOF) {
	//		break
	//	}
	//	test.Assert(t, err == nil, err)
	//	t.Logf("res: %v", res)
	//}
	//atomic.AddInt32(&serverStreamCount, -1)
	//waitServerStreamDone()

	//// bidi stream
	//bs, err := cli.BidiStream(ctx)
	//test.Assert(t, err == nil, err)
	//for {
	//	req := new(Request)
	//	req.Message = "ServerStream"
	//
	//	err := bs.Send(ctx, req)
	//	test.Assert(t, err == nil, err)
	//
	//	res, err := ss.Recv(ctx)
	//	if errors.Is(err, io.EOF) {
	//		break
	//	}
	//	test.Assert(t, err == nil, err)
	//	test.Assert(t, req.Message == res.Message, res)
	//}
}
