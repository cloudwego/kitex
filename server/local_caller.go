/*
 * Copyright 2024 CloudWeGo Authors
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

package server

import (
	"context"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/cloudwego/kitex/pkg/consts"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

// localCallerAddr is a synthetic address used as the "From" endpoint in RPCInfo,
// since there is no real network connection for in-process calls.
var localCallerAddr = utils.NewNetAddr("tcp", "127.0.0.1:0")

// LocalCaller calls a registered unary Kitex service handler in-process through
// the normal server unary middleware chain (timeout, ACL, user MWs, core MW).
// No network connection or codec is involved.
// Only non-streaming methods present in ServiceInfo.Methods are supported.
// Services that rely on ServiceInfo.GenericMethod are not supported.
//
// In a normal Kitex server, pkg/remote/trans (the transport layer) reads bytes
// from the network, decodes them into args, calls the middleware chain s.eps(ctx, args, result),
// encodes the result, and writes bytes back. LocalCaller replaces that entire transport
// layer by calling s.eps directly with the caller-provided args/result objects,
// skipping all network I/O and serialization.
//
// method accepts two forms:
//   - "ServiceName/MethodName": resolves the service by name
//   - "MethodName": allowed when the target can be inferred by the same
//     method-only lookup used for service-less server traffic
//     (fallback service first, otherwise the matching service)
//
// args and result must be the generated arg/result wrapper types from kitex_gen
// for the resolved unary method.
type LocalCaller interface {
	// Call resolves method and invokes the unary handler through the normal
	// server middleware chain using the caller-provided generated args/result
	// wrapper values.
	//
	// method accepts the same two forms as ResolveMethod:
	//   - "ServiceName/MethodName"
	//   - "MethodName", when method-only lookup can infer the target service
	//
	// args must be the generated wrapper type returned by MethodInfo.NewArgs()
	// for the resolved method. For non-oneway methods, result must be the
	// generated wrapper type returned by MethodInfo.NewResult().
	//
	// Callers that need to validate the method string or allocate correctly
	// typed wrapper values ahead of time can use ResolveMethod first, then
	// construct args/result from the returned MethodInfo.
	Call(ctx context.Context, method string, args, result any) error

	// ResolveMethod looks up the method info for the given method string.
	//
	// method accepts the same two forms as Call: "ServiceName/MethodName" or "MethodName".
	// ctx is currently unused. LocalCaller resolves only from ServiceInfo.Methods,
	// so services that depend on ServiceInfo.GenericMethod are not supported.
	//
	// It returns the resolved method descriptor, or an error if the service or
	// method cannot be resolved.
	ResolveMethod(ctx context.Context, method string) (serviceinfo.MethodInfo, error)
}

// cachedResult stores a successful method resolution so repeated
// lookups for the same method string are O(1) after the first call.
type cachedResult struct {
	methodName string
	svcInfo    *serviceinfo.ServiceInfo
	mi         serviceinfo.MethodInfo
	argsType   reflect.Type
	resultType reflect.Type // nil for oneway methods
}

func newCachedResult(methodName string, svcInfo *serviceinfo.ServiceInfo, mi serviceinfo.MethodInfo) *cachedResult {
	cr := &cachedResult{
		methodName: methodName,
		svcInfo:    svcInfo,
		mi:         mi,
		argsType:   reflect.TypeOf(mi.NewArgs()),
	}
	if !mi.OneWay() {
		cr.resultType = reflect.TypeOf(mi.NewResult())
	}
	return cr
}

type localCaller struct {
	svr      *server
	traceCtl *rpcinfo.TraceController
	caller   string
	cache    sync.Map // method string -> *cachedResult
}

// NewLocalCaller creates a LocalCaller that routes calls through svr's middleware chain.
// caller is the service name of the calling service, used as the "From" endpoint
// in RPCInfo (equivalent to the remote service name seen by server-side middleware).
// The server does NOT need to be running (Run() is not required).
func NewLocalCaller(caller string, svr Server) (LocalCaller, error) {
	s, ok := svr.(*server)
	if !ok {
		return nil, fmt.Errorf("NewLocalCaller: unsupported Server implementation %T", svr)
	}
	if caller == "" {
		return nil, fmt.Errorf("NewLocalCaller: caller must not be empty")
	}
	// init builds the middleware chain (s.eps); must be done under lock
	// since init mutates server state, but is idempotent after the first call.
	s.Lock()
	s.init()
	s.Unlock()
	if err := s.check(); err != nil {
		return nil, err
	}
	if len(s.svcs.knownSvcMap) == 0 {
		return nil, fmt.Errorf("NewLocalCaller: no known services registered; unknown-service-only setups are not supported")
	}
	traceCtl := s.opt.TracerCtl
	if traceCtl == nil {
		traceCtl = &rpcinfo.TraceController{}
	}

	return &localCaller{
		svr:      s,
		traceCtl: traceCtl,
		caller:   caller,
	}, nil
}

func (lc *localCaller) Call(ctx context.Context, method string, args, result any) (retErr error) {
	cr, err := lc.resolve(method)
	if err != nil {
		return err
	}
	svcInfo, mi, methodName := cr.svcInfo, cr.mi, cr.methodName

	if sm := mi.StreamingMode(); sm != serviceinfo.StreamingNone {
		return fmt.Errorf("LocalCaller: streaming method %q is not supported (mode=%d); only non-streaming unary methods are allowed", method, sm)
	}

	// validate args/result types using cached reflect.Type (zero alloc)
	if reflect.TypeOf(args) != cr.argsType {
		return fmt.Errorf("LocalCaller: args type mismatch for method %q: got %T, want %s", method, args, cr.argsType)
	}
	if cr.resultType != nil && reflect.TypeOf(result) != cr.resultType {
		return fmt.Errorf("LocalCaller: result type mismatch for method %q: got %T, want %s", method, result, cr.resultType)
	}

	// Extract the caller's method from ctx before we overwrite it.
	callerMethod := "unknown"
	if m, _ := ctx.Value(consts.CtxKeyMethod).(string); m != "" {
		callerMethod = m
	}

	ri := lc.newLocalRPCInfo(svcInfo, mi, methodName, callerMethod)
	// Reuse the caller's context directly - preserving its values, deadline, and cancellation
	// and only layer per-call RPC metadata onto it.
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	//nolint:staticcheck
	ctx = context.WithValue(ctx, consts.CtxKeyMethod, methodName)
	ctx = lc.traceCtl.DoStart(ctx, ri)
	var callErr error

	defer func() {
		if panicErr := recover(); panicErr != nil {
			wrapped := kerrors.ErrPanic.WithCauseAndStack(
				fmt.Errorf("[happened in LocalCaller, method=%s.%s] %v", svcInfo.ServiceName, methodName, panicErr),
				string(debug.Stack()),
			)
			rpcinfo.AsMutableRPCStats(ri.Stats()).SetPanicked(wrapped)
			callErr = wrapped
			retErr = wrapped
		}
		lc.traceCtl.DoFinish(ctx, ri, callErr)
		rpcinfo.PutRPCInfo(ri)
	}()

	callErr = lc.svr.eps(ctx, args, result)
	if callErr != nil {
		return callErr
	}
	if bizErr := ri.Invocation().BizStatusErr(); bizErr != nil {
		// Biz status errors are not returned by the endpoint chain (it returns nil).
		// They are stashed in RPCInfo by invokeHandleEndpoint and must be surfaced here,
		// matching what a normal Kitex client would observe.
		retErr = bizErr
	}
	return retErr
}

func (lc *localCaller) ResolveMethod(_ context.Context, method string) (serviceinfo.MethodInfo, error) {
	cr, err := lc.resolve(method)
	if err != nil {
		return nil, err
	}
	return cr.mi, nil
}

// resolve returns the cached result for the given method string, resolving on first call.
func (lc *localCaller) resolve(method string) (*cachedResult, error) {
	if v, ok := lc.cache.Load(method); ok {
		return v.(*cachedResult), nil
	}
	cr, err := lc.doResolve(method)
	if err != nil {
		return nil, err
	}
	lc.cache.Store(method, cr)
	return cr, nil
}

func (lc *localCaller) doResolve(method string) (*cachedResult, error) {
	if idx := strings.IndexByte(method, '/'); idx >= 0 {
		// "ServiceName/MethodName" form
		svcName := method[:idx]
		methodName := method[idx+1:]
		if svcName == "" {
			return nil, fmt.Errorf("LocalCaller: empty service name in %q", method)
		}
		if methodName == "" {
			return nil, fmt.Errorf("LocalCaller: empty method name in %q", method)
		}
		svc := lc.svr.svcs.getService(svcName)
		if svc == nil {
			return nil, fmt.Errorf("LocalCaller: service %q not found", svcName)
		}
		mi := svc.svcInfo.Methods[methodName]
		if mi == nil {
			if svc.svcInfo.GenericMethod != nil {
				return nil, fmt.Errorf("LocalCaller: service %q uses ServiceInfo.GenericMethod, which is not supported", svcName)
			}
			return nil, fmt.Errorf("LocalCaller: method %q not found in service %q", methodName, svcName)
		}
		return newCachedResult(methodName, svc.svcInfo, mi), nil
	}

	// Short form: bare method names should resolve the same way server-side
	// service-less traffic does, including preferring the fallback service when
	// duplicate method names are intentionally routed there.
	svcInfo := lc.svr.svcs.searchByMethodName(method)
	if svcInfo == nil {
		return nil, fmt.Errorf("LocalCaller: method %q not found in any registered service", method)
	}
	mi := svcInfo.Methods[method]
	if mi == nil {
		if svcInfo.GenericMethod != nil {
			return nil, fmt.Errorf("LocalCaller: service %q resolves method %q via ServiceInfo.GenericMethod, which is not supported", svcInfo.ServiceName, method)
		}
		return nil, fmt.Errorf("LocalCaller: method %q not found in any registered service", method)
	}
	return newCachedResult(method, svcInfo, mi), nil
}

// newLocalRPCInfo allocates a fresh per-call RPCInfo following the same pattern as
// the normal server-side allocation in initOrResetRPCInfoFunc (server.go).
func (lc *localCaller) newLocalRPCInfo(svcInfo *serviceinfo.ServiceInfo, mi serviceinfo.MethodInfo, methodName, callerMethod string) rpcinfo.RPCInfo {
	s := lc.svr
	rpcStats := rpcinfo.AsMutableRPCStats(rpcinfo.NewRPCStats())
	if s.opt.StatsLevel != nil {
		rpcStats.SetLevel(*s.opt.StatsLevel)
	}
	from := rpcinfo.NewMutableEndpointInfo(lc.caller, callerMethod, localCallerAddr, nil)
	ri := rpcinfo.NewRPCInfo(
		from.ImmutableView(),
		rpcinfo.FromBasicInfo(s.opt.Svr),
		rpcinfo.NewServerInvocation(),
		rpcinfo.AsMutableRPCConfig(s.opt.Configs).Clone().ImmutableView(),
		rpcStats.ImmutableView(),
	)
	rpcinfo.AsMutableEndpointInfo(ri.To()).SetMethod(methodName)

	if setter, ok := ri.Invocation().(rpcinfo.InvocationSetter); ok {
		setter.SetPackageName(svcInfo.GetPackageName())
		setter.SetServiceName(svcInfo.ServiceName)
		setter.SetMethodName(methodName)
		setter.SetMethodInfo(mi)
	}
	return ri
}
