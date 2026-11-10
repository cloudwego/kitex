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
	"fmt"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync/atomic"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/localsession/backup"

	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/internal/client"
	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/acl"
	"github.com/cloudwego/kitex/pkg/consts"
	"github.com/cloudwego/kitex/pkg/diagnosis"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/endpoint/cep"
	"github.com/cloudwego/kitex/pkg/event"
	"github.com/cloudwego/kitex/pkg/fallback"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/loadbalance/lbcache"
	"github.com/cloudwego/kitex/pkg/proxy"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/bound"
	"github.com/cloudwego/kitex/pkg/remote/remotecli"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/rpctimeout"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/utils"
	"github.com/cloudwego/kitex/pkg/warmup"
	"github.com/cloudwego/kitex/transport"
)

// Client is the core interface abstraction of kitex client.
// It is designed for generated codes and should not be used directly.
// Parameter method specifies the method of a RPC call.
// Request is a packing of request parameters in the actual method defined in IDL, consist of zero, one
// or multiple arguments. So is response to the actual result type.
// Response may be nil to address oneway calls.
type Client interface {
	Call(ctx context.Context, method string, request, response interface{}) error
}

type kClient struct {
	svcInfo *serviceinfo.ServiceInfo
	eps     endpoint.UnaryEndpoint
	sEps    cep.StreamEndpoint

	opt *client.Options
	lbf *lbcache.BalancerFactory

	inited bool
	closed bool
}

// Set finalizer on kClient does not take effect, because kClient has a circular reference problem
// when construct the endpoint.Endpoint in the invokeHandleEndpoint,
// so wrapping kClient as kcFinalizerClient, and set finalizer on kcFinalizerClient, it can solve this problem.
type kcFinalizerClient struct {
	*kClient
}

func (kf *kcFinalizerClient) Call(ctx context.Context, method string, request, response interface{}) error {
	defer runtime.KeepAlive(kf)
	return kf.kClient.Call(ctx, method, request, response)
}

// NewClient creates a kitex.Client with the given ServiceInfo, it is from generated code.
func NewClient(svcInfo *serviceinfo.ServiceInfo, opts ...Option) (Client, error) {
	if svcInfo == nil {
		return nil, errors.New("NewClient: no service info")
	}
	kc := &kcFinalizerClient{kClient: &kClient{}}
	kc.svcInfo = svcInfo
	// 注意, 这个NewOptions可以插入种类丰富的配置, 实际上是执行了一堆的类似于middleware的函数
	kc.opt = client.NewOptions(opts)
	if err := kc.init(); err != nil {
		_ = kc.Close()
		return nil, err
	}
	// like os.File, if kc is garbage-collected, but Close is not called, call Close.
	runtime.SetFinalizer(kc, func(c *kcFinalizerClient) {
		_ = c.Close()
	})
	return kc, nil
}

func (kc *kClient) init() (err error) {
	initTransportProtocol(kc.svcInfo, kc.opt.Configs)
	if err = kc.checkOptions(); err != nil {
		return err
	}
	if err = kc.initCircuitBreaker(); err != nil {
		return err
	}
	if err = kc.initRetryer(); err != nil {
		return err
	}
	if err = kc.initProxy(); err != nil {
		return err
	}
	if err = kc.initConnPool(); err != nil {
		return err
	}
	if err = kc.initLBCache(); err != nil {
		return err
	}
	ctx := kc.initContext()
	mw := kc.initMiddlewares(ctx)
	kc.initDebugService()
	kc.richRemoteOption()
	if err = kc.buildInvokeChain(mw); err != nil {
		return err
	}
	if err = kc.warmingUp(); err != nil {
		return err
	}
	kc.inited = true
	return nil
}

func (kc *kClient) checkOptions() (err error) {
	if kc.opt.Svr.ServiceName == "" {
		return errors.New("service name is required")
	}
	return nil
}

func (kc *kClient) initCircuitBreaker() error {
	if kc.opt.CBSuite != nil {
		// Bus 是通知机制, 监听关心的消息, 然后进行回调
		// events类似log, 记录关键事件
		kc.opt.CBSuite.SetEventBusAndQueue(kc.opt.Bus, kc.opt.Events)
	}
	return nil
}

