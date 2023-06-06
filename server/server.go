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

// Package server .
package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	internal_server "github.com/cloudwego/kitex/internal/server"
	"github.com/cloudwego/kitex/pkg/acl"
	"github.com/cloudwego/kitex/pkg/diagnosis"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/gofunc"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limiter"
	"github.com/cloudwego/kitex/pkg/registry"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/bound"
	"github.com/cloudwego/kitex/pkg/remote/remotesvr"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/stats"
)

// Server is a abstraction of a RPC server. It accepts connections and dispatches them to the service
// registered to it.
type Server interface {
	RegisterService(svcInfo *serviceinfo.ServiceInfo, handler interface{}) error
	GetServiceInfo() *serviceinfo.ServiceInfo
	Run() error
	Stop() error
}

type server struct {
	opt     *internal_server.Options
	svcInfo *serviceinfo.ServiceInfo

	// actual rpc service implement of biz
	handler interface{}
	eps     endpoint.Endpoint
	mws     []endpoint.Middleware
	svr     remotesvr.Server
	stopped sync.Once

	sync.Mutex
}

// NewServer creates a server with the given Options.
func NewServer(ops ...Option) Server {
	s := &server{
		opt: internal_server.NewOptions(ops),
	}
	s.init()
	return s
}

func (s *server) init() {
	ctx := fillContext(s.opt)
	s.mws = richMWsWithBuilder(ctx, s.opt.MWBs, s)
	s.mws = append(s.mws, acl.NewACLMiddleware(s.opt.ACLRules))
	if s.opt.ErrHandle != nil {
		// errorHandleMW must be the last middleware,
		// to ensure it only catches the server handler's error.
		s.mws = append(s.mws, newErrorHandleMW(s.opt.ErrHandle))
	}
	if ds := s.opt.DebugService; ds != nil {
		ds.RegisterProbeFunc(diagnosis.OptionsKey, diagnosis.WrapAsProbeFunc(s.opt.DebugInfo))
		ds.RegisterProbeFunc(diagnosis.ChangeEventsKey, s.opt.Events.Dump)
	}
	s.buildInvokeChain()
}

func (s *server) Endpoints() endpoint.Endpoint {
	return s.eps
}

func (s *server) SetEndpoints(e endpoint.Endpoint) {
	s.eps = e
}

func (s *server) Option() *internal_server.Options {
	return s.opt
}

func fillContext(opt *internal_server.Options) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, endpoint.CtxEventBusKey, opt.Bus)
	ctx = context.WithValue(ctx, endpoint.CtxEventQueueKey, opt.Events)
	return ctx
}

func richMWsWithBuilder(ctx context.Context, mwBs []endpoint.MiddlewareBuilder, ks *server) []endpoint.Middleware {
	for i := range mwBs {
		ks.mws = append(ks.mws, mwBs[i](ctx))
	}
	return ks.mws
}

// newErrorHandleMW provides a hook point for server error handling.
func newErrorHandleMW(errHandle func(context.Context, error) error) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request, response interface{}) error {
			err := next(ctx, request, response)
			if err == nil {
				return nil
			}
			return errHandle(ctx, err)
		}
	}
}

func (s *server) initOrResetRPCInfoFunc() func(rpcinfo.RPCInfo, net.Addr) rpcinfo.RPCInfo {
	return func(ri rpcinfo.RPCInfo, rAddr net.Addr) rpcinfo.RPCInfo {
		// Reset rpcinfo if it exists in ctx.
		if ri != nil {
			fi := rpcinfo.AsMutableEndpointInfo(ri.From())
			fi.Reset()
			fi.SetAddress(rAddr)
			rpcinfo.AsMutableEndpointInfo(ri.To()).ResetFromBasicInfo(s.opt.Svr)
			if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
				setter.Reset()
			}
			rpcinfo.AsMutableRPCConfig(ri.Config()).CopyFrom(s.opt.Configs)
			rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats())
			rpcStats.Reset()
			if s.opt.StatsLevel != nil {
				rpcStats.SetLevel(*s.opt.StatsLevel)
			}
			return ri
		}
		rpcStats := rpcinfo.AsMutableRPCStats(rpcinfo.NewRPCStats())
		if s.opt.StatsLevel != nil {
			rpcStats.SetLevel(*s.opt.StatsLevel)
		}

		// Export read-only views to external users and keep a mapping for internal users.
		ri = rpcinfo.NewRPCInfo(
			rpcinfo.EmptyEndpointInfo(),
			rpcinfo.FromBasicInfo(s.opt.Svr),
			rpcinfo.NewServerInvocation(),
			rpcinfo.AsMutableRPCConfig(s.opt.Configs).Clone().ImmutableView(),
			rpcStats.ImmutableView(),
		)
		rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(rAddr)
		return ri
	}
}

