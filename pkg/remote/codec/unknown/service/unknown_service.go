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

package unknown

import (
	"context"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

const (
	// UnknownService name
	UnknownService = "$UnknownService" // private as "$"
	// UnknownCall name
	UnknownMethod = "$UnknownCall"
)

type Args struct {
	Request     []byte
	Method      string
	ServiceName string
}

type Result struct {
	Success     []byte
	Method      string
	ServiceName string
}

type UnknownMethodService interface {
	UnknownMethodHandler(ctx context.Context, method string, request []byte) ([]byte, error)
}

// NewServiceInfo create   serviceInfo
func NewServiceInfo(pcType serviceinfo.PayloadCodec, service, method string) *serviceinfo.ServiceInfo {
	methods := map[string]serviceinfo.MethodInfo{
		method: serviceinfo.NewMethodInfo(callHandler, newServiceArgs, newServiceResult, false),
	}
	handlerType := (*UnknownMethodService)(nil)

	svcInfo := &serviceinfo.ServiceInfo{
		ServiceName:  service,
		HandlerType:  handlerType,
		Methods:      methods,
		PayloadCodec: pcType,
		Extra:        make(map[string]interface{}),
	}

	return svcInfo
}

func SetServiceInfo(svcInfo *serviceinfo.ServiceInfo, pcType serviceinfo.PayloadCodec, service, method string) {
	svcInfo.ServiceName = service
	svcInfo.PayloadCodec = pcType
	svcInfo.HandlerType = (*UnknownMethodService)(nil)
	methods := map[string]serviceinfo.MethodInfo{
		method: serviceinfo.NewMethodInfo(callHandler, newServiceArgs, newServiceResult, false),
	}
	svcInfo.Methods = methods

}

func callHandler(ctx context.Context, handler, arg, result interface{}) error {
	realArg := arg.(*Args)
	realResult := result.(*Result)
	realResult.Method = realArg.Method
	realResult.ServiceName = realArg.ServiceName
	success, err := handler.(UnknownMethodService).UnknownMethodHandler(ctx, realArg.Method, realArg.Request)
	if err != nil {
		return err
	}
	realResult.Success = success
	return nil
}

func newServiceArgs() interface{} {
	return &Args{}
}

func newServiceResult() interface{} {
	return &Result{}
}