func (kc *kClient) initRetryer() error {
	if kc.opt.UnaryOptions.RetryContainer == nil {
		if kc.opt.UnaryOptions.RetryMethodPolicies == nil {
			return nil
		}
		// RetryContainer负责retryer的选择, 比如调用有明确指定retryer则用指定的, 否则按自己维护的map 寻找对应方法名的retryer, 都没有可以找默认的
		// 负责retry的管理, 类似于熔断的机制, 有个公共的retry 次数, 阻止短时间内的大量retry
		// 还负责策略的生命周期, 比如初始化, 策略变更等.
		kc.opt.InitRetryContainer()
	}
	// 内置3种retry策略, 失败retry. backup retry, 就是不等到失败再retry, 而是发起备份retry. mixed retry, 混合的场景
	// 策略无法完全自定义, 但是可以部分控制retry的时机, 比如看见xx 错误再retry等
	// 可以从远程下载配置到本地, 然后再更新本地
	return kc.opt.UnaryOptions.RetryContainer.Init(kc.opt.UnaryOptions.RetryMethodPolicies, kc.opt.UnaryOptions.RetryWithResult)
}

func (kc *kClient) initContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, endpoint.CtxEventBusKey, kc.opt.Bus)
	ctx = context.WithValue(ctx, endpoint.CtxEventQueueKey, kc.opt.Events)
	ctx = context.WithValue(ctx, rpctimeout.TimeoutAdjustKey, &kc.opt.ExtraTimeout)
	if chr, ok := kc.opt.Proxy.(proxy.ContextHandler); ok {
		ctx = chr.HandleContext(ctx)
	}
	return ctx
}

func (kc *kClient) initProxy() error {
	if kc.opt.Proxy != nil {
		cfg := proxy.Config{
			ServerInfo:   kc.opt.Svr,
			Resolver:     kc.opt.Resolver,
			Balancer:     kc.opt.Balancer,
			Pool:         kc.opt.RemoteOpt.ConnPool,
			FixedTargets: kc.opt.Targets,
			RPCConfig:    kc.opt.Configs,
		}
		// 可以处理非常丰富的事情, 比如创建连接池, 保存配置, 后续的Resolver以及Balancer, Pool的赋值可以体现出proxy的影响范围
		// 可以在后续调用proxy.ResolveProxyInstance 获取一个实例, 后续在newProxyMW可以调用
		if err := kc.opt.Proxy.Configure(&cfg); err != nil {
			return err
		}
		// update fields in the client option for further use.
		kc.opt.Resolver = cfg.Resolver
		kc.opt.Balancer = cfg.Balancer
		// close predefined pool when proxy init new pool.
		if cfg.Pool != kc.opt.RemoteOpt.ConnPool && kc.opt.RemoteOpt.ConnPool != nil {
			kc.opt.RemoteOpt.ConnPool.Close()
		}
		kc.opt.RemoteOpt.ConnPool = cfg.Pool
		kc.opt.Targets = cfg.FixedTargets
	}
	return nil
}

/*
这个连接池比较广义, 正常是tcp连接
可以是uds连接
连接复用有几种形式

 1. 同一个连接先后复用, A 发完, A 收到响应, 归还连接但不关闭; 获取归还的链接, B再发, B收到响应

 2. 多路复用. 同一个连接同时承载多个请求. 这个需要协议的支持, 对于一个字节流, 要能识别出一个请求的边界, 以及这个请求的归属.
    多路复用对协议的要求:
    a. 能够准确识别字节流的一段报文的边界[报文A][报文B]
    b. 每个报文有id表示自身属于哪一个请求

    多路复用对客户端服务端的要求:
    a. 配套实现, 对于同一请求, 保证发送同id的报文, 不同请求发送不同id的报文. 服务端要能按id分别处理对应请求的报文

    kitex实现可以参考 netpollmux, 但不再维护了. 所以先不关注
    而grpc与ttheader streaming还有对应的多路复用实现

有几个问题需要弄清楚:
1. 传输层与上层的关系是什么样的, 一个请求发送完毕后获得返回是怎么通知上层处理的.
  - 上层是调用Recv阻塞, 底层TODO
    2. grpc/http2 在kitex里是怎么被使用的
    TODO
*/
func (kc *kClient) initConnPool() error {
	pool := kc.opt.RemoteOpt.ConnPool
	// client关闭回调, 清理连接池
	kc.opt.CloseCallbacks = append(kc.opt.CloseCallbacks, pool.Close)

	// debug用, 可以dump连接池信息
	if df, ok := pool.(interface{ Dump() interface{} }); ok {
		kc.opt.DebugService.RegisterProbeFunc(diagnosis.ConnPoolKey, df.Dump)
	}
	if r, ok := pool.(remote.ConnPoolReporter); ok && kc.opt.RemoteOpt.EnableConnPoolReporter {
		r.EnableReporter()
	}

	// 长连接监听实例信息
	if long, ok := pool.(remote.LongConnPool); ok {
		kc.opt.Bus.Watch(discovery.ChangeEventName, func(ev *event.Event) {
			ch, ok := ev.Extra.(*discovery.Change)
			if !ok {
				return
			}
			for _, inst := range ch.Removed {
				if addr := inst.Address(); addr != nil {
					long.Clean(addr.Network(), addr.String())
				}
			}
		})
	}
	return nil
}

