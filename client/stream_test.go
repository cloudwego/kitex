package client

import (
	"context"
	"testing"

	"github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

var (
	mockCLiTransHandler = &mocks.MockCliTransHandler{}
	opts                = []Option{
		WithTransHandlerFactory(mocks.NewMockCliTransHandlerFactory(mockCLiTransHandler)),
		WithResolver(resolver404),
		WithDialer(dialer),
		WithDestService("destService"),
	}
	svcInfo   = mocks.ServiceInfo()
	req, resp = &streaming.Result{}, &streaming.Result{}
)

type mockRPCInfo struct{}

func (m mockRPCInfo) From() rpcinfo.EndpointInfo     { return nil }
func (m mockRPCInfo) To() rpcinfo.EndpointInfo       { return nil }
func (m mockRPCInfo) Invocation() rpcinfo.Invocation { return nil }
func (m mockRPCInfo) Config() rpcinfo.RPCConfig {
	return rpcinfo.NewRPCConfig()
}
func (m mockRPCInfo) Stats() rpcinfo.RPCStats { return nil }

func TestStream(t *testing.T) {
	ctx := context.Background()

	var err error
	kc := &kClient{
		opt:     client.NewOptions(opts),
		svcInfo: svcInfo,
	}

	_ = kc.init()

	err = kc.Stream(ctx, "mock_method", req, resp)
	test.Assert(t, err == nil, err)
}

func TestStreaming(t *testing.T) {
	var err error
	kc := &kClient{
		opt:     client.NewOptions(opts),
		svcInfo: svcInfo,
	}
	ctx = rpcinfo.NewCtxWithRPCInfo(context.Background(), mockRPCInfo{})
	stream := &stream{
		stream: nphttp2.NewStream(ctx, svcInfo, mocks.Conn{}, &mocks.MockCliTransHandler{}),
		kc:     kc,
	}

	// recv nil msg
	err = stream.RecvMsg(nil)
	test.Assert(t, err == nil, err)

	// send nil msg
	err = stream.SendMsg(nil)
	test.Assert(t, err == nil, err)

	// close
	err = stream.Close()
	test.Assert(t, err == nil, err)
}

func TestUninitClient(t *testing.T) {
	ctx := context.Background()

	kc := &kClient{
		opt:     client.NewOptions(opts),
		svcInfo: svcInfo,
	}

	test.PanicAt(t, func() {
		_ = kc.Stream(ctx, "mock_method", req, resp)
	}, func(err interface{}) bool {
		return err.(string) == "client not initialized"
	})
}

func TestClosedClient(t *testing.T) {
	ctx := context.Background()

	kc := &kClient{
		opt:     client.NewOptions(opts),
		svcInfo: svcInfo,
	}
	_ = kc.init()
	_ = kc.Close()

	test.PanicAt(t, func() {
		_ = kc.Stream(ctx, "mock_method", req, resp)
	}, func(err interface{}) bool {
		return err.(string) == "client is already closed"
	})
}

func TestNilCtx(t *testing.T) {
	var ctx context.Context

	kc := &kClient{
		opt:     client.NewOptions(opts),
		svcInfo: svcInfo,
	}
	_ = kc.init()

	test.PanicAt(t, func() {
		_ = kc.Stream(ctx, "mock_method", req, resp)
	}, func(err interface{}) bool {
		return err.(string) == "ctx is nil"
	})
}
