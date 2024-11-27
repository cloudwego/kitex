package test

import (
	"context"
	"fmt"
	"github.com/tidwall/gjson"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/kitex/client/genericclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic"
)

func TestGenericStreamingWithStreamXServer(t *testing.T) {
	addr, svr, err := initStreamxTestServer(new(TestServiceImpl))
	test.Assert(t, err == nil, err)
	defer svr.Stop()

	idl := "./idl/example.thrift"
	g, err := getJsonThriftGeneric(idl, false)
	test.Assert(t, err == nil)
	gCli := newGenericStreamingClient(g, addr)

	// with dynamicgo
	dg, err := getJsonThriftGeneric(idl, true)
	test.Assert(t, err == nil)
	dgCli := newGenericStreamingClient(dg, addr)
	ctx := context.Background()

	// PingPong
	testPingPong(t, ctx, gCli)
	testPingPong(t, ctx, dgCli)

	// Client Streaming
	testClientStreaming(t, ctx, gCli)
	testClientStreaming(t, ctx, dgCli)

	// Server Streaming
	testServerStreaming(t, ctx, gCli)
	testServerStreaming(t, ctx, dgCli)

	// Bidi Streaming
	testBidiStreaming(t, ctx, gCli)
	testBidiStreaming(t, ctx, dgCli)
}

func testPingPong(t *testing.T, ctx context.Context, cli genericclient.StreamClient[any]) {
	resp, err := cli.GenericCall(ctx, "PingPong", `{"Message": "PingPong message"}`)
	test.Assert(t, err == nil, err)
	strResp, ok := resp.(string)
	test.Assert(t, ok)
	test.Assert(t, reflect.DeepEqual(gjson.Get(strResp, "Message").String(), "PingPong message"))
	test.Assert(t, reflect.DeepEqual(gjson.Get(strResp, "Type").String(), "0"))
}

func testClientStreaming(t *testing.T, ctx context.Context, cli genericclient.StreamClient[any]) {
	ctx, streamCli, err := genericclient.NewClientV2Streaming(ctx, cli, "ClientStream")
	test.Assert(t, err == nil, err)
	for i := 0; i < 3; i++ {
		req := fmt.Sprintf(`{"Message": "grpc client streaming generic %dth request"}`, i)
		err = streamCli.Send(ctx, req)
		test.Assert(t, err == nil, err)
	}
	resp, err := streamCli.CloseAndRecv(ctx)
	test.Assert(t, err == nil, err)
	strResp, ok := resp.(string)
	test.Assert(t, ok)
	fmt.Printf("clientStreaming message received: %v\n", strResp)
	test.Assert(t, reflect.DeepEqual(gjson.Get(strResp, "Message").String(), "grpc client streaming generic 2th request"))
	test.Assert(t, reflect.DeepEqual(gjson.Get(strResp, "Type").String(), "0"))
}

func testServerStreaming(t *testing.T, ctx context.Context, cli genericclient.StreamClient[any]) {
	ctx, streamCli, err := genericclient.NewServerV2Streaming(ctx, cli, "ServerStream", `{"Message": "grpc server streaming generic request"}`)
	test.Assert(t, err == nil, err)
	count := int64(0)
	for {
		resp, err := streamCli.Recv(ctx)
		if err != nil {
			test.Assert(t, err == io.EOF, err)
			fmt.Println("serverStreaming message receive done")
			break
		} else {
			strResp, ok := resp.(string)
			test.Assert(t, ok)
			fmt.Printf("serverStreaming message received: %s\n", strResp)
			test.Assert(t, reflect.DeepEqual(gjson.Get(strResp, "Message").String(), "grpc server streaming generic request"))
			test.Assert(t, reflect.DeepEqual(gjson.Get(strResp, "Type").Int(), count))
			count++
		}
	}
}

func testBidiStreaming(t *testing.T, ctx context.Context, cli genericclient.StreamClient[any]) {
	ctx, streamCli, err := genericclient.NewBidirectionalV2Streaming(ctx, cli, "BidiStream")
	test.Assert(t, err == nil, err)
	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			req := fmt.Sprintf(`{"Message": "grpc bidirectional streaming generic %dth request"}`, i)
			err = streamCli.Send(ctx, req)
			test.Assert(t, err == nil)
			fmt.Printf("Send: req = %s\n", req)
		}
		err = streamCli.CloseSend(ctx)
		test.Assert(t, err == nil)
	}()

	go func() {
		defer wg.Done()
		for {
			resp, err := streamCli.Recv(ctx)
			if err != nil {
				test.Assert(t, err == io.EOF, err)
				fmt.Println("bidirectionalStreaming message receive done")
				break
			} else {
				strResp, ok := resp.(string)
				test.Assert(t, ok)
				fmt.Printf("bidirectionalStreaming message received: %s\n", strResp)
				test.Assert(t, strings.Contains(gjson.Get(strResp, "Message").String(), "grpc bidirectional streaming generic"))
			}
		}
	}()
	wg.Wait()
}

func getJsonThriftGeneric(idl string, enableDynamicgo bool) (generic.Generic, error) {
	var p generic.DescriptorProvider
	var err error
	if enableDynamicgo {
		p, err = generic.NewThriftFileProviderWithDynamicGo(idl)
		if err != nil {
			return nil, err
		}
	} else {
		p, err = generic.NewThriftFileProvider(idl)
		if err != nil {
			return nil, err
		}
	}
	g, err := generic.JSONThriftGeneric(p)
	if err != nil {
		return nil, err
	}
	return g, nil
}