/*
主要是绑定了回调.
注意, 缓存是全局的, 多个client共享.
里面主要注册了事件的回调函数. initLBCache主要是串起了各个组件
1. 初始化的时候balancer, resolver的具体类型已经确定, 这里通过包装器使得可以调用不同种类的balancer, resolver

2. LBCache.Refresh() 触发整个流程, 调用resolver获取最新实际, 进行Diff, 然后执行包装器的Rebalance函数.

3. 包装器的Rebalance函数先执行onChange回调, 将Diff通知给其他感兴趣的组件, 然后调用balancer自身的Rebalance函数

4. balancer自身的Rebalance函数 依照自身的实现进行实例选择的更新

目的是不同的client A, B有可能调用相同的服务端, 在这种情况下, resolver等返回的信息共享是有相当大的价值的. 这一共享通过对balancerFactory的共享间接达到
*/
func (kc *kClient) initLBCache() error {
	if kc.opt.Proxy != nil && kc.opt.Resolver == nil {
		return nil
	}
	// 回调
	onChange := discoveryEventHandler(discovery.ChangeEventName, kc.opt.Bus, kc.opt.Events)
	onDelete := discoveryEventHandler(discovery.DeleteEventName, kc.opt.Bus, kc.opt.Events)
	resolver := kc.opt.Resolver
	if resolver == nil {
		// fake a resolver instead of returning an error directly because users may use
		// callopt.WithHostPort to specify target addresses after NewClient.
		resolver = &discovery.SynthesizedResolver{
			ResolveFunc: func(ctx context.Context, target string) (discovery.Result, error) {
				return discovery.Result{}, kerrors.ErrNoResolver
			},
			NameFunc: func() string { return "no_resolver" },
		}
	}
	// because we cannot ensure that user's custom loadbalancer is cacheable, we need to disable it here
	cacheOpts := lbcache.Options{DiagnosisService: kc.opt.DebugService, Cacheable: false}
	balancer := kc.opt.Balancer
	if balancer == nil {
		// default internal lb balancer is cacheable
		cacheOpts.Cacheable = true
		balancer = loadbalance.NewWeightedBalancer()
	}
	if kc.opt.BalancerCacheOpt != nil {
		cacheOpts = *kc.opt.BalancerCacheOpt
	}
	kc.lbf = lbcache.NewBalancerFactory(resolver, balancer, cacheOpts)
	rbIdx := kc.lbf.RegisterRebalanceHook(onChange)
	kc.opt.CloseCallbacks = append(kc.opt.CloseCallbacks, func() error {
		kc.lbf.DeregisterRebalanceHook(rbIdx)
		return nil
	})
	dIdx := kc.lbf.RegisterDeleteHook(onDelete)
	kc.opt.CloseCallbacks = append(kc.opt.CloseCallbacks, func() error {
		kc.lbf.DeregisterDeleteHook(dIdx)
		return nil
	})
	return nil
}

type middleware struct {
	// unary middleware
	mws  []endpoint.Middleware      // wrap the mws at `context mw -> customized mws -> other mws`
	uMws []endpoint.UnaryMiddleware // wrap the mws at `service cb mw -> xds router -> rpctimeout mw -> customized unary mws`

	// streaming middleware
	smws []endpoint.Middleware  // wrap the mws at `xds router -> context mw -> customized mws -> other mws`
	sMws []cep.StreamMiddleware // wrap the mws at `service cb mw -> customized stream mws`
}

// For unary:
// service cb mw -> xds router -> rpctimeout mw -> customized unary mws -> context mw -> customized mws -> other mws

// For streaming:
// service cb mw -> customized stream mws -> xds router -> context mw -> customized mws -> other mws

