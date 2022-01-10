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
	internal_stats "github.com/cloudwego/kitex/internal/stats"
	"github.com/cloudwego/kitex/pkg/acl"
	"github.com/cloudwego/kitex/pkg/diagnosis"
	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/endpoint"
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
func newErrorHandleMW(errHandle func(error) error) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, request, response interface{}) error {
			err := next(ctx, request, response)
			if err == nil {
				return nil
			}
			return errHandle(err)
		}
	}
}

func (s *server) initRPCInfoFunc() func(context.Context, net.Addr) (rpcinfo.RPCInfo, context.Context) {
	return func(ctx context.Context, rAddr net.Addr) (rpcinfo.RPCInfo, context.Context) {
		if ctx == nil {
			ctx = context.Background()
		}
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
		rpcinfo.AsMutableEndpointInfo(ri.From()).SetAddress(rAddr)
		ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
		return ri, ctx
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

	errCh := s.svr.Start()
	for i := range onServerStart {
		go onServerStart[i]()
	}
	s.Lock()
	s.buildRegistryInfo(s.svr.Address())
	s.Unlock()

	if err = s.waitExit(errCh); err != nil {
		klog.Errorf("KITEX: received error and exit: error=%s", err.Error())
	}
	for i := range onShutdown {
		onShutdown[i]()
	}
	// stop server after user hooks
	if e := s.Stop(); e != nil && err == nil {
		err = e
		klog.Errorf("KITEX: stop server error: error=%s", e.Error())
	}
	return
}

// Stop stops the server gracefully.
func (s *server) Stop() (err error) {
	s.Lock()
	defer s.Unlock()
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
			internal_stats.Record(ctx, ri, stats.ServerHandleFinish, err)
		}()
		implHandlerFunc := s.svcInfo.MethodInfo(methodName).Handler()
		internal_stats.Record(ctx, ri, stats.ServerHandleStart, nil)
		err = implHandlerFunc(ctx, s.handler, args, resp)
		if err != nil {
			err = kerrors.ErrBiz.WithCause(err)
		}
		return err
	}
}

func (s *server) initBasicRemoteOption() {
	remoteOpt := s.opt.RemoteOpt
	remoteOpt.SvcInfo = s.svcInfo
	remoteOpt.InitRPCInfoFunc = s.initRPCInfoFunc()
	remoteOpt.TracerCtl = s.opt.TracerCtl
	remoteOpt.ReadWriteTimeout = s.opt.Configs.ReadWriteTimeout()
}

func (s *server) richRemoteOption() {
	s.initBasicRemoteOption()

	s.addBoundHandlers(s.opt.RemoteOpt)
}

func (s *server) addBoundHandlers(opt *remote.ServerOption) {
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
	connLimit, qpsLimit, ok := s.buildLimiterWithOpt()
	if ok {
		limitHdlr := bound.NewServerLimiterHandler(connLimit, qpsLimit, s.opt.LimitReporter)
		doAddBoundHandlerToHead(limitHdlr, opt)
	}
}

func (s *server) buildLimiterWithOpt() (connLimit limiter.ConcurrencyLimiter, qpsLimit limiter.RateLimiter, ok bool) {
	if s.opt.Limits != nil {
		if s.opt.Limits.MaxConnections > 0 {
			connLimit = limiter.NewConcurrencyLimiter(s.opt.Limits.MaxConnections)
		} else {
			connLimit = &limiter.DummyConcurrencyLimiter{}
		}
		if s.opt.Limits.MaxQPS > 0 {
			interval := time.Millisecond * 100 // FIXME: should not care this implementation-specific parameter
			qpsLimit = limiter.NewQPSLimiter(interval, s.opt.Limits.MaxQPS)
		} else {
			qpsLimit = &limiter.DummyRateLimiter{}
		}

		if s.opt.Limits.UpdateControl != nil {
			updater := limiter.NewLimiterWrapper(connLimit, qpsLimit)
			s.opt.Limits.UpdateControl(updater)
		}
		ok = true
	}
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
