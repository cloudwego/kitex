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

package jsonrpc_test

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/streamx"
	"github.com/cloudwego/kitex/pkg/streamx/provider/jsonrpc"
	"github.com/cloudwego/kitex/server/streamxserver"
	"github.com/cloudwego/netpoll"
)

func TestJSONRPC(t *testing.T) {
	addr := test.GetLocalAddress()
	ln, err := netpoll.CreateListener("tcp", addr)
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
	sp, err := jsonrpc.NewServerProvider(serviceInfo)
	test.Assert(t, err == nil, err)
	svr := streamxserver.NewServer(streamxserver.WithListener(ln))
	err = svr.RegisterService(serviceInfo, new(serviceImpl),
		streamxserver.WithProvider(sp),
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
		streamxclient.WithHostPorts(addr),
		streamxclient.WithStreamRecvMiddleware(func(next streamx.StreamRecvEndpoint) streamx.StreamRecvEndpoint {
			return func(ctx context.Context, stream streamx.Stream, res any) (err error) {
				return next(ctx, stream, res)
			}
		}),
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
	// err = ss.CloseSend(ctx)
	// test.Assert(t, err == nil, err)
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
