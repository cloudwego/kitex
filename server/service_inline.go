package server

import (
	"context"
	"net"
	"unsafe"

	"github.com/cloudwego/kitex/pkg/consts"
	"github.com/cloudwego/kitex/pkg/utils"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
)

var localAddr net.Addr

func init() {
	localAddr = utils.NewNetAddr("tcp", "127.0.0.1")
}

func constructServerCtxWithMetadata(cliCtx context.Context) (serverCtx context.Context) {
	serverCtx = context.Background()
	// metainfo
	// forward transmission
	kvs := make(map[string]string, 16)
	metainfo.SaveMetaInfoToMap(cliCtx, kvs)
	if len(kvs) > 0 {
		serverCtx = metainfo.SetMetaInfoFromMap(serverCtx, kvs)
	}
	serverCtx = metainfo.TransferForward(serverCtx)
	// reverse transmission, backward mark
	serverCtx = metainfo.WithBackwardValuesToSend(serverCtx)
	return serverCtx
}

func (s *server) constructServerRPCInfo(svrCtx context.Context, cr rpcinfo.RPCInfo) (newServerCtx context.Context, svrRPCInfo rpcinfo.RPCInfo) {
	rpcStats := rpcinfo.AsMutableRPCStats(rpcinfo.NewRPCStats())
	if s.opt.StatsLevel != nil {
		rpcStats.SetLevel(*s.opt.StatsLevel)
	}
	// Export read-only views to external users and keep a mapping for internal users.
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.EmptyEndpointInfo(),
		rpcinfo.FromBasicInfo(s.opt.Svr),
		rpcinfo.NewServerInvocation(),
		rpcinfo.AsMutableRPCConfig(s.opt.Configs).Clone().ImmutableView(),
		rpcStats.ImmutableView(),
	)
	rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(localAddr)
	svrCtx = rpcinfo.NewCtxWithRPCInfo(svrCtx, ri)

	// handle common rpcinfo
	method := cr.To().Method()
	if ink, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
		ink.SetMethodName(method)
		ink.SetServiceName(cr.Invocation().Extra(consts.SERVICE_INLINE_SERVICE_NAME).(string))
	}
	rpcinfo.AsMutableEndpointInfo(ri.To()).SetMethod(method)
	svrCtx = context.WithValue(svrCtx, consts.CtxKeyMethod, method)
	return svrCtx, ri
}

func (s *server) BuildServiceInlineInvokeChain() endpoint.Endpoint {
	svrTraceCtl := s.opt.TracerCtl
	if svrTraceCtl == nil {
		svrTraceCtl = &rpcinfo.TraceController{}
	}

	innerHandlerEp := s.invokeHandleEndpoint()
	mw := func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp interface{}) (err error) {
			serverCtx := constructServerCtxWithMetadata(ctx)
			defer func() {
				// backward key
				kvs := metainfo.AllBackwardValuesToSend(serverCtx)
				if len(kvs) > 0 {
					metainfo.SetBackwardValuesFromMap(ctx, kvs)
				}
			}()
			ptr := ctx.Value(consts.SERVICE_INLINE_RPCINFO_KEY).(unsafe.Pointer)
			cliRPCInfo := *(*rpcinfo.RPCInfo)(ptr)
			serverCtx, svrRPCInfo := s.constructServerRPCInfo(serverCtx, cliRPCInfo)
			defer func() {
				rpcinfo.PutRPCInfo(svrRPCInfo)
			}()

			// server trace
			serverCtx = svrTraceCtl.DoStart(serverCtx, svrRPCInfo)
			serverCtx = context.WithValue(serverCtx, consts.SERVICE_INLINE_DATA_KEY, ctx.Value(consts.SERVICE_INLINE_DATA_KEY))
			err = next(serverCtx, req, resp)

			bizErr := svrRPCInfo.Invocation().BizStatusErr()
			if bizErr != nil {
				if cliSetter, ok := cliRPCInfo.Invocation().(rpcinfo.InvocationSetter); ok {
					cliSetter.SetBizStatusErr(bizErr)
				}
			}
			// finish server trace
			// contextServiceInlineHandler may convert nil err to non nil err, so handle trace here
			svrTraceCtl.DoFinish(serverCtx, svrRPCInfo, err)

			return
		}
	}
	mws := []endpoint.Middleware{mw}
	mws = append(mws, s.mws...)
	return endpoint.Chain(mws...)(innerHandlerEp)
}