// The reason why the xds router must be placed before the timeout middleware is that the timeout configuration is
// obtained from the rds config, and the timeout middleware only takes effect in the unary scenario.
// Therefore, in the streaming scenario, the xds router can be placed after the stream middleware.
func (kc *kClient) initMiddlewares(ctx context.Context) (mw middleware) {
	// 1. universal middlewares
	builderMWs := richMWsWithBuilder(ctx, kc.opt.MWBs)
	mw.mws = append(mw.mws, contextMW)
	mw.mws = append(mw.mws, builderMWs...)
	mw.mws = append(mw.mws, acl.NewACLMiddleware(kc.opt.ACLRules))
	if kc.opt.Proxy == nil {
		mw.mws = append(mw.mws, newResolveMWBuilder(kc.lbf)(ctx))
		mw.mws = append(mw.mws, kc.opt.CBSuite.InstanceCBMW())
		mw.mws = append(mw.mws, richMWsWithBuilder(ctx, kc.opt.IMWBs)...)
	} else {
		if kc.opt.Resolver != nil { // customized service discovery
			mw.mws = append(mw.mws, newResolveMWBuilder(kc.lbf)(ctx))
		}
		// 这个地方是调用proxy的地方
		mw.mws = append(mw.mws, newProxyMW(kc.opt.Proxy))
	}
	mw.mws = append(mw.mws, newIOErrorHandleMW(kc.opt.ErrHandle))

	// 2. unary middlewares
	mw.uMws = append(mw.uMws, kc.opt.CBSuite.ServiceCBMW().ToUnaryMiddleware())
	if kc.opt.XDSEnabled && kc.opt.XDSRouterMiddleware != nil && kc.opt.Proxy == nil {
		// integrate xds if enabled
		mw.uMws = append(mw.uMws, kc.opt.XDSRouterMiddleware.ToUnaryMiddleware())
	}
	mw.uMws = append(mw.uMws, rpcTimeoutMW(ctx))
	kc.opt.UnaryOptions.InitMiddlewares(ctx)
	mw.uMws = append(mw.uMws, kc.opt.UnaryOptions.UnaryMiddlewares...)

	// 3. stream middlewares
	kc.opt.Streaming.InitMiddlewares(ctx)
	mw.smws = mw.mws
	if kc.opt.XDSEnabled && kc.opt.XDSRouterMiddleware != nil && kc.opt.Proxy == nil {
		// integrate xds if enabled
		mw.smws = append([]endpoint.Middleware{kc.opt.XDSRouterMiddleware}, mw.smws...)
	}
	kc.opt.StreamOptions.InitMiddlewares(ctx)
	mw.sMws = append([]cep.StreamMiddleware{kc.opt.CBSuite.StreamingServiceCBMW()}, kc.opt.StreamOptions.StreamMiddlewares...)
	return
}

func richMWsWithBuilder(ctx context.Context, mwBs []endpoint.MiddlewareBuilder) (mws []endpoint.Middleware) {
	for i := range mwBs {
		if mw := mwBs[i](ctx); mw != nil {
			mws = append(mws, mw)
		}
	}
	return
}

// initRPCInfo initializes the RPCInfo structure and attaches it to context.
func (kc *kClient) initRPCInfo(ctx context.Context, method string, retryTimes int,
	firstRI rpcinfo.RPCInfo, streamCall bool,
) (context.Context, rpcinfo.RPCInfo, *callopt.CallOptions) {
	return initRPCInfo(ctx, method, kc.opt, kc.svcInfo, retryTimes, firstRI, streamCall)
}

func applyCallOptions(ctx context.Context, cfg rpcinfo.MutableRPCConfig, svr remoteinfo.RemoteInfo, opt *client.Options) (context.Context, *callopt.CallOptions) {
	cos := CallOptionsFromCtx(ctx)
	if len(cos) > 0 {
		info, callOpts := callopt.Apply(cos, cfg, svr, opt.Locks, opt.HTTPResolver)
		ctx = context.WithValue(ctx, ctxCallOptionInfoKey, info)
		return ctx, callOpts
	}
	opt.Locks.ApplyLocks(cfg, svr)
	return ctx, nil
}

