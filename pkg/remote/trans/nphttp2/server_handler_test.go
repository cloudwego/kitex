package nphttp2

import (
	"context"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"testing"
	"time"
)

func TestServerHandler(t *testing.T) {

	// init
	opt := MockServerOption()
	msg := MockNewMessage()
	npConn := MockNpConn(mockAddr0)
	tr, err := MockServerTransport(mockAddr0)
	test.Assert(t, err == nil, err)
	s := grpc.MockNewServerSideStream()
	serverConn := newServerConn(tr, s)
	// test NewTransHandler()
	handler, err := NewSvrTransHandlerFactory().NewTransHandler(opt)
	test.Assert(t, err == nil, err)

	// test Read()
	// mock grpc encoded msg data into stream recv buffer
	grpc.MockStreamRecvHelloRequest(s)
	err = handler.Read(context.Background(), serverConn, msg)
	test.Assert(t, err == nil, err)

	// test Write()
	err = handler.Write(context.Background(), serverConn, msg)
	test.Assert(t, err == nil, err)

	//模拟handler整个的运转过程,触发一次metaHeader，才能覆盖大部分的代码
	// 这里上层对tr没有把控。。。导致很难写单测。。。。。
	//初始化（需要被迫停止，不然一直read循环）
	go handler.OnRead(context.Background(), npConn)

	defer func() {
		recover()
	}()

	time.Sleep(time.Second)
	panic("test stop!")

}
