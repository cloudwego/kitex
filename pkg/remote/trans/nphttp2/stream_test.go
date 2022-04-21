package nphttp2

import (
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"testing"
)

func TestStream(t *testing.T) {

	// init
	opt := MockServerOption()
	tr, err := MockServerTransport(mockAddr0)
	test.Assert(t, err == nil, err)
	s := grpc.MockNewServerSideStream()
	serverConn := newServerConn(tr, s)
	handler, err := NewSvrTransHandlerFactory().NewTransHandler(opt)
	ctx := MockCtxWithRPCInfo()

	// test NewStream()
	stream := NewStream(ctx, &serviceinfo.ServiceInfo{
		PackageName:     "",
		ServiceName:     "",
		HandlerType:     nil,
		Methods:         nil,
		PayloadCodec:    0,
		KiteXGenVersion: "",
		Extra:           nil,
		GenericMethod:   nil,
	}, serverConn, handler)

	// test Context()
	strCtx := stream.Context()
	test.Assert(t, strCtx == ctx)

	// test RecvMsg()
	msg := MockNewMessage().Data()
	grpc.MockStreamRecvHelloRequest(s)
	err = stream.RecvMsg(msg)
	test.Assert(t, err == nil, err)

	// test SendMsg()
	err = stream.SendMsg(msg)
	test.Assert(t, err == nil, err)

	// test Close()
	err = stream.Close()
	test.Assert(t, err == nil, err)

}