// Call implements the Client interface .
func (kc *kClient) Call(ctx context.Context, method string, request, response interface{}) (err error) {
	// merge backup context if no metainfo found in ctx
	ctx = backup.RecoverCtxOnDemands(ctx, kc.opt.CtxBackupHandler)

	validateForCall(ctx, kc.inited, kc.closed)
	var ri rpcinfo.RPCInfo
	var callOpts *callopt.CallOptions
	ctx, ri, callOpts = kc.initRPCInfo(ctx, method, 0, nil, false)

	ctx = kc.opt.TracerCtl.DoStart(ctx, ri)
	var reportErr error
	var recycleRI bool
	defer func() {
		if panicInfo := recover(); panicInfo != nil {
			err = rpcinfo.ClientPanicToErr(ctx, panicInfo, ri, false)
			reportErr = err
		}
		kc.opt.TracerCtl.DoFinish(ctx, ri, reportErr)
		if recycleRI {
			// why need check recycleRI to decide if recycle RPCInfo?
			// 1. no retry, rpc timeout happen will cause panic when response return
			// 2. retry success, will cause panic when first call return
			// 3. backup request may cause panic, cannot recycle first RPCInfo
			// RPCInfo will be recycled after rpc is finished,
			// holding RPCInfo in a new goroutine is forbidden.
			rpcinfo.PutRPCInfo(ri)
		}
		callOpts.Recycle()
	}()

	callOptRetry := getCalloptRetryPolicy(callOpts)
	if kc.opt.UnaryOptions.RetryContainer == nil && callOptRetry != nil && callOptRetry.Enable {
		// setup retry in callopt
		kc.opt.InitRetryContainer()
	}

	// Add necessary keys to context for isolation between kitex client method calls
	ctx = retry.PrepareRetryContext(ctx)

	if mi := ri.Invocation().MethodInfo(); mi == nil {
		err = kerrors.ErrNonExistentMethod(kc.svcInfo.ServiceName, method)
	} else if mi.StreamingMode() != serviceinfo.StreamingNone && mi.StreamingMode() != serviceinfo.StreamingUnary {
		err = kerrors.ErrNotUnaryMethod(kc.svcInfo.ServiceName, method)
	} else if kc.opt.UnaryOptions.RetryContainer == nil {
		// call without retry policy
		err = kc.eps(ctx, request, response)
		if err == nil {
			recycleRI = true
		}
	} else {
		var lastRI rpcinfo.RPCInfo
		lastRI, recycleRI, err = kc.opt.UnaryOptions.RetryContainer.WithRetryIfNeeded(ctx, callOptRetry, kc.rpcCallWithRetry(ri, method), ri, request, response)
		if ri != lastRI {
			// reset ri of ctx to lastRI
			ctx = rpcinfo.NewCtxWithRPCInfo(ctx, lastRI)
		}
		ri = lastRI
	}

	// do fallback if with setup
	err, reportErr = doFallbackIfNeeded(ctx, ri, request, response, err, kc.opt.UnaryOptions.Fallback, callOpts)
	return err
}

func (kc *kClient) rpcCallWithRetry(ri rpcinfo.RPCInfo, method string) retry.RPCCallFunc {
	// call with retry policy
	var callTimes int32
	// prevRI represents a value of rpcinfo.RPCInfo type.
	var prevRI atomic.Value
	return func(ctx context.Context, r retry.Retryer, request, response interface{}) (rpcinfo.RPCInfo, error) {
		currCallTimes := int(atomic.AddInt32(&callTimes, 1))
		cRI := ri
		if currCallTimes > 1 {
			ctx, cRI, _ = kc.initRPCInfo(ctx, method, currCallTimes-1, ri, false)
			ctx = metainfo.WithPersistentValue(ctx, retry.TransitKey, strconv.Itoa(currCallTimes-1))
			if prevRI.Load() == nil {
				prevRI.Store(ri)
			}
			r.Prepare(ctx, prevRI.Load().(rpcinfo.RPCInfo), cRI)
			prevRI.Store(cRI)
		}
		callErr := kc.eps(ctx, request, response)
		return cRI, callErr
	}
}

func (kc *kClient) initDebugService() {
	if ds := kc.opt.DebugService; ds != nil {
		ds.RegisterProbeFunc(diagnosis.DestServiceKey, diagnosis.WrapAsProbeFunc(kc.opt.Svr.ServiceName))
		ds.RegisterProbeFunc(diagnosis.OptionsKey, diagnosis.WrapAsProbeFunc(kc.opt.DebugInfo))
		ds.RegisterProbeFunc(diagnosis.ChangeEventsKey, kc.opt.Events.Dump)
		ds.RegisterProbeFunc(diagnosis.ServiceInfosKey, diagnosis.WrapAsProbeFunc(map[string]*serviceinfo.ServiceInfo{kc.svcInfo.ServiceName: kc.svcInfo}))
	}
}