func (s *server) buildInvokeChain() {
	innerHandlerEp := s.invokeHandleEndpoint()
	s.eps = endpoint.Chain(s.mws...)(innerHandlerEp)
}

// RegisterService should not be called by users directly.
func (s *server) RegisterService(svcInfo *serviceinfo.ServiceInfo, handler interface{}) error {
	if s.svcInfo != nil {
		panic(fmt.Sprintf("Service[%s] is already defined", s.svcInfo.ServiceName))
	}
	s.svcInfo = svcInfo
	s.handler = handler
	diagnosis.RegisterProbeFunc(s.opt.DebugService, diagnosis.ServiceInfoKey, diagnosis.WrapAsProbeFunc(s.svcInfo))
	return nil
}

func (s *server) GetServiceInfo() *serviceinfo.ServiceInfo {
	return s.svcInfo
}

// Run runs the server.
func (s *server) Run() (err error) {
	if s.svcInfo == nil {
		return errors.New("no service, use RegisterService to set one")
	}
	if err = s.check(); err != nil {
		return err
	}
	svrCfg := s.opt.RemoteOpt
	addr := svrCfg.Address // should not be nil
	if s.opt.Proxy != nil {
		svrCfg.Address, err = s.opt.Proxy.Replace(addr)
		if err != nil {
			return
		}
	}

	s.fillMoreServiceInfo(s.opt.RemoteOpt.Address)
	s.richRemoteOption()
	transHdlr, err := s.newSvrTransHandler()
	if err != nil {
		return err
	}
	s.Lock()
	s.svr, err = remotesvr.NewServer(s.opt.RemoteOpt, s.eps, transHdlr)
	s.Unlock()
	if err != nil {
		return err
	}

	// start profiler
	if s.opt.RemoteOpt.Profiler != nil {
		gofunc.GoFunc(context.Background(), func() {
			klog.Info("KITEX: server starting profiler")
			err := s.opt.RemoteOpt.Profiler.Run(context.Background())
			if err != nil {
				klog.Errorf("KITEX: server started profiler error: error=%s", err.Error())
			}
		})
	}

	errCh := s.svr.Start()
	select {
	case err = <-errCh:
		klog.Errorf("KITEX: server start error: error=%s", err.Error())
		return err
	default:
	}
	muStartHooks.Lock()
	for i := range onServerStart {
		go onServerStart[i]()
	}
	muStartHooks.Unlock()
	s.Lock()
	s.buildRegistryInfo(s.svr.Address())
	s.Unlock()

	if err = s.waitExit(errCh); err != nil {
		klog.Errorf("KITEX: received error and exit: error=%s", err.Error())
	}
	if e := s.Stop(); e != nil && err == nil {
		err = e
		klog.Errorf("KITEX: stop server error: error=%s", e.Error())
	}
	return
}

// Stop stops the server gracefully.
func (s *server) Stop() (err error) {
	s.stopped.Do(func() {
		s.Lock()
		defer s.Unlock()

		muShutdownHooks.Lock()
		for i := range onShutdown {
			onShutdown[i]()
		}
		muShutdownHooks.Unlock()

		if s.opt.RegistryInfo != nil {
			err = s.opt.Registry.Deregister(s.opt.RegistryInfo)
			s.opt.RegistryInfo = nil
		}
		if s.svr != nil {
			if e := s.svr.Stop(); e != nil {
				err = e
			}
			s.svr = nil
		}
	})
	return
}

