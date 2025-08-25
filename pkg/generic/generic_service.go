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

package generic

import (
	"context"
	"errors"
	"fmt"

	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

var (
	errGenericCallNotImplemented     = errors.New("generic.ServiceV2 GenericCall not implemented")
	errClientStreamingNotImplemented = errors.New("generic.ServiceV2 ClientStreaming not implemented")
	errServerStreamingNotImplemented = errors.New("generic.ServiceV2 ServerStreaming not implemented")
	errBidiStreamingNotImplemented   = errors.New("generic.ServiceV2 BidiStreaming not implemented")
)

// Service is the v1 generic service interface.
type Service interface {
	// GenericCall handle the generic call
	GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error)
}

// ServiceV2 is the new generic service interface, provides methods to handle streaming requests
// and it also supports multi services. All methods are optional.
type ServiceV2 struct {
	// GenericCall handles pingpong/unary requests.
	//
	// NOTE: If the generic type is BinaryThriftGenericV2, GRPC unary requests won't be handled by this method.
	// Instead, they will be handled by BidiStreaming.
	GenericCall func(ctx context.Context, service, method string, request interface{}) (response interface{}, err error)

	// ClientStreaming handles client streaming call.
	//
	// NOTE: If the generic type is BinaryThriftGenericV2, Client streaming requests won't be handled by this method.
	// Instead, they will be handled by BidiStreaming.
	ClientStreaming func(ctx context.Context, service, method string, stream ClientStreamingServer) (err error)

	// ServerStreaming handles server streaming call.
	//
	// NOTE: If the generic type is BinaryThriftGenericV2, Server streaming requests won't be handled by this method.
	// Instead, they will be handled by BidiStreaming.
	ServerStreaming func(ctx context.Context, service, method string, request interface{}, stream ServerStreamingServer) (err error)

	// BidiStreaming handles the bidi streaming call.
	//
	// NOTE: If the generic type is BinaryThriftGenericV2, all streaming requests (including GRPC unary) will be handled
	// by this method. Since we cannot determine the stream mode without IDL info, we can only treat the requests
	// as bidi streaming by default.
	BidiStreaming func(ctx context.Context, service, method string, stream BidiStreamingServer) (err error)
}

// ServiceInfoWithGeneric create a generic ServiceInfo from Generic.
func ServiceInfoWithGeneric(g Generic) *serviceinfo.ServiceInfo {
	handlerType := (*Service)(nil)

	svcInfo := &serviceinfo.ServiceInfo{
		ServiceName:   g.IDLServiceName(),
		HandlerType:   handlerType,
		Methods:       make(map[string]serviceinfo.MethodInfo),
		PayloadCodec:  g.PayloadCodecType(),
		Extra:         make(map[string]interface{}),
		GenericMethod: g.GenericMethod(),
	}
	svcInfo.Extra["generic"] = true
	if combineService, _ := g.GetExtra(serviceinfo.CombineServiceKey).(bool); combineService {
		svcInfo.Extra[serviceinfo.CombineServiceKey] = true
	}
	if pkg, _ := g.GetExtra(serviceinfo.PackageName).(string); pkg != "" {
		svcInfo.Extra[serviceinfo.PackageName] = pkg
	}
	if isBinaryGeneric, _ := g.GetExtra(IsBinaryGeneric).(bool); isBinaryGeneric {
		svcInfo.Extra[IsBinaryGeneric] = true
	}
	return svcInfo
}

func callHandler(ctx context.Context, handler, arg, result interface{}) error {
	realArg := arg.(*Args)
	realResult := result.(*Result)
	switch svc := handler.(type) {
	case *ServiceV2:
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()
		serviceName := ri.Invocation().ServiceName()
		if svc.GenericCall == nil {
			return errGenericCallNotImplemented
		}
		success, err := svc.GenericCall(ctx, serviceName, methodName, realArg.Request)
		if err != nil {
			return err
		}
		realResult.Success = success
		return nil
	case Service:
		success, err := handler.(Service).GenericCall(ctx, realArg.Method, realArg.Request)
		if err != nil {
			return err
		}
		realResult.Success = success
		return nil
	default:
		return fmt.Errorf("CallHandler: unknown handler type %T", handler)
	}
}

func newGenericServiceCallArgs() interface{} {
	return &Args{}
}

func newGenericServiceCallResult() interface{} {
	return &Result{}
}

func clientStreamingHandlerGetter(mi serviceinfo.MethodInfo) serviceinfo.MethodHandler {
	return func(ctx context.Context, handler, arg, result interface{}) error {
		st, err := streaming.GetServerStreamFromArg(arg)
		if err != nil {
			return err
		}
		gst := &clientStreamingServer{
			methodInfo: mi,
			streaming:  st,
		}
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()
		serviceName := ri.Invocation().ServiceName()
		svcv2 := handler.(*ServiceV2)
		if svcv2.ClientStreaming == nil {
			return errClientStreamingNotImplemented
		}
		return svcv2.ClientStreaming(ctx, serviceName, methodName, gst)
	}
}

func serverStreamingHandlerGetter(mi serviceinfo.MethodInfo) serviceinfo.MethodHandler {
	return func(ctx context.Context, handler, arg, result interface{}) error {
		st, err := streaming.GetServerStreamFromArg(arg)
		if err != nil {
			return err
		}
		gst := &serverStreamingServer{
			methodInfo: mi,
			streaming:  st,
		}
		args := mi.NewArgs().(*Args)
		if err = st.RecvMsg(ctx, args); err != nil {
			return err
		}
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()
		serviceName := ri.Invocation().ServiceName()
		svcv2 := handler.(*ServiceV2)
		if svcv2.ServerStreaming == nil {
			return errServerStreamingNotImplemented
		}
		return svcv2.ServerStreaming(ctx, serviceName, methodName, args.Request, gst)
	}
}

func bidiStreamingHandlerGetter(mi serviceinfo.MethodInfo) serviceinfo.MethodHandler {
	return func(ctx context.Context, handler, arg, result interface{}) error {
		st, err := streaming.GetServerStreamFromArg(arg)
		if err != nil {
			return err
		}
		gst := &bidiStreamingServer{
			methodInfo: mi,
			streaming:  st,
		}
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()
		serviceName := ri.Invocation().ServiceName()
		switch svc := handler.(type) {
		case *ServiceV2:
			if svc.BidiStreaming == nil {
				return errBidiStreamingNotImplemented
			}
			return svc.BidiStreaming(ctx, serviceName, methodName, gst)
		default:
			return fmt.Errorf("BidiStreamingHandler: unknown handler type %T", handler)
		}
	}
}

// WithCodec set codec instance for Args or Result
type WithCodec interface {
	SetCodec(codec interface{})
}

// Args generic request
type Args = igeneric.Args

// Result generic response
type Result = igeneric.Result

var (
	_ WithCodec = (*Args)(nil)
	_ WithCodec = (*Result)(nil)
)
