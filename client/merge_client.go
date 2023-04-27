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
	"net"
	"runtime/debug"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/internal/client"
	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/consts"
	"github.com/cloudwego/kitex/pkg/diagnosis"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/remote/bound"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/rpctimeout"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

var localAddr net.Addr

func init() {
	localAddr = utils.NewNetAddr("tcp", "127.0.0.1")
}

type MergeMetaHandler func(ctx, svrCtx context.Context) context.Context

type mergeClient struct {
	svcInfo *serviceinfo.ServiceInfo
	mws     []endpoint.Middleware
	eps     endpoint.Endpoint
	opt     *client.Options

	inited bool
	closed bool

	turboMode bool

	// server info
	serverEps endpoint.Endpoint
	serverOpt *internal_server.Options

	mergeMetaHandler MergeMetaHandler
}

type ServerMethod interface {
	Endpoints() endpoint.Endpoint
	Option() *internal_server.Options
	GetServiceInfo() *serviceinfo.ServiceInfo
}

// NewClient creates a kitex.Client with the given ServiceInfo, it is from generated code.
func NewMergeClient(svcInfo *serviceinfo.ServiceInfo, s ServerMethod, opts ...Option) (Client, error) {
	if svcInfo == nil {
		return nil, errors.New("NewClient: no service info")
	}
	kc := &mergeClient{}
	kc.svcInfo = svcInfo
	kc.opt = client.NewOptions(opts)
	kc.serverEps = s.Endpoints()
	kc.serverOpt = s.Option()
	kc.serverOpt.RemoteOpt.SvcInfo = s.GetServiceInfo()
	if err := kc.init(); err != nil {
		_ = kc.Close()
		return nil, err
	}
	return kc, nil
}

func (kc *mergeClient) SetMergeMetaHandler(fn MergeMetaHandler) {
	kc.mergeMetaHandler = fn
}

func (kc *mergeClient) init() (err error) {
	// initTransportProtocol(kc.svcInfo, kc.opt.Configs)
	if err = kc.checkOptions(); err != nil {
		return err
	}
	ctx := kc.initContext()
	kc.initMiddlewares(ctx)
	kc.initDebugService()
	kc.richRemoteOption()
	if err = kc.buildInvokeChain(); err != nil {
		return err
	}
	kc.inited = true
	return nil
}

func (kc *mergeClient) checkOptions() (err error) {
	if kc.opt.Svr.ServiceName == "" {
		return errors.New("service name is required")
	}
	return nil
}

func (kc *mergeClient) initContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, endpoint.CtxEventBusKey, kc.opt.Bus)
	ctx = context.WithValue(ctx, endpoint.CtxEventQueueKey, kc.opt.Events)
	ctx = context.WithValue(ctx, rpctimeout.TimeoutAdjustKey, &kc.opt.ExtraTimeout)
	if chr, ok := kc.opt.Proxy.(proxy.ContextHandler); ok {
		ctx = chr.HandleContext(ctx)
	}
	return ctx
}

func (kc *mergeClient) initMiddlewares(ctx context.Context) {
	builderMWs := richMWsWithBuilder(ctx, kc.opt.MWBs)
	kc.mws = append(kc.mws, contextMW)
	kc.mws = append(kc.mws, builderMWs...)
	kc.mws = append(kc.mws, richMWsWithBuilder(ctx, kc.opt.IMWBs)...)
}