func (s *server) invokeHandleEndpoint() endpoint.Endpoint {
	return func(ctx context.Context, args, resp interface{}) (err error) {
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()
		if methodName == "" && s.svcInfo.ServiceName != serviceinfo.GenericService {
			return errors.New("method name is empty in rpcinfo, should not happen")
		}
		defer func() {
			if handlerErr := recover(); handlerErr != nil {
				err = kerrors.ErrPanic.WithCauseAndStack(fmt.Errorf("[happened in biz handler] %s", handlerErr), string(debug.Stack()))
				rpcStats := rpcinfo.AsMutableRPCStats(ri.Stats())
				rpcStats.SetPanicked(err)
			}
			rpcinfo.Record(ctx, ri, stats.ServerHandleFinish, err)
		}()
		implHandlerFunc := s.svcInfo.MethodInfo(methodName).Handler()
		rpcinfo.Record(ctx, ri, stats.ServerHandleStart, nil)
		err = implHandlerFunc(ctx, s.handler, args, resp)
		if err != nil {
			if bizErr, ok := kerrors.FromBizStatusError(err); ok {
				if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
					setter.SetBizStatusErr(bizErr)
					return nil
				}
			}
			err = kerrors.ErrBiz.WithCause(err)
		}
		return err
	}
}

func (s *server) initBasicRemoteOption() {
	remoteOpt := s.opt.RemoteOpt
	remoteOpt.SvcInfo = s.svcInfo
	remoteOpt.InitOrResetRPCInfoFunc = s.initOrResetRPCInfoFunc()
	remoteOpt.TracerCtl = s.opt.TracerCtl
	remoteOpt.ReadWriteTimeout = s.opt.Configs.ReadWriteTimeout()
}

func (s *server) richRemoteOption() {
	s.initBasicRemoteOption()

	s.addBoundHandlers(s.opt.RemoteOpt)
}

func (s *server) addBoundHandlers(opt *remote.ServerOption) {
	// add profiler meta handler, which should be exec after other MetaHandlers
	if opt.Profiler != nil && opt.ProfilerMessageTagging != nil {
		s.opt.MetaHandlers = append(s.opt.MetaHandlers,
			remote.NewProfilerMetaHandler(opt.Profiler, opt.ProfilerMessageTagging),
		)
	}
	// for server trans info handler
	if len(s.opt.MetaHandlers) > 0 {
		transInfoHdlr := bound.NewTransMetaHandler(s.opt.MetaHandlers)
		// meta handler exec before boundHandlers which add with option
		doAddBoundHandlerToHead(transInfoHdlr, opt)
		for _, h := range s.opt.MetaHandlers {
			if shdlr, ok := h.(remote.StreamingMetaHandler); ok {
				opt.StreamingMetaHandlers = append(opt.StreamingMetaHandlers, shdlr)
			}
		}
	}

	// for server limiter, the handler should be added as first one
	limitHdlr := s.buildLimiterWithOpt()
	if limitHdlr != nil {
		doAddBoundHandlerToHead(limitHdlr, opt)
	}
}

/*
 * There are two times when the rate limiter can take effect for a non-multiplexed server,
 * which are the OnRead and OnMessage callback. OnRead is called before request decoded
 * and OnMessage is called after.
 * Therefore, the optimization point is that we can make rate limiter take effect in OnRead as
 * possible to save computational cost of decoding.
 * The implementation is that when using the default rate limiter to launching a non-multiplexed
 * service, use the `serverLimiterOnReadHandler` whose rate limiting takes effect in the OnRead
 * callback.
 */
func (s *server) buildLimiterWithOpt() (handler remote.InboundHandler) {
	limits := s.opt.Limit.Limits
	connLimit := s.opt.Limit.ConLimit
	qpsLimit := s.opt.Limit.QPSLimit
	if limits == nil && connLimit == nil && qpsLimit == nil {
		return
	}
	if connLimit == nil {
		if limits != nil && limits.MaxConnections > 0 {
			connLimit = limiter.NewConnectionLimiter(limits.MaxConnections)
		} else {
			connLimit = &limiter.DummyConcurrencyLimiter{}
		}
	}
	if qpsLimit == nil {
		if limits != nil && limits.MaxQPS > 0 {
			interval := time.Millisecond * 100 // FIXME: should not care this implementation-specific parameter
			qpsLimit = limiter.NewQPSLimiter(interval, limits.MaxQPS)
		} else {
			qpsLimit = &limiter.DummyRateLimiter{}
		}
	} else {
		s.opt.Limit.QPSLimitPostDecode = true
	}
	if limits != nil && limits.UpdateControl != nil {
		updater := limiter.NewLimiterWrapper(connLimit, qpsLimit)
		limits.UpdateControl(updater)
	}
	handler = bound.NewServerLimiterHandler(connLimit, qpsLimit, s.opt.Limit.LimitReporter, s.opt.Limit.QPSLimitPostDecode)
	// TODO: gRPC limiter
	return
}

