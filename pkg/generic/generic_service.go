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

	"github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
)

// Service generic service interface
type Service interface {
	// GenericCall handle the generic call
	GenericCall(ctx context.Context, method string, request interface{}) (response interface{}, err error)
}

type ServiceV2 interface {
	// GenericCall handle the generic call
	GenericCall(ctx context.Context, service, method string, request interface{}) (response interface{}, err error)
	// ClientStreaming
	ClientStreaming(ctx context.Context, service, method string, stream streaming.ServerStream) (err error)
	// ServerStreaming
	ServerStreaming(ctx context.Context, service, method string, request interface{}, stream streaming.ServerStream) (err error)
	// BidiStreaming
	BidiStreaming(ctx context.Context, service, method string, stream streaming.ServerStream) (err error)
}

// ServiceInfoWithGeneric create a generic ServiceInfo
func ServiceInfoWithGeneric(g Generic) *serviceinfo.ServiceInfo {
	handlerType := (*Service)(nil)

	methods, genericMethod := g.Methods()

	svcInfo := &serviceinfo.ServiceInfo{
		ServiceName:   g.IDLServiceName(),
		HandlerType:   handlerType,
		Methods:       methods,
		PayloadCodec:  g.PayloadCodecType(),
		Extra:         make(map[string]interface{}),
		GenericMethod: genericMethod,
	}
	svcInfo.Extra["generic"] = true
	if extra, ok := g.(ExtraProvider); ok {
		if extra.GetExtra(CombineServiceKey) == "true" {
			svcInfo.Extra[CombineServiceKey] = true
		}
		if pkg := extra.GetExtra(packageNameKey); pkg != "" {
			svcInfo.Extra[packageNameKey] = pkg
		}
	}
	return svcInfo
}

func callHandler(ctx context.Context, handler, arg, result interface{}) error {
	realArg := arg.(*Args)
	realResult := result.(*Result)
	if svcv2, ok := handler.(ServiceV2); ok {
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()
		serviceName := ri.Invocation().ServiceName()
		success, err := svcv2.GenericCall(ctx, serviceName, methodName, realArg.Request)
		if err != nil {
			return err
		}
		realResult.Success = success
		return nil
	}
	success, err := handler.(Service).GenericCall(ctx, realArg.Method, realArg.Request)
	if err != nil {
		return err
	}
	realResult.Success = success
	return nil
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
			methodInfo:   mi,
			ServerStream: st,
		}
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()
		serviceName := ri.Invocation().ServiceName()
		return handler.(ServiceV2).ClientStreaming(ctx, serviceName, methodName, gst)
	}
}

func serverStreamingHandlerGetter(mi serviceinfo.MethodInfo) serviceinfo.MethodHandler {
	return func(ctx context.Context, handler, arg, result interface{}) error {
		st, err := streaming.GetServerStreamFromArg(arg)
		if err != nil {
			return err
		}
		gst := &serverStreamingServer{
			methodInfo:   mi,
			ServerStream: st,
		}
		args := mi.NewArgs().(*Args)
		if err = st.RecvMsg(ctx, args); err != nil {
			return err
		}
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()
		serviceName := ri.Invocation().ServiceName()
		return handler.(ServiceV2).ServerStreaming(ctx, serviceName, methodName, args.Request, gst)
	}
}

func bidiStreamingHandlerGetter(mi serviceinfo.MethodInfo) serviceinfo.MethodHandler {
	return func(ctx context.Context, handler, arg, result interface{}) error {
		st, err := streaming.GetServerStreamFromArg(arg)
		if err != nil {
			return err
		}
		gst := &bidiStreamingServer{
			methodInfo:   mi,
			ServerStream: st,
		}
		ri := rpcinfo.GetRPCInfo(ctx)
		methodName := ri.Invocation().MethodName()
		serviceName := ri.Invocation().ServiceName()
		return handler.(ServiceV2).BidiStreaming(ctx, serviceName, methodName, gst)
	}
}

// WithCodec set codec instance for Args or Result
type WithCodec interface {
	SetCodec(codec interface{})
}

// Args generic request
type Args = generic.Args

// Result generic response
type Result = generic.Result

var (
	_ WithCodec = (*Args)(nil)
	_ WithCodec = (*Result)(nil)
)

func getGenericStreamingMethodInfoKey(streamingMode serviceinfo.StreamingMode) string {
	switch streamingMode {
	case serviceinfo.StreamingClient:
		return serviceinfo.GenericClientStreamingMethod
	case serviceinfo.StreamingServer:
		return serviceinfo.GenericServerStreamingMethod
	case serviceinfo.StreamingBidirectional:
		return serviceinfo.GenericBidirectionalStreamingMethod
	default:
		return serviceinfo.GenericMethod
	}
}
