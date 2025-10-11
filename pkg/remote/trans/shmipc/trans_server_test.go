//go:build linux
// +build linux

package shmipc

import (
	"context"
	"net"

	mocksremote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

func initUDSServer() (transSvr remote.TransServer, errCh chan error) {
	errCh = make(chan error, 1)
	tf := netpoll.NewTransServerFactory()

	opt := initServerOption()
	hdlr, _ := NewSvrTransHandlerFactory().NewTransHandler(opt)
	transPl := remote.NewTransPipeline(hdlr)
	hdlr.(remote.InvokeHandleFuncSetter).SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
		return nil
	})
	transSvr = tf.NewTransServer(opt, transPl)
	ln, err := transSvr.CreateListener(uds)
	if err != nil {
		errCh <- err
	}

	go func() { errCh <- transSvr.BootstrapServer(ln) }()
	return transSvr, errCh
}

func initShmIPCServer() (transSvr remote.TransServer, errCh chan error) {
	errCh = make(chan error, 1)
	shmUDS, _ := net.ResolveUnixAddr("unix", shmUDSAddrStr)
	tf := NewTransServerFactory(shmUDS)

	opt := initServerOption()
	hdlr, _ := NewSvrTransHandlerFactory().NewTransHandler(opt)
	transPl := remote.NewTransPipeline(hdlr)
	hdlr.(remote.InvokeHandleFuncSetter).SetInvokeHandleFunc(func(ctx context.Context, req, resp interface{}) (err error) {
		return nil
	})
	transSvr = tf.NewTransServer(opt, transPl)
	ln, err := transSvr.CreateListener(shmUDS)
	if err != nil {
		errCh <- err
	}

	go func() { errCh <- transSvr.BootstrapServer(ln) }()
	return transSvr, errCh
}

func initServerOption() *remote.ServerOption {
	return &remote.ServerOption{
		InitOrResetRPCInfoFunc: func(ri rpcinfo.RPCInfo, rAddr net.Addr) rpcinfo.RPCInfo {
			stats := rpcinfo.AsMutableRPCStats(rpcinfo.NewRPCStats())
			stats.SetLevel(2)

			// Export read-only views to external users and keep a mapping for internal users.
			ri = rpcinfo.NewRPCInfo(
				rpcinfo.EmptyEndpointInfo(),
				rpcinfo.FromBasicInfo(&rpcinfo.EndpointBasicInfo{}),
				rpcinfo.NewServerInvocation(),
				rpcinfo.NewRPCConfig(),
				stats.ImmutableView(),
			)
			rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(rAddr)
			return ri
		},
		Codec:       &MockCodec{EncodeFunc: mockEncode, DecodeFunc: mockDecode},
		SvcSearcher: mocksremote.NewDefaultSvcSearcher(),
	}
}
