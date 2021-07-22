/*
 * Copyright 2021 CloudWeGo Authors
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

package client

import (
	"context"
	"errors"
	"runtime"
	"strconv"

	"github.com/bytedance/gopkg/cloud/metainfo"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/internal/client"
	"github.com/cloudwego/kitex/pkg/acl"
	"github.com/cloudwego/kitex/pkg/consts"
	"github.com/cloudwego/kitex/pkg/diagnosis"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/bound"
	"github.com/cloudwego/kitex/pkg/remote/remotecli"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/transport"
)

// Client is the core interface abstraction of kitex client.
// It is designed for generated codes and should not be used directly.
// Parameter method specifies the method of a RPC call.
// Request is a packing of request parameters in the actual method defined in IDL, consist of zero, one
// or multiple arguments. So is response to the actual result type.
// Response may be nil to address oneway calls.
type Client interface {
	Call(ctx context.Context, method string, request interface{}, response interface{}) error
}

type kClient struct {
	svcInfo *serviceinfo.ServiceInfo
	mws     []endpoint.Middleware
	eps     endpoint.Endpoint
	sEps    endpoint.Endpoint
	opt     *client.Options

	inited bool
	closed bool
}

// NewClient creates a kitex.Client with the given ServiceInfo, it is from generated code.
func NewClient(svcInfo *serviceinfo.ServiceInfo, opts ...Option) (Client, error) {
	if svcInfo == nil {
		return nil, errors.New("NewClient: no service info")
	}
	kc := &kClient{
		svcInfo: svcInfo,
		opt:     client.NewOptions(opts),
	}
	if err := kc.init(); err != nil {
		return nil, err
	}
	// like os.File, if kc is garbage-collected, but Close is not called, call Close.
	runtime.SetFinalizer(kc, func(c *kClient) {
		c.Close()
	})
	return kc, nil
}

func (kc *kClient) init() error {
	initTransportProtocol(kc.svcInfo, kc.opt.Configs)

	ctx := fillContext(kc.opt)
	if err := kc.checkOptions(); err != nil {
		return err
	}
	if err := kc.initRetryer(); err != nil {
		return err
	}
	kc.initMiddlewares(ctx)
	kc.inited = true
	if kc.opt.DebugService != nil {
		kc.opt.DebugService.RegisterProbeFunc(diagnosis.DestServiceKey, diagnosis.WrapAsProbeFunc(kc.opt.Svr.ServiceName))
		kc.opt.DebugService.RegisterProbeFunc(diagnosis.OptionsKey, diagnosis.WrapAsProbeFunc(kc.opt.DebugInfo))
		kc.opt.DebugService.RegisterProbeFunc(diagnosis.ChangeEventsKey, kc.opt.Events.Dump)
		kc.opt.DebugService.RegisterProbeFunc(diagnosis.ServiceInfoKey, diagnosis.WrapAsProbeFunc(kc.svcInfo))
	}
	kc.richRemoteOption(ctx)
	return kc.buildInvokeChain()
}

func (kc *kClient) initRetryer() error {
	if kc.opt.RetryContainer == nil {
		kc.opt.RetryContainer = retry.NewRetryContainer()
	}
	return kc.opt.RetryContainer.Init(kc.opt.RetryPolicy, kc.opt.Logger)
}

func initTransportProtocol(svcInfo *serviceinfo.ServiceInfo, cfg rpcinfo.RPCConfig) {
	if svcInfo.PayloadCodec == serviceinfo.Protobuf && cfg.TransportProtocol() != transport.GRPC {
		// pb use ttheader framed by default
		rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.TTHeaderFramed)
	}
}

func fillContext(opt *client.Options) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, endpoint.CtxEventBusKey, opt.Bus)
	ctx = context.WithValue(ctx, endpoint.CtxEventQueueKey, opt.Events)
	ctx = context.WithValue(ctx, endpoint.CtxLoggerKey, opt.Logger)
	return ctx
}

func (kc *kClient) checkOptions() (err error) {
	if kc.opt.Svr.ServiceName == "" {
		return errors.New("service name is required")
	}
	if kc.opt.Proxy != nil {
		cfg := proxy.Config{
			ServiceName:  kc.opt.Svr.ServiceName,
			Resolver:     kc.opt.Resolver,
			Balancer:     kc.opt.Balancer,
			Pool:         kc.opt.RemoteOpt.ConnPool,
			FixedTargets: kc.opt.Targets,
		}
		if err = kc.opt.Proxy.Configure(&cfg); err != nil {
			return err
		}
		updateOptWithProxyCfg(cfg, kc.opt)
	}
	if kc.opt.Logger == nil {
		return errors.New("logger need to be initialized")
	}
	return nil
}

func (kc *kClient) initMiddlewares(ctx context.Context) {
	kc.mws = richMWsWithBuilder(ctx, kc.opt.MWBs, kc)
	// add new middlewares
	kc.mws = append(kc.mws, acl.NewACLMiddleware(kc.opt.ACLRules))
	if kc.opt.Proxy == nil {
		kc.mws = append(kc.mws, newResolveMWBuilder(kc.opt)(ctx))
		kc.mws = richMWsWithBuilder(ctx, kc.opt.IMWBs, kc)
	} else {
		if kc.opt.Resolver != nil { // customized service discovery
			kc.mws = append(kc.mws, newResolveMWBuilder(kc.opt)(ctx))
		}
		kc.mws = append(kc.mws, newProxyMW(kc.opt.Proxy))
	}
	kc.mws = append(kc.mws, newIOErrorHandleMW(kc.opt.ErrHandle))
}

func richMWsWithBuilder(ctx context.Context, mwBs []endpoint.MiddlewareBuilder, kc *kClient) []endpoint.Middleware {
	for i := range mwBs {
		kc.mws = append(kc.mws, mwBs[i](ctx))
	}
	return kc.mws
}

// initRPCInfo initializes the RPCInfo structure and attaches it to context.
func (kc *kClient) initRPCInfo(ctx context.Context, method string) (context.Context, rpcinfo.RPCInfo) {
	cfg := rpcinfo.AsMutableRPCConfig(kc.opt.Configs).Clone()
	rmt := remoteinfo.NewRemoteInfo(kc.opt.Svr, method)
	ctx = kc.applyCallOptions(ctx, cfg.ImmutableView(), rmt)
	rpcStats := rpcinfo.AsMutableRPCStats(rpcinfo.NewRPCStats())
	if kc.opt.StatsLevel != nil {
		rpcStats.SetLevel(*kc.opt.StatsLevel)
	}

	// Export read-only views to external users.
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.FromBasicInfo(kc.opt.Cli),
		rmt.ImmutableView(),
		rpcinfo.NewInvocation(kc.svcInfo.ServiceName, method, kc.svcInfo.GetPackageName()),
		cfg.ImmutableView(),
		rpcStats.ImmutableView(),
	)

	if fromMethod := ctx.Value(consts.CtxKeyMethod); fromMethod != nil {
		rpcinfo.AsMutableEndpointInfo(ri.From()).SetMethod(fromMethod.(string))
	}

	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	return ctx, ri
}

func (kc *kClient) applyCallOptions(ctx context.Context, cfg rpcinfo.RPCConfig, svr remoteinfo.RemoteInfo) context.Context {
	cos := CallOptionsFromCtx(ctx)
	if len(cos) > 0 {
		info := callopt.Apply(cos, cfg, svr, kc.opt.Locks, kc.opt.HTTPResolver)
		ctx = context.WithValue(ctx, ctxCallOptionInfoKey, info)
	} else {
		kc.opt.Locks.ApplyLocks(cfg, svr)
	}
	return ctx
}

// Call implements the Client interface .
func (kc *kClient) Call(ctx context.Context, method string, request interface{}, response interface{}) error {
	if !kc.inited {
		panic("client not initialized")
	}
	if kc.closed {
		panic("client is already closed")
	}
	if ctx == nil {
		panic("ctx is nil")
	}
	var ri rpcinfo.RPCInfo
	ctx, ri = kc.initRPCInfo(ctx, method)

	callTimes := 0
	var prevRI rpcinfo.RPCInfo
	ctx = kc.opt.TracerCtl.DoStart(ctx, ri, kc.opt.Logger)
	recycleRI, err := kc.opt.RetryContainer.WithRetryIfNeeded(ctx, func(ctx context.Context, r retry.Retryer) (rpcinfo.RPCInfo, error) {
		callTimes++
		retryCtx := ctx
		cRI := ri
		if callTimes > 1 {
			retryCtx, cRI = kc.initRPCInfo(ctx, method)
			retryCtx = metainfo.WithPersistentValue(retryCtx, retry.TransitKey, strconv.Itoa(callTimes-1))
			if prevRI == nil {
				prevRI = ri
			}
			r.Prepare(retryCtx, prevRI, cRI)
			prevRI = cRI
		}
		err := kc.eps(retryCtx, request, response)
		return cRI, err
	}, ri, request)
	kc.opt.TracerCtl.DoFinish(ctx, ri, err, kc.opt.Logger)
	if recycleRI {
		// why need check recycleRI to decide if recycle RPCInfo?
		// 1. no retry, rpc timeout happen will cause panic when response return
		// 2. retry success, will cause panic when first call return
		// 3. backup request may cause panic, cannot recycle first RPCInfo
		// RPCInfo will be recycled after rpc is finished,
		// holding RPCInfo in a new goroutine is forbidden.
		rpcinfo.PutRPCInfo(ri)
	}

	return err
}

func (kc *kClient) richRemoteOption(ctx context.Context) {
	kc.opt.RemoteOpt.SvcInfo = kc.svcInfo
	kc.opt.RemoteOpt.Logger = kc.opt.Logger

	kc.addBoundHandlers(kc.opt.RemoteOpt)
}

func (kc *kClient) addBoundHandlers(opt *remote.ClientOption) {
	// for client trans info handler
	if len(kc.opt.MetaHandlers) > 0 {
		// TODO in stream situations, meta is only assembled when the stream creates
		// metaHandler needs to be called separately.
		// (newClientStreamer: call WriteMeta before remotecli.NewClient)
		transInfoHdlr := bound.NewTransMetaHandler(kc.opt.MetaHandlers)
		doAddBoundHandler(transInfoHdlr, opt)
	}
}

func (kc *kClient) buildInvokeChain() error {
	innerHandlerEp, err := kc.invokeHandleEndpoint()
	if err != nil {
		return err
	}
	kc.eps = endpoint.Chain(kc.mws...)(innerHandlerEp)

	innerStreamingEp, err := kc.invokeStreamingEndpoint()
	if err != nil {
		return err
	}
	kc.sEps = endpoint.Chain(kc.mws...)(innerStreamingEp)
	return nil
}

func (kc *kClient) invokeHandleEndpoint() (endpoint.Endpoint, error) {
	transPipl, err := newCliTransHandler(kc.opt.RemoteOpt)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, req, resp interface{}) (err error) {
		var sendMsg remote.Message
		var recvMsg remote.Message
		defer func() {
			remote.RecycleMessage(sendMsg)
			// Notice, recycle and decode may race if decode exec in another goroutine.
			// No race now, it is ok to recycle. Or recvMsg recycle depend on recv err
			remote.RecycleMessage(recvMsg)
		}()
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()

		cli, err := remotecli.NewClient(ctx, ri, transPipl, kc.opt.RemoteOpt)
		if err != nil {
			return
		}
		config := ri.Config()
		if kc.svcInfo.MethodInfo(methodName).OneWay() {
			sendMsg = remote.NewMessage(req, kc.svcInfo, ri, remote.Oneway, remote.Client)
		} else {
			sendMsg = remote.NewMessage(req, kc.svcInfo, ri, remote.Call, remote.Client)
		}
		sendMsg.SetProtocolInfo(remote.NewProtocolInfo(config.TransportProtocol(), kc.svcInfo.PayloadCodec))

		if err = cli.Send(ctx, ri, sendMsg); err != nil {
			return
		}
		if resp == nil || kc.svcInfo.MethodInfo(methodName).OneWay() {
			cli.Recv(ctx, ri, nil)
			return nil
		}

		recvMsg = remote.NewMessage(resp, kc.opt.RemoteOpt.SvcInfo, ri, remote.Reply, remote.Client)
		err = cli.Recv(ctx, ri, recvMsg)
		return err
	}, nil
}

// Close is not concurrency safe.
func (kc *kClient) Close() error {
	if kc.closed {
		panic("client is already closed")
	}
	kc.closed = true
	var errs utils.ErrChain
	for _, cb := range kc.opt.CloseCallbacks {
		if err := cb(); err != nil {
			errs.Append(err)
		}
	}
	runtime.SetFinalizer(kc, nil)
	if errs.HasError() {
		return errs
	}
	return nil
}

func doAddBoundHandler(h remote.BoundHandler, opt *remote.ClientOption) {
	add := false
	if ih, ok := h.(remote.InboundHandler); ok {
		opt.Inbounds = append(opt.Inbounds, ih)
		add = true
	}
	if oh, ok := h.(remote.OutboundHandler); ok {
		opt.Outbounds = append(opt.Outbounds, oh)
		add = true
	}
	if !add {
		panic("invalid BoundHandler: must implement InboundHandler or OuboundHandler")
	}
}

func newCliTransHandler(opt *remote.ClientOption) (remote.ClientTransHandler, error) {
	handler, err := opt.CliHandlerFactory.NewTransHandler(opt)
	if err != nil {
		return nil, err
	}
	transPl := remote.NewTransPipeline(handler)
	for _, ib := range opt.Inbounds {
		transPl.AddInboundHandler(ib)
	}
	for _, ob := range opt.Outbounds {
		transPl.AddOutboundHandler(ob)
	}
	return transPl, nil
}

func updateOptWithProxyCfg(cfg proxy.Config, opt *client.Options) {
	opt.Resolver = cfg.Resolver
	opt.Balancer = cfg.Balancer
	opt.RemoteOpt.ConnPool = cfg.Pool
	opt.Targets = cfg.FixedTargets
}
