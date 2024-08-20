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
	"sync/atomic"
	"testing"
	"time"
)

const testAddr = "127.0.0.1:12345"

func TestJSONRPC(t *testing.T) {
	ln, err := netpoll.CreateListener("tcp", testAddr)
	test.Assert(t, err == nil, err)

	// create server
	var serverStreamCount uint32
	svr, err := NewServer(
		new(serviceImpl),
		streamxserver.WithListener(ln),
		streamxserver.WithServerMiddleware[streamx.ServerStreamEndpoint](
			// middleware example: server streaming mode
			func(next streamx.ServerStreamEndpoint) streamx.ServerStreamEndpoint {
				return func(ctx context.Context, reqArgs any, streamArgs streamx.StreamArgs) error {
					log.Printf("Server middleware receive : reqArgs=%v streamArgs=%v", reqArgs, streamArgs)
					err := next(ctx, reqArgs, streamArgs)
					if err != nil {
						return err
					}
					atomic.AddUint32(&serverStreamCount, 1)
					stream := streamArgs.Stream()
					_ = stream
					return nil
				}
			}),
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
	)
	test.Assert(t, err == nil, err)

	// client stream
	cs, err := cli.ClientStream(ctx)
	test.Assert(t, err == nil, err)
	for i := 0; i < 10; i++ {
		req := new(Request)
		req.Type = int32(i)
		req.Message = "ClientStream"
		err = cs.Send(ctx, req)
		test.Assert(t, err == nil, err)
	}
	err = cs.CloseSend(ctx)
	test.Assert(t, err == nil, err)

	// server stream
	req := new(Request)
	req.Message = "ServerStream"
	ss, err := cli.ServerStream(ctx, req)
	test.Assert(t, err == nil, err)
	for {
		res, err := ss.Recv(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		test.Assert(t, err == nil, err)
		t.Logf("res: %v", res)
	}
	//count := atomic.LoadUint32(&serverStreamCount)
	//test.Assert(t, count == 1, count)

	// bidi stream
	bs, err := cli.BidiStream(ctx)
	test.Assert(t, err == nil, err)
	for {
		req := new(Request)
		req.Message = "ServerStream"

		err := bs.Send(ctx, req)
		test.Assert(t, err == nil, err)

		res, err := ss.Recv(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		test.Assert(t, err == nil, err)
		test.Assert(t, req.Message == res.Message, res)
	}
}
