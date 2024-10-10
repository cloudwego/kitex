package ttstream_test

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/client/streamxclient"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/streamx/provider/ttstream"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/server/streamxserver"
)

const (
	normalErr int32 = iota + 1
	bizErr
)

var (
	testCode  = int32(10001)
	testMsg   = "biz testMsg"
	testExtra = map[string]string{
		"testKey": "testVal",
	}
	normalErrMsg = "normal error"
)

func assertNormalErr(t *testing.T, err error) {
	ex, ok := err.(*thrift.ApplicationException)
	test.Assert(t, ok, err)
	test.Assert(t, ex.TypeID() == remote.InternalError, ex.TypeID())
	test.Assert(t, ex.Msg() == "biz error: "+normalErrMsg, ex.Msg())
}

func assertBizErr(t *testing.T, err error) {
	bizIntf, ok := kerrors.FromBizStatusError(err)
	test.Assert(t, ok)
	test.Assert(t, bizIntf.BizStatusCode() == testCode, bizIntf.BizStatusCode())
	test.Assert(t, bizIntf.BizMessage() == testMsg, bizIntf.BizMessage())
	test.DeepEqual(t, bizIntf.BizExtra(), testExtra)
}

func TestTTHeaderStreamingErrorHandling(t *testing.T) {
	klog.SetLevel(klog.LevelDebug)
	var addr = test.GetLocalAddress()
	ln, err := netpoll.CreateListener("tcp", addr)
	test.Assert(t, err == nil, err)
	defer ln.Close()

	svr := server.NewServer(server.WithListener(ln), server.WithExitWaitTime(time.Millisecond*10))
	sp, err := ttstream.NewServerProvider(streamingServiceInfo)
	test.Assert(t, err == nil, err)
	err = svr.RegisterService(
		streamingServiceInfo,
		new(streamingService),
		streamxserver.WithProvider(sp),
	)
	test.Assert(t, err == nil, err)
	go func() {
		err := svr.Run()
		test.Assert(t, err == nil, err)
	}()
	defer svr.Stop()
	test.WaitServerStart(addr)

	streamClient, err := NewStreamingClient(
		"kitex.service.streaming",
		streamxclient.WithHostPorts(addr),
	)
	test.Assert(t, err == nil, err)

	t.Logf("=== UnaryWithErr normalErr ===")
	req := new(Request)
	req.Type = normalErr
	res, err := streamClient.UnaryWithErr(context.Background(), req)
	test.Assert(t, res == nil, res)
	test.Assert(t, err != nil, err)
	assertNormalErr(t, err)

	t.Logf("=== UnaryWithErr bizErr ===")
	req = new(Request)
	req.Type = bizErr
	res, err = streamClient.UnaryWithErr(context.Background(), req)
	test.Assert(t, res == nil, res)
	test.Assert(t, err != nil, err)
	assertBizErr(t, err)

	t.Logf("=== ClientStreamWithErr normalErr ===")
	ctx := context.Background()
	cliStream, err := streamClient.ClientStreamWithErr(ctx)
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
	ctx = context.Background()
	cliStream, err = streamClient.ClientStreamWithErr(ctx)
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
	ctx = context.Background()
	req = new(Request)
	req.Type = normalErr
	svrStream, err := streamClient.ServerStreamWithErr(ctx, req)
	test.Assert(t, err == nil, err)
	test.Assert(t, svrStream != nil, svrStream)
	res, err = svrStream.Recv(ctx)
	test.Assert(t, res == nil, res)
	test.Assert(t, err != nil, err)
	assertNormalErr(t, err)

	t.Logf("=== ServerStreamWithErr bizErr ===")
	ctx = context.Background()
	req = new(Request)
	req.Type = bizErr
	svrStream, err = streamClient.ServerStreamWithErr(ctx, req)
	test.Assert(t, err == nil, err)
	test.Assert(t, svrStream != nil, svrStream)
	res, err = svrStream.Recv(ctx)
	test.Assert(t, res == nil, res)
	test.Assert(t, err != nil, err)
	assertBizErr(t, err)

	t.Logf("=== BidiStreamWithErr normalErr ===")
	ctx = context.Background()
	bidiStream, err := streamClient.BidiStreamWithErr(ctx)
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
	ctx = context.Background()
	bidiStream, err = streamClient.BidiStreamWithErr(ctx)
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
}