func (kc *kClient) richRemoteOption() {
	kc.opt.RemoteOpt.SvcInfo = kc.svcInfo
	// for client trans info handler
	if len(kc.opt.MetaHandlers) > 0 {
		// TODO in stream situations, meta is only assembled when the stream creates
		// metaHandler needs to be called separately.
		// (newClientStreamer: call WriteMeta before remotecli.NewClient)
		transInfoHdlr := bound.NewTransMetaHandler(kc.opt.MetaHandlers)
		kc.opt.RemoteOpt.PrependBoundHandler(transInfoHdlr)

		// add meta handlers into streaming meta handlers
		for _, h := range kc.opt.MetaHandlers {
			if shdlr, ok := h.(remote.StreamingMetaHandler); ok {
				kc.opt.RemoteOpt.StreamingMetaHandlers = append(kc.opt.RemoteOpt.StreamingMetaHandlers, shdlr)
			}
		}
	}
}

func (kc *kClient) buildInvokeChain(mw middleware) error {
	innerHandlerEp, err := kc.invokeHandleEndpoint()
	if err != nil {
		return err
	}
	eps := endpoint.Chain(mw.mws...)(innerHandlerEp)

	kc.eps = endpoint.UnaryChain(mw.uMws...)(func(ctx context.Context, req, resp interface{}) (err error) {
		return eps(ctx, req, resp)
	})

	innerStreamingEp, err := kc.invokeStreamingEndpoint()
	if err != nil {
		return err
	}
	sEps := endpoint.Chain(mw.smws...)(innerStreamingEp)
	kc.sEps = cep.StreamChain(mw.sMws...)(func(ctx context.Context) (st streaming.ClientStream, err error) {
		resp := &streaming.Result{}
		err = sEps(ctx, nil, resp)
		if err != nil {
			return nil, err
		}
		return resp.ClientStream, nil
	})
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

		cli, err := remotecli.NewClient(ctx, ri, transPipl, kc.opt.RemoteOpt)
		if err != nil {
			return
		}

		defer cli.Recycle()
		m := ri.Invocation().MethodInfo()
		if m.OneWay() {
			sendMsg = remote.NewMessage(req, ri, remote.Oneway, remote.Client)
		} else {
			sendMsg = remote.NewMessage(req, ri, remote.Call, remote.Client)
		}

		if err = cli.Send(ctx, ri, sendMsg); err != nil {
			return
		}
		if m.OneWay() {
			cli.Recv(ctx, ri, nil)
			return nil
		}

		recvMsg = remote.NewMessage(resp, ri, remote.Reply, remote.Client)
		err = cli.Recv(ctx, ri, recvMsg)
		return err
	}, nil
}

