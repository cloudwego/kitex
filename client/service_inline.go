/*
 * Copyright 2023 CloudWeGo Authors
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
	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

var localAddr net.Addr

func init() {
	localAddr = utils.NewNetAddr("tcp", "127.0.0.1")
}

type ContextServiceInlineHandler interface {
	WriteMeta(cliCtx, svrCtx context.Context, req interface{}) (newSvrCtx context.Context, err error)
	ReadMeta(cliCtx, svrCtx context.Context, resp interface{}) (newCliCtx context.Context, err error)
}

type serviceInlineClient struct {
	svcInfo *serviceinfo.ServiceInfo
	mws     []endpoint.Middleware
	eps     endpoint.Endpoint
	opt     *client.Options

	inited bool
	closed bool

	// server info
	serverEps endpoint.Endpoint
	serverOpt *internal_server.Options

	contextServiceInlineHandler ContextServiceInlineHandler
}

type ServerInitialInfo interface {
	Endpoints() endpoint.Endpoint
	Option() *internal_server.Options
	GetServiceInfo() *serviceinfo.ServiceInfo
}

// NewServiceInlineClient creates a kitex.Client with the given ServiceInfo, it is from generated code.
func NewServiceInlineClient(svcInfo *serviceinfo.ServiceInfo, s ServerInitialInfo, opts ...Option) (Client, error) {
	if svcInfo == nil {
		return nil, errors.New("NewClient: no service info")
	}
	kc := &serviceInlineClient{}
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

func (kc *serviceInlineClient) SetContextServiceInlineHandler(simh ContextServiceInlineHandler) {
	kc.contextServiceInlineHandler = simh
}

func (kc *serviceInlineClient) init() (err error) {
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

func (kc *serviceInlineClient) checkOptions() (err error) {
	if kc.opt.Svr.ServiceName == "" {
		return errors.New("service name is required")
	}
	return nil
}

func (kc *serviceInlineClient) initContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, endpoint.CtxEventBusKey, kc.opt.Bus)
	ctx = context.WithValue(ctx, endpoint.CtxEventQueueKey, kc.opt.Events)
	return ctx
}

func (kc *serviceInlineClient) initMiddlewares(ctx context.Context) {
	builderMWs := richMWsWithBuilder(ctx, kc.opt.MWBs)
	kc.mws = append(kc.mws, contextMW)
	kc.mws = append(kc.mws, builderMWs...)
}

// initRPCInfo initializes the RPCInfo structure and attaches it to context.
func (kc *serviceInlineClient) initRPCInfo(ctx context.Context, method string) (context.Context, rpcinfo.RPCInfo, *callopt.CallOptions) {
	iriv := initRpcInfoVar{svcInfo: kc.svcInfo, opt: kc.opt}
	return initRpcInfo(ctx, method, iriv)
}

// Call implements the Client interface .
func (kc *serviceInlineClient) Call(ctx context.Context, method string, request, response interface{}) error {
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
	if err == nil {
		err = ri.Invocation().BizStatusErr()
	}
	rpcinfo.PutRPCInfo(ri)
	return err
}

func (kc *serviceInlineClient) initDebugService() {
	cdi := clientDebugInfo{
		serviceName: kc.opt.Svr.ServiceName,
		debugInfo:   kc.opt.DebugInfo,
		evt:         kc.opt.Events,
		svcInfo:     kc.svcInfo,
	}
	initDebugService(kc.opt.DebugService, cdi)
}

func (kc *serviceInlineClient) richRemoteOption() {
	kc.opt.RemoteOpt.SvcInfo = kc.svcInfo
}

func (kc *serviceInlineClient) buildInvokeChain() error {
	innerHandlerEp, err := kc.invokeHandleEndpoint()
	if err != nil {
		return err
	}
	kc.eps = endpoint.Chain(kc.mws...)(innerHandlerEp)
	return nil
}

func (kc *serviceInlineClient) constructServerCtxWithMetadata(cliCtx context.Context) (serverCtx context.Context) {
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

func (kc *serviceInlineClient) constructServerRpcInfo(svrCtx, cliCtx context.Context) (newServerCtx context.Context, svrRpcInfo rpcinfo.RPCInfo) {
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
	svrCtx = rpcinfo.NewCtxWithRPCInfo(svrCtx, ri)

	cliRpcInfo := rpcinfo.GetRPCInfo(cliCtx)
	// handle common rpcinfo
	method := cliRpcInfo.To().Method()
	if ink, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
		ink.SetMethodName(method)
		ink.SetServiceName(kc.svcInfo.ServiceName)
	}
	rpcinfo.AsMutableEndpointInfo(ri.To()).SetMethod(method)
	return svrCtx, ri
}

func (kc *serviceInlineClient) invokeHandleEndpoint() (endpoint.Endpoint, error) {
	svrTraceCtl := kc.serverOpt.TracerCtl
	if svrTraceCtl == nil {
		svrTraceCtl = &stats.Controller{}
	}

	return func(ctx context.Context, req, resp interface{}) (err error) {
		serverCtx := kc.constructServerCtxWithMetadata(ctx)
		defer func() {
			// backward key
			kvs := metainfo.AllBackwardValuesToSend(serverCtx)
			if len(kvs) > 0 {
				metainfo.SetBackwardValuesFromMap(ctx, kvs)
			}
		}()
		serverCtx, svrRpcinfo := kc.constructServerRpcInfo(serverCtx, ctx)
		defer func() {
			rpcinfo.PutRPCInfo(svrRpcinfo)
		}()

		// server trace
		serverCtx = svrTraceCtl.DoStart(serverCtx, svrRpcinfo)

		if kc.contextServiceInlineHandler != nil {
			serverCtx, err = kc.contextServiceInlineHandler.WriteMeta(ctx, serverCtx, req)
			if err != nil {
				return err
			}
		}

		// server logic
		err = kc.serverEps(serverCtx, req, resp)
		// finish server trace
		// contextServiceInlineHandler may convert nil err to non nil err, so handle trace here
		svrTraceCtl.DoFinish(serverCtx, svrRpcinfo, err)

		if kc.contextServiceInlineHandler != nil {
			var err1 error
			ctx, err1 = kc.contextServiceInlineHandler.ReadMeta(ctx, serverCtx, resp)
			if err1 != nil {
				return err1
			}
		}
		return err
	}, nil
}

// Close is not concurrency safe.
func (kc *serviceInlineClient) Close() error {
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
	if errs.HasError() {
		return errs
	}
	return nil
}
