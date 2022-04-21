package nphttp2

import (
	"context"
	"errors"
	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/netpoll"
	"google.golang.org/protobuf/runtime/protoimpl"
	"net"
	"time"
)

var (
	mockAddr0 = "127.0.0.1:8000"
	mockAddr1 = "127.0.0.1:8001"
)

func MockConnPool() *connPool {
	connPool := NewConnPool("test", uint32(0), grpc.ConnectOptions{})
	return connPool
}

func MockConnOption() remote.ConnOption {
	return remote.ConnOption{Dialer: MockDialer(), ConnectTimeout: time.Second}
}

func MockServerOption() *remote.ServerOption {
	return &remote.ServerOption{
		SvcInfo:               nil,
		TransServerFactory:    nil,
		SvrHandlerFactory:     nil,
		Codec:                 nil,
		PayloadCodec:          nil,
		Address:               nil,
		ReusePort:             false,
		ExitWaitTime:          0,
		AcceptFailedDelayTime: 0,
		MaxConnectionIdleTime: 0,
		ReadWriteTimeout:      0,
		InitRPCInfoFunc:       nil,
		TracerCtl:             nil,
		GRPCCfg: &grpc.ServerConfig{
			MaxStreams:                 0,
			KeepaliveParams:            grpc.ServerKeepalive{},
			KeepaliveEnforcementPolicy: grpc.EnforcementPolicy{},
			InitialWindowSize:          0,
			InitialConnWindowSize:      0,
			MaxHeaderListSize:          nil,
		},
		Option: remote.Option{},
	}
}

func MockClientOption() *remote.ClientOption {
	return &remote.ClientOption{
		SvcInfo:           nil,
		CliHandlerFactory: nil,
		Codec:             nil,
		PayloadCodec:      nil,
		ConnPool:          MockConnPool(),
		Dialer:            MockDialer(),
		Option: remote.Option{
			Outbounds:             nil,
			Inbounds:              nil,
			StreamingMetaHandlers: nil,
		},
		EnableConnPoolReporter: false,
	}
}

func MockNpConn(address string) *MockNetpollConn {
	na := utils.NewNetAddr("tcp", address)
	return &MockNetpollConn{
		Conn: mocks.Conn{
			RemoteAddrFunc: func() net.Addr { return na },
			WriteFunc: func(b []byte) (n int, err error) {
				// mock write preface
				return len(b), nil
			},
		},
		WriterFunc: func() (r netpoll.Writer) {

			return &MockNetpollWriter{}
		},
		ReaderFunc: func() (r netpoll.Reader) {

			return &MockNetpollReader{}
		},
	}
}

func MockDialer() *remote.SynthesizedDialer {
	d := &remote.SynthesizedDialer{}
	d.DialFunc = func(network, address string, timeout time.Duration) (net.Conn, error) {
		connectCost := time.Millisecond * 10
		if timeout < connectCost {
			return nil, errors.New("connect timeout")
		}
		return MockNpConn(address), nil
	}
	return d
}

func MockCtxWithRPCInfo() context.Context {
	return rpcinfo.NewCtxWithRPCInfo(context.Background(), MockRPCInfo())
}

func MockRPCInfo() rpcinfo.RPCInfo {
	method := "method"
	c := rpcinfo.NewEndpointInfo("", method, nil, nil)
	endpointTags := map[string]string{}
	endpointTags[rpcinfo.HTTPURL] = "https://github.com/cloudwego/kitex"
	s := rpcinfo.NewEndpointInfo("", method, nil, endpointTags)
	ink := rpcinfo.NewInvocation("", method)
	cfg := rpcinfo.NewRPCConfig()
	ri := rpcinfo.NewRPCInfo(c, s, ink, cfg, rpcinfo.NewRPCStats())
	return ri
}

func MockNewMessage() *MockMessage {
	return &MockMessage{
		DataFunc: func() interface{} {
			return &HelloRequest{
				state:         protoimpl.MessageState{},
				sizeCache:     0,
				unknownFields: nil,
				Name:          "test",
			}
		},
	}
}

func MockServerTransport(addr string) (grpc.ServerTransport, error) {
	opt := MockServerOption()
	npConn := MockNpConn(addr)
	tr, err := grpc.NewServerTransport(context.Background(), npConn, opt.GRPCCfg)
	return tr, err
}