// Close is not concurrency safe.
func (kc *kClient) Close() error {
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

func initTransportProtocol(svcInfo *serviceinfo.ServiceInfo, cfg rpcinfo.RPCConfig) {
	mutableRPCConfig := rpcinfo.AsMutableRPCConfig(cfg)
	mutableRPCConfig.SetPayloadCodec(svcInfo.PayloadCodec)
	if svcInfo.PayloadCodec == serviceinfo.Protobuf && cfg.TransportProtocol()&transport.GRPC != transport.GRPC {
		// pb use ttheader framed by default
		mutableRPCConfig.SetTransportProtocol(transport.TTHeaderFramed)
	}
}

func (kc *kClient) warmingUp() error {
	if kc.opt.WarmUpOption == nil {
		return nil
	}
	wuo := kc.opt.WarmUpOption
	doWarmupPool := wuo.PoolOption != nil && kc.opt.Proxy == nil

	// service discovery
	if kc.opt.Resolver == nil {
		return nil
	}
	nas := make(map[string][]string)
	ctx := context.Background()

	var dests []rpcinfo.EndpointInfo
	if ro := kc.opt.WarmUpOption.ResolverOption; ro != nil {
		for _, d := range ro.Dests {
			dests = append(dests, rpcinfo.FromBasicInfo(d))
		}
	}
	if len(dests) == 0 && doWarmupPool && len(wuo.PoolOption.Targets) == 0 {
		// build a default destination for the resolver
		cfg := rpcinfo.AsMutableRPCConfig(kc.opt.Configs).Clone()
		rmt := remoteinfo.NewRemoteInfo(kc.opt.Svr, "*")
		ctx, _ = applyCallOptions(ctx, cfg, rmt, kc.opt)
		dests = append(dests, rmt.ImmutableView())
	}

	for _, dest := range dests {
		lb, err := kc.lbf.Get(ctx, dest)
		if err != nil {
			switch kc.opt.WarmUpOption.ErrorHandling {
			case warmup.IgnoreError:
			case warmup.WarningLog:
				klog.Warnf("KITEX: failed to warm up service discovery: %s", err.Error())
			case warmup.ErrorLog:
				klog.Errorf("KITEX: failed to warm up service discovery: %s", err.Error())
			case warmup.FailFast:
				return fmt.Errorf("service discovery warm-up: %w", err)
			}
			continue
		}
		if res, ok := lb.GetResult(); ok {
			for _, i := range res.Instances {
				if addr := i.Address(); addr != nil {
					n, a := addr.Network(), addr.String()
					nas[n] = append(nas[n], a)
				}
			}
		}
	}

	// connection pool
	if !doWarmupPool {
		return nil
	}

	if len(wuo.PoolOption.Targets) == 0 {
		wuo.PoolOption.Targets = nas
	}

	pool := kc.opt.RemoteOpt.ConnPool
	if wp, ok := pool.(warmup.Pool); ok {
		co := remote.ConnOption{
			Dialer:         kc.opt.RemoteOpt.Dialer,
			ConnectTimeout: kc.opt.Configs.ConnectTimeout(),
		}
		err := wp.WarmUp(kc.opt.WarmUpOption.ErrorHandling, wuo.PoolOption, co)
		if err != nil {
			switch kc.opt.WarmUpOption.ErrorHandling {
			case warmup.IgnoreError:
			case warmup.WarningLog:
				klog.Warnf("KITEX: failed to warm up connection pool: %s", err.Error())
			case warmup.ErrorLog:
				klog.Errorf("KITEX: failed to warm up connection pool: %s", err.Error())
			case warmup.FailFast:
				return fmt.Errorf("connection pool warm-up: %w", err)
			}
		}
	} else {
		klog.Warnf("KITEX: connection pool<%T> does not support warm-up operation", pool)
	}

	return nil
}

func validateForCall(ctx context.Context, inited, closed bool) {
	if !inited {
		panic("client not initialized")
	}
	if closed {
		panic("client is already closed")
	}
	if ctx == nil {
		panic("ctx is nil")
	}
}

func getCalloptRetryPolicy(callOpts *callopt.CallOptions) (callOptRetry *retry.Policy) {
	if callOpts != nil {
		if callOpts.RetryPolicy.Enable {
			callOptRetry = &callOpts.RetryPolicy
		}
	}
	return
}

func doFallbackIfNeeded(ctx context.Context, ri rpcinfo.RPCInfo, request, response interface{}, oriErr error, cliFallback *fallback.Policy, callOpts *callopt.CallOptions) (err, reportErr error) {
	fallback, hasFallback := getFallbackPolicy(cliFallback, callOpts)
	err = oriErr
	reportErr = oriErr
	var fbErr error
	if hasFallback {
		reportAsFB := false
		// Notice: If rpc err is nil, rpcStatAsFB will always be false, even if it's set to true by user.
		fbErr, reportAsFB = fallback.DoIfNeeded(ctx, ri, request, response, oriErr)
		if reportAsFB {
			reportErr = fbErr
		}
		err = fbErr
	}

	if err == nil && !hasFallback {
		err = ri.Invocation().BizStatusErr()
	}
	return
}

// return fallback policy from call option and client option.
func getFallbackPolicy(cliOptFB *fallback.Policy, callOpts *callopt.CallOptions) (fb *fallback.Policy, hasFallback bool) {
	var callOptFB *fallback.Policy
	if callOpts != nil {
		callOptFB = callOpts.Fallback
	}
	if callOptFB != nil {
		return callOptFB, true
	}
	if cliOptFB != nil {
		return cliOptFB, true
	}
	return nil, false
}

// purifyProtocol purifies the transport protocol for one rpc request according to whether it's stream call or not.
// It simplifies the latter judgement logic to avoid processing multiple mixed protocols.
func purifyProtocol(cfg rpcinfo.MutableRPCConfig, tp transport.Protocol, streamCall bool) {
	cfg.SetTransportProtocol(transport.PurePayload) // reset transport protocol to pure payload
	if streamCall {
		if tp&(transport.GRPC|transport.GRPCStreaming) != 0 {
			// reset to grpc transport protocol for better forward compatibility
			tp = transport.GRPC
		} else if tp&transport.TTHeaderStreaming != 0 {
			// also add TTHeaderFramed to be compatible with protocol judgement logic,
			// cuz to some extent, TTHeaderStreaming is also part of the ttheader protocol.
			tp = transport.TTHeaderStreaming | transport.TTHeaderFramed
		}
	} else {
		if tp&transport.GRPC != 0 {
			tp = transport.GRPC
		} else if tp&(transport.TTHeaderStreaming|transport.GRPCStreaming) != 0 {
			tp = tp &^ (transport.TTHeaderStreaming | transport.GRPCStreaming) // remove streaming protocol
		}
	}
	cfg.SetTransportProtocol(tp)
}

func initRPCInfo(ctx context.Context, method string, opt *client.Options, svcInfo *serviceinfo.ServiceInfo,
	retryTimes int, firstRI rpcinfo.RPCInfo, streamCall bool,
) (context.Context, rpcinfo.RPCInfo, *callopt.CallOptions) {
	cfg := rpcinfo.AsMutableRPCConfig(opt.Configs).Clone()
	rmt := remoteinfo.NewRemoteInfo(opt.Svr, method)
	var callOpts *callopt.CallOptions
	ctx, callOpts = applyCallOptions(ctx, cfg, rmt, opt)
	var rpcStats rpcinfo.MutableRPCStats
	if firstRI != nil {
		rpcStats = rpcinfo.AsMutableRPCStats(firstRI.Stats().CopyForRetry())
	} else {
		rpcStats = rpcinfo.AsMutableRPCStats(rpcinfo.NewRPCStats())
	}
	if opt.StatsLevel != nil {
		rpcStats.SetLevel(*opt.StatsLevel)
	}

	mi := svcInfo.MethodInfo(ctx, method)
	if mi != nil && mi.OneWay() {
		cfg.SetInteractionMode(rpcinfo.Oneway)
	}
	if retryTimes > 0 {
		// it is used to distinguish the request is a local retry request.
		rmt.SetTag(rpcinfo.RetryTag, strconv.Itoa(retryTimes))
	}

	// Export read-only views to external users.
	ri := rpcinfo.NewRPCInfo(
		rpcinfo.FromBasicInfo(opt.Cli),
		rmt.ImmutableView(),
		rpcinfo.NewInvocation(svcInfo.ServiceName, method, svcInfo.GetPackageName()),
		cfg.ImmutableView(),
		rpcStats.ImmutableView(),
	)

	inkSetter := ri.Invocation().(rpcinfo.InvocationSetter)
	// set ServiceInfo to judge whether it's combine service in ttheader codec.
	inkSetter.SetExtra(rpcinfo.InvocationServiceInfoKey, svcInfo)
	if mi != nil {
		inkSetter.SetStreamingMode(mi.StreamingMode())
		inkSetter.SetMethodInfo(mi)
	}
	if streamCall {
		cfg.SetInteractionMode(rpcinfo.Streaming)
	}

	purifyProtocol(cfg, ri.Config().TransportProtocol(), streamCall)

	if fromMethod := ctx.Value(consts.CtxKeyMethod); fromMethod != nil {
		rpcinfo.AsMutableEndpointInfo(ri.From()).SetMethod(fromMethod.(string))
	}

	if p := opt.Timeouts; p != nil {
		if c := p.Timeouts(ri); c != nil {
			_ = cfg.SetRPCTimeout(c.RPCTimeout())
			_ = cfg.SetConnectTimeout(c.ConnectTimeout())
			_ = cfg.SetReadWriteTimeout(c.ReadWriteTimeout())
		}
	}
	sopt := opt.StreamOptions
	if sopt.RecvTimeout > 0 {
		cfg.SetStreamRecvTimeout(sopt.RecvTimeout)
	}
	if sopt.RecvTimeoutConfig.Timeout > 0 {
		cfg.SetStreamRecvTimeoutConfig(sopt.RecvTimeoutConfig)
	}

	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)

	if callOpts != nil {
		if callOpts.CompressorName != "" {
			// set send grpc compressor at client to tell how to server decode
			remote.SetSendCompressor(ri, callOpts.CompressorName)
		}
		if callOpts.StreamOptions.RecvTimeout != 0 {
			cfg.SetStreamRecvTimeout(callOpts.StreamOptions.RecvTimeout)
		}
		if callOpts.StreamOptions.RecvTimeoutConfig.Timeout > 0 {
			cfg.SetStreamRecvTimeoutConfig(callOpts.StreamOptions.RecvTimeoutConfig)
		}
		// WithBinaryGenericIDLService only take effect for BinaryThriftGenericV2 and BinaryPbGeneric
		if callOpts.BinaryGenericIDLService != "" && igeneric.BinaryGenericSupportsMultiService(svcInfo) {
			inkSetter.SetServiceName(callOpts.BinaryGenericIDLService)
		}
	}

	return ctx, ri, callOpts
}
