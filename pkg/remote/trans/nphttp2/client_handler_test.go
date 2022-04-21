package nphttp2

import (
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"testing"
	"time"
)

func TestClientHandler(t *testing.T) {

	opt := MockClientOption()
	ctx := MockCtxWithRPCInfo()
	msg := MockNewMessage()
	conn, err := opt.ConnPool.Get(ctx, "tcp", mockAddr0, remote.ConnOption{Dialer: opt.Dialer, ConnectTimeout: time.Second})

	// test NewTransHandler()
	cliTransHandler, err := NewCliTransHandlerFactory().NewTransHandler(opt)
	test.Assert(t, err == nil, err)

	// test Read()
	grpc.MockStreamRecv(conn.(*clientConn).s)
	err = cliTransHandler.Read(ctx, conn, msg)
	test.Assert(t, err != nil, err)

	// test Write()
	err = cliTransHandler.Write(ctx, conn, msg)
	test.Assert(t, err == nil, err)

}

//todo too many meaningless
func TestClientHandlerOnAction(t *testing.T) {

	// init
	opt := MockClientOption()
	ctx := MockCtxWithRPCInfo()
	conn, err := opt.ConnPool.Get(ctx, "tcp", mockAddr0, remote.ConnOption{Dialer: opt.Dialer, ConnectTimeout: time.Second})
	cliTransHandler, err := newCliTransHandler(opt)
	test.Assert(t, err == nil, err)

	// test OnConnection()
	cliCtx, err := cliTransHandler.OnConnect(ctx)
	test.Assert(t, err == nil, err)
	test.Assert(t, cliCtx == ctx)

	// test OnMessage()
	cliCtx, err = cliTransHandler.OnMessage(ctx, new(MockMessage), new(MockMessage))
	test.Assert(t, err == nil, err)
	test.Assert(t, cliCtx == ctx)

	// test OnRead()
	grpc.DontWaitHeader(conn.(*clientConn).s)
	cliTransHandler.OnRead(ctx, conn)

	// test SetPipeline()
	cliTransHandler.SetPipeline(nil)

	defer func() {
		err1 := recover()
		test.Assert(t, err1 != nil)
	}()

	defer func() {
		err1 := recover()
		test.Assert(t, err1 != nil)
		// test OnInactive()
		cliTransHandler.OnInactive(ctx, conn)
	}()
	// test OnError()
	cliTransHandler.OnError(ctx, nil, conn)

}