func (s *server) check() error {
	if s.svcInfo == nil {
		return errors.New("run: no service. Use RegisterService to set one")
	}
	if s.handler == nil || reflect.ValueOf(s.handler).IsNil() {
		return errors.New("run: handler is nil")
	}
	return nil
}

func doAddBoundHandlerToHead(h remote.BoundHandler, opt *remote.ServerOption) {
	add := false
	if ih, ok := h.(remote.InboundHandler); ok {
		handlers := []remote.InboundHandler{ih}
		opt.Inbounds = append(handlers, opt.Inbounds...)
		add = true
	}
	if oh, ok := h.(remote.OutboundHandler); ok {
		handlers := []remote.OutboundHandler{oh}
		opt.Outbounds = append(handlers, opt.Outbounds...)
		add = true
	}
	if !add {
		panic("invalid BoundHandler: must implement InboundHandler or OutboundHandler")
	}
}

func doAddBoundHandler(h remote.BoundHandler, opt *remote.ServerOption) {
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
		panic("invalid BoundHandler: must implement InboundHandler or OutboundHandler")
	}
}

func (s *server) newSvrTransHandler() (handler remote.ServerTransHandler, err error) {
	transHdlrFactory := s.opt.RemoteOpt.SvrHandlerFactory
	transHdlr, err := transHdlrFactory.NewTransHandler(s.opt.RemoteOpt)
	if err != nil {
		return nil, err
	}
	if setter, ok := transHdlr.(remote.InvokeHandleFuncSetter); ok {
		setter.SetInvokeHandleFunc(s.eps)
	}
	transPl := remote.NewTransPipeline(transHdlr)

	for _, ib := range s.opt.RemoteOpt.Inbounds {
		transPl.AddInboundHandler(ib)
	}
	for _, ob := range s.opt.RemoteOpt.Outbounds {
		transPl.AddOutboundHandler(ob)
	}
	return transPl, nil
}

func (s *server) buildRegistryInfo(lAddr net.Addr) {
	if s.opt.RegistryInfo == nil {
		s.opt.RegistryInfo = &registry.Info{}
	}
	info := s.opt.RegistryInfo
	// notice: lAddr may be nil when listen failed
	info.Addr = lAddr
	if info.ServiceName == "" {
		info.ServiceName = s.opt.Svr.ServiceName
	}
	if info.PayloadCodec == "" {
		info.PayloadCodec = s.opt.RemoteOpt.SvcInfo.PayloadCodec.String()
	}
	if info.Weight == 0 {
		info.Weight = discovery.DefaultWeight
	}
}

func (s *server) fillMoreServiceInfo(lAddr net.Addr) {
	ni := *s.svcInfo
	si := &ni
	extra := make(map[string]interface{}, len(si.Extra)+2)
	for k, v := range si.Extra {
		extra[k] = v
	}
	extra["address"] = lAddr
	extra["transports"] = s.opt.SupportedTransportsFunc(*s.opt.RemoteOpt)
	si.Extra = extra
	s.svcInfo = si
}

func (s *server) waitExit(errCh chan error) error {
	exitSignal := s.opt.ExitSignal()

	// service may not be available as soon as startup.
	delayRegister := time.After(1 * time.Second)
	for {
		select {
		case err := <-exitSignal:
			return err
		case err := <-errCh:
			return err
		case <-delayRegister:
			s.Lock()
			if err := s.opt.Registry.Register(s.opt.RegistryInfo); err != nil {
				s.Unlock()
				return err
			}
			s.Unlock()
		}
	}
}