// initRPCInfo initializes the RPCInfo structure and attaches it to context.
func (kc *mergeClient) initRPCInfo(ctx context.Context, method string) (context.Context, rpcinfo.RPCInfo, *callopt.CallOptions) {
	cfg := rpcinfo.AsMutableRPCConfig(kc.opt.Configs).Clone()
	rmt := remoteinfo.NewRemoteInfo(kc.opt.Svr, method)
	var callOpts *callopt.CallOptions
	ctx, callOpts = kc.applyCallOptions(ctx, cfg, rmt)
	rpcStats := rpcinfo.AsMutableRPCStats(rpcinfo.NewRPCStats())
	if kc.opt.StatsLevel != nil {
		rpcStats.SetLevel(*kc.opt.StatsLevel)
	}

	mi := kc.svcInfo.MethodInfo(method)
	if mi != nil && mi.OneWay() {
		cfg.SetInteractionMode(rpcinfo.Oneway)
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

	if p := kc.opt.Timeouts; p != nil {
		if c := p.Timeouts(ri); c != nil {
			_ = cfg.SetRPCTimeout(c.RPCTimeout())
			_ = cfg.SetConnectTimeout(c.ConnectTimeout())
			_ = cfg.SetReadWriteTimeout(c.ReadWriteTimeout())
		}
	}

	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	return ctx, ri, callOpts
}

func (kc *mergeClient) applyCallOptions(ctx context.Context, cfg rpcinfo.MutableRPCConfig, svr remoteinfo.RemoteInfo) (context.Context, *callopt.CallOptions) {
	cos := CallOptionsFromCtx(ctx)
	if len(cos) > 0 {
		info, callOpts := callopt.Apply(cos, cfg, svr, kc.opt.Locks, kc.opt.HTTPResolver)
		ctx = context.WithValue(ctx, ctxCallOptionInfoKey, info)
		return ctx, callOpts
	}
	kc.opt.Locks.ApplyLocks(cfg, svr)
	return ctx, nil
}

// Call implements the Client interface .
func (kc *mergeClient) Call(ctx context.Context, method string, request, response interface{}) error {
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
	var callOpts *callopt.CallOptions
	ctx, ri, callOpts = kc.initRPCInfo(ctx, method)

	ctx = kc.opt.TracerCtl.DoStart(ctx, ri)

	err := kc.eps(ctx, request, response)

	kc.opt.TracerCtl.DoFinish(ctx, ri, err)
	callOpts.Recycle()

	// why need check recycleRI to decide if recycle RPCInfo?
	// 1. no retry, rpc timeout happen will cause panic when response return
	// 2. retry success, will cause panic when first call return
	// 3. backup request may cause panic, cannot recycle first RPCInfo
	// RPCInfo will be recycled after rpc is finished,
	// holding RPCInfo in a new goroutine is forbidden.
	// TODO: 如果没有 retry 的话，是不是就可以直接回收了
	rpcinfo.PutRPCInfo(ri)
	return err
}

func (kc *mergeClient) initDebugService() {
	if ds := kc.opt.DebugService; ds != nil {
		ds.RegisterProbeFunc(diagnosis.DestServiceKey, diagnosis.WrapAsProbeFunc(kc.opt.Svr.ServiceName))
		ds.RegisterProbeFunc(diagnosis.OptionsKey, diagnosis.WrapAsProbeFunc(kc.opt.DebugInfo))
		ds.RegisterProbeFunc(diagnosis.ChangeEventsKey, kc.opt.Events.Dump)
		ds.RegisterProbeFunc(diagnosis.ServiceInfoKey, diagnosis.WrapAsProbeFunc(kc.svcInfo))
	}
}

func (kc *mergeClient) richRemoteOption() {
	kc.opt.RemoteOpt.SvcInfo = kc.svcInfo
	// for client trans info handler
	if len(kc.opt.MetaHandlers) > 0 {
		// TODO in stream situations, meta is only assembled when the stream creates
		// metaHandler needs to be called separately.
		// (newClientStreamer: call WriteMeta before remotecli.NewClient)
		transInfoHdlr := bound.NewTransMetaHandler(kc.opt.MetaHandlers)
		kc.opt.RemoteOpt.PrependBoundHandler(transInfoHdlr)
	}
}

func (kc *mergeClient) buildInvokeChain() error {
	innerHandlerEp, err := kc.invokeHandleEndpoint()
	if err != nil {
		return err
	}
	kc.eps = endpoint.Chain(kc.mws...)(innerHandlerEp)
	return nil
}

func (kc *mergeClient) invokeHandleEndpoint() (endpoint.Endpoint, error) {
	// handle trace
	svrTraceCtl := kc.serverOpt.TracerCtl
	if svrTraceCtl == nil {
		svrTraceCtl = &stats.Controller{}
	}

	return func(ctx context.Context, req, resp interface{}) (err error) {
		serverCtx := context.Background()
		cliRpcInfo := rpcinfo.GetRPCInfo(ctx)
		// metainfo
		// forward transmission
		kvs := make(map[string]string, 16)
		metainfo.SaveMetaInfoToMap(ctx, kvs)
		if len(kvs) > 0 {
			serverCtx = metainfo.SetMetaInfoFromMap(serverCtx, kvs)
		}
		serverCtx = metainfo.TransferForward(serverCtx)
		// reverse transmission, backward mark
		serverCtx = metainfo.WithBackwardValuesToSend(serverCtx)

		// TODO: reuse server rpc info
		svrRpcinfo := kc.initServerRpcInfo()
		serverCtx = rpcinfo.NewCtxWithRPCInfo(serverCtx, svrRpcinfo)

		// handle common rpcinfo
		method := cliRpcInfo.To().Method()
		if ink, ok := svrRpcinfo.Invocation().(rpcinfo.InvocationSetter); ok {
			ink.SetMethodName(method)
			ink.SetServiceName(kc.svcInfo.ServiceName)
		}
		rpcinfo.AsMutableEndpointInfo(svrRpcinfo.To()).SetMethod(method)
		serverCtx = svrTraceCtl.DoStart(serverCtx, svrRpcinfo)
		// server trace
		defer func() {
			svrTraceCtl.DoFinish(serverCtx, svrRpcinfo, err)
		}()
		// meta handler
		if kc.mergeMetaHandler != nil {
			serverCtx = kc.mergeMetaHandler(ctx, serverCtx)
		}

		// server logic
		err = kc.serverEps(serverCtx, req, resp)
		if err != nil {
			if bizErr, ok := kerrors.FromBizStatusError(err); ok {
				if cliSetter, ok := cliRpcInfo.Invocation().(rpcinfo.InvocationSetter); ok {
					if len(bizErr.BizExtra()) > 0 {
						cliSetter.SetBizStatusErr(kerrors.NewBizStatusErrorWithExtra(bizErr.BizStatusCode(), bizErr.BizMessage(), bizErr.BizExtra()))
					} else {
						cliSetter.SetBizStatusErr(kerrors.NewBizStatusError(bizErr.BizStatusCode(), bizErr.BizMessage()))
					}
				}
			}
		}

		// forward key
		kvs = metainfo.AllBackwardValuesToSend(serverCtx)
		if len(kvs) > 0 {
			metainfo.SetBackwardValuesFromMap(ctx, kvs)
		}

		return err
	}, nil
}

func (kc *mergeClient) initServerRpcInfo() rpcinfo.RPCInfo {
	rpcStats := rpcinfo.AsMutableRPCStats(rpcinfo.NewRPCStats())
	if kc.serverOpt.StatsLevel != nil {
		rpcStats.SetLevel(*kc.serverOpt.StatsLevel)
	}
	// Export read-only views to external users and keep a mapping for internal users.
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.EmptyEndpointInfo(),
		rpcinfo.FromBasicInfo(kc.serverOpt.Svr),
		rpcinfo.NewServerInvocation(),
		rpcinfo.AsMutableRPCConfig(kc.serverOpt.Configs).Clone().ImmutableView(),
		rpcStats.ImmutableView(),
	)
	rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(localAddr)
	return ri
}

// Close is not concurrency safe.
func (kc *mergeClient) Close() error {
	defer func() {
		if err := recover(); err != nil {
			klog.Warnf("KITEX: panic when close client, error=%s, stack=%s", err, string(debug.Stack()))
		}
	}()
	if kc.closed {
		return nil
	}
	kc.closed = true
	var errs utils.ErrChain
	for _, cb := range kc.opt.CloseCallbacks {
		if err := cb(); err != nil {
			errs.Append(err)
		}
	}
	if kc.opt.CBSuite != nil {
		if err := kc.opt.CBSuite.Close(); err != nil {
			errs.Append(err)
		}
	}
	if errs.HasError() {
		return errs
	}
	return nil
}
