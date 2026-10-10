/*
 * Copyright 2026 CloudWeGo Authors
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

package localstream

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

func newTestRPCInfo() rpcinfo.RPCInfo {
	ink := rpcinfo.NewInvocation("svc", "method")
	return rpcinfo.NewRPCInfo(nil, nil, ink, rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
}

func TestPipeWriteRead(t *testing.T) {
	p := newPipe(0)
	ctx := context.Background()

	test.Assert(t, p.write(ctx, "a") == nil)
	v, err := p.read(ctx)
	test.Assert(t, err == nil, err)
	test.Assert(t, v.(string) == "a", v)
}

func TestPipeReadEOFAfterClose(t *testing.T) {
	p := newPipe(1)
	ctx := context.Background()
	test.Assert(t, p.write(ctx, 1) == nil)
	p.close(nil)

	// buffered item is still readable after close
	v, err := p.read(ctx)
	test.Assert(t, err == nil, err)
	test.Assert(t, v.(int) == 1, v)

	// once drained, read returns io.EOF
	_, err = p.read(ctx)
	test.Assert(t, err == io.EOF, err)
}

func TestPipeCloseWithErr(t *testing.T) {
	p := newPipe(1)
	ctx := context.Background()
	myErr := errors.New("boom")
	p.close(myErr)

	// write after close returns the stored err
	test.Assert(t, p.write(ctx, 1) == myErr)
	// read after close (empty) returns the stored err
	_, err := p.read(ctx)
	test.Assert(t, err == myErr, err)
}

func TestPipeCloseIdempotent(t *testing.T) {
	p := newPipe(1)
	p.close(nil)
	// second close must not panic (no double close of channel)
	p.close(errors.New("ignored"))
	_, err := p.read(context.Background())
	test.Assert(t, err == io.EOF, err)
}

func TestPipeWriteCtxCancel(t *testing.T) {
	p := newPipe(1)
	// fill the single buffer slot
	test.Assert(t, p.write(context.Background(), 1) == nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := p.write(ctx, 2)
	test.Assert(t, err == context.Canceled, err)
}

func TestPipeReadCtxCancel(t *testing.T) {
	p := newPipe(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.read(ctx)
	test.Assert(t, err == context.Canceled, err)
}

func TestAssignMessage(t *testing.T) {
	// value -> value
	var dstVal int
	test.Assert(t, assignMessage(&dstVal, 42) == nil)
	test.Assert(t, dstVal == 42, dstVal)

	// pointer src -> value dst (deref)
	src := 7
	dstVal = 0
	test.Assert(t, assignMessage(&dstVal, &src) == nil)
	test.Assert(t, dstVal == 7, dstVal)

	// pointer -> pointer
	var dstPtr *int
	test.Assert(t, assignMessage(&dstPtr, &src) == nil)
	test.Assert(t, dstPtr == &src, dstPtr)

	// nil src zeroes the dst
	dstVal = 99
	test.Assert(t, assignMessage(&dstVal, nil) == nil)
	test.Assert(t, dstVal == 0, dstVal)
}

func TestAssignMessageErrors(t *testing.T) {
	// nil target
	test.Assert(t, assignMessage(nil, 1) != nil)

	// non-pointer target
	test.Assert(t, assignMessage(1, 1) != nil)

	// nil pointer target
	var p *int
	test.Assert(t, assignMessage(p, 1) != nil)

	// incompatible types
	var dst int
	test.Assert(t, assignMessage(&dst, "string") != nil)
}

func TestEndpointBidirectional(t *testing.T) {
	ri := newTestRPCInfo()
	ctx := context.Background()
	client, server := NewPair(ctx, ri)

	// client -> server
	test.Assert(t, client.SendMsg(ctx, "ping") == nil)
	var got string
	test.Assert(t, server.RecvMsg(ctx, &got) == nil)
	test.Assert(t, got == "ping", got)

	// server -> client
	test.Assert(t, server.SendMsg(ctx, "pong") == nil)
	got = ""
	test.Assert(t, client.RecvMsg(ctx, &got) == nil)
	test.Assert(t, got == "pong", got)
}

func TestEndpointClientCloseSendUnblocksServerRecv(t *testing.T) {
	ri := newTestRPCInfo()
	ctx := context.Background()
	client, server := NewPair(ctx, ri)

	test.Assert(t, client.CloseSend(ctx) == nil)
	var got string
	err := server.RecvMsg(ctx, &got)
	test.Assert(t, err == io.EOF, err)
}

func TestEndpointFinishServerSurfacesBizErr(t *testing.T) {
	ri := newTestRPCInfo()
	setter := ri.Invocation().(rpcinfo.InvocationSetter)
	setter.SetBizStatusErr(kerrors.NewBizStatusError(100, "biz error"))

	ctx := context.Background()
	client, server := NewPair(ctx, ri)

	// server finishes the stream with EOF; because a biz error is stashed on
	// RPCInfo, the client should observe the biz error instead of io.EOF.
	server.FinishServer(nil)
	var got string
	err := client.RecvMsg(ctx, &got)
	bizErr, ok := kerrors.FromBizStatusError(err)
	test.Assert(t, ok, err)
	test.Assert(t, bizErr.BizStatusCode() == 100, bizErr)
}

func TestEndpointServerSideKeepsRawEOF(t *testing.T) {
	ri := newTestRPCInfo()
	setter := ri.Invocation().(rpcinfo.InvocationSetter)
	setter.SetBizStatusErr(kerrors.NewBizStatusError(100, "biz error"))

	ctx := context.Background()
	client, server := NewPair(ctx, ri)

	// client closes its send direction; the server side must keep observing raw
	// io.EOF even when a biz error exists on RPCInfo.
	test.Assert(t, client.CloseSend(ctx) == nil)
	var got string
	err := server.RecvMsg(ctx, &got)
	test.Assert(t, err == io.EOF, err)
}

func TestEndpointHeaderBlocksUntilSent(t *testing.T) {
	ri := newTestRPCInfo()
	ctx := context.Background()
	client, server := NewPair(ctx, ri)

	done := make(chan streaming.Header, 1)
	go func() {
		hd, err := client.Header()
		test.Assert(t, err == nil, err)
		done <- hd
	}()

	// header should not be ready yet
	select {
	case <-done:
		t.Fatal("Header returned before SendHeader")
	case <-time.After(20 * time.Millisecond):
	}

	test.Assert(t, server.SetHeader(streaming.Header{"k": "v"}) == nil)
	test.Assert(t, server.SendHeader(nil) == nil)

	select {
	case hd := <-done:
		test.Assert(t, hd["k"] == "v", hd)
	case <-time.After(time.Second):
		t.Fatal("Header did not return after SendHeader")
	}
}

func TestEndpointHeaderCtxCancel(t *testing.T) {
	ri := newTestRPCInfo()
	ctx, cancel := context.WithCancel(context.Background())
	client, _ := NewPair(ctx, ri)

	cancel()
	_, err := client.Header()
	test.Assert(t, err == context.Canceled, err)
}

func TestEndpointFirstServerSendImplicitlySendsHeader(t *testing.T) {
	ri := newTestRPCInfo()
	ctx := context.Background()
	client, server := NewPair(ctx, ri)

	test.Assert(t, server.SendMsg(ctx, "data") == nil)

	// client.Header must unblock because the first server SendMsg sends header.
	hd, err := client.Header()
	test.Assert(t, err == nil, err)
	test.Assert(t, hd != nil)
}

func TestEndpointDoFinishOnce(t *testing.T) {
	ri := newTestRPCInfo()
	client, _ := NewPair(context.Background(), ri)

	var mu sync.Mutex
	count := 0
	client.RegisterCloseCallback(func(error) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.DoFinish(nil)
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	test.Assert(t, count == 1, count)
}

func TestEndpointGRPCAdapterForwards(t *testing.T) {
	ri := newTestRPCInfo()
	ctx := context.Background()
	client, server := NewPair(ctx, ri)

	gs := server.GetGRPCStream()
	test.Assert(t, gs != nil)
	// grpcAdapter is cached
	test.Assert(t, server.GetGRPCStream() == gs)

	// grpcAdapter.SendMsg forwards to the underlying endpoint's send pipe.
	test.Assert(t, gs.SendMsg("via-grpc") == nil)
	var got string
	test.Assert(t, client.RecvMsg(ctx, &got) == nil)
	test.Assert(t, got == "via-grpc", got)
}
