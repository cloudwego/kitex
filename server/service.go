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

package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	igeneric "github.com/cloudwego/kitex/internal/generic"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// set generic streaming mode to -1, which means not allow method searching by binary generic.
var notAllowBinaryGenericCtx = igeneric.WithGenericStreamingMode(context.Background(), serviceinfo.StreamingMode(-1))

type service struct {
	svcInfo              *serviceinfo.ServiceInfo
	handler              interface{}
	unknownMethodHandler interface{}
}

func newService(svcInfo *serviceinfo.ServiceInfo, handler interface{}) *service {
	return &service{svcInfo: svcInfo, handler: handler}
}

func (s *service) getHandler(methodName string) interface{} {
	if s.unknownMethodHandler == nil {
		return s.handler
	}
	if mi := s.svcInfo.MethodInfo(notAllowBinaryGenericCtx, methodName); mi != nil {
		return s.handler
	}
	return s.unknownMethodHandler
}

type unknownService struct {
	mutex   sync.RWMutex
	svcs    map[string]*service
	handler interface{}
}

func (u *unknownService) getSvc(svcName string) *service {
	u.mutex.RLock()
	svc := u.svcs[svcName]
	u.mutex.RUnlock()
	return svc
}

func (u *unknownService) getOrStoreSvc(svcName string, codecType serviceinfo.PayloadCodec) *service {
	u.mutex.RLock()
	svc, ok := u.svcs[svcName]
	u.mutex.RUnlock()
	if ok {
		return svc
	}
	u.mutex.Lock()
	defer u.mutex.Unlock()
	svc, ok = u.svcs[svcName]
	if ok {
		return svc
	}
	var g generic.Generic
	switch codecType {
	case serviceinfo.Thrift:
		g = generic.BinaryThriftGenericV2(svcName)
	case serviceinfo.Protobuf:
		g = generic.BinaryPbGeneric(svcName, "")
	default:
		return nil
	}
	svc = &service{
		svcInfo: generic.ServiceInfoWithGeneric(g),
		handler: u.handler,
	}
	u.svcs[svcName] = svc
	return svc
}

type services struct {
	knownSvcMap map[string]*service // key: service name

	// fallbackSvc is to get the unique svcInfo
	// if there are multiple services which have the same method.
	fallbackSvc     *service
	nonFallbackSvcs []*service

	unknownSvc *unknownService

	// be compatible with binary thrift generic v1
	binaryThriftGenericV1SvcInfo *serviceinfo.ServiceInfo
	// be compatible with server combine service
	combineSvcInfo *serviceinfo.ServiceInfo

	refuseTrafficWithoutServiceName bool
}

func newServices() *services {
	return &services{
		knownSvcMap: map[string]*service{},
	}
}

func (s *services) addService(svcInfo *serviceinfo.ServiceInfo, handler interface{}, registerOpts *RegisterOptions) error {
	// unknown service
	if registerOpts.IsUnknownService {
		if s.unknownSvc != nil {
			return errors.New("multiple unknown services cannot be registered")
		}
		s.unknownSvc = &unknownService{svcs: map[string]*service{}, handler: handler}
		return nil
	}

	svc := newService(svcInfo, handler)
	if registerOpts.IsFallbackService {
		if s.fallbackSvc != nil {
			return fmt.Errorf("multiple fallback services cannot be registered. [%s] is already registered as a fallback service", s.fallbackSvc.svcInfo.ServiceName)
		}
		s.fallbackSvc = svc
	}
	if _, ok := s.knownSvcMap[svcInfo.ServiceName]; ok {
		return fmt.Errorf("service [%s] has already been registered", svcInfo.ServiceName)
	}
	s.knownSvcMap[svcInfo.ServiceName] = svc
	if !registerOpts.IsFallbackService {
		s.nonFallbackSvcs = append(s.nonFallbackSvcs, svc)
	}
	return nil
}

func (s *services) getKnownSvcInfoMap() map[string]*serviceinfo.ServiceInfo {
	svcInfoMap := map[string]*serviceinfo.ServiceInfo{}
	for name, svc := range s.knownSvcMap {
		svcInfoMap[name] = svc.svcInfo
	}
	return svcInfoMap
}

func (s *services) check(refuseTrafficWithoutServiceName bool) error {
	if len(s.knownSvcMap) == 0 && s.unknownSvc == nil {
		return errors.New("run: no service. Use RegisterService to set one")
	}
	for _, svc := range s.knownSvcMap {
		// special treatment to binary thrift generic v1, it doesn't support multi services and unknown service.
		if svc.svcInfo.ServiceName == serviceinfo.GenericService {
			s.binaryThriftGenericV1SvcInfo = svc.svcInfo
			if len(s.knownSvcMap) > 1 {
				return fmt.Errorf("binary thrift generic v1 doesn't support multi services")
			}
			if s.unknownSvc != nil {
				return fmt.Errorf("binary thrift generic v1 doesn't support unknown service")
			}
			return nil
		}
		// special treatment to combine service, it doesn't support multi services and unknown service.
		if isCombineService, _ := svc.svcInfo.Extra[serviceinfo.CombineServiceKey].(bool); isCombineService {
			s.combineSvcInfo = svc.svcInfo
			if len(s.knownSvcMap) > 1 {
				return fmt.Errorf("combine service doesn't support multi services")
			}
			if s.unknownSvc != nil {
				return fmt.Errorf("combine service doesn't support unknown service")
			}
			return nil
		}
	}
	if s.unknownSvc != nil {
		for _, svc := range s.knownSvcMap {
			// register binary fallback method for every service info
			if nSvcInfo := registerBinaryGenericMethodFunc(svc.svcInfo); nSvcInfo != nil {
				svc.svcInfo = nSvcInfo
				svc.unknownMethodHandler = s.unknownSvc.handler
			}
		}
	}
	if refuseTrafficWithoutServiceName {
		s.refuseTrafficWithoutServiceName = true
		return nil
	}
	// Checking whether conflicting methods have fallback service.
	// It can only check service info for generated code, but not for generic services such as json/map,
	// because Methods map of generic services are empty.
	fallbackCheckingMap := make(map[string]int)
	for _, svc := range s.knownSvcMap {
		for method := range svc.svcInfo.Methods {
			if svc == s.fallbackSvc {
				fallbackCheckingMap[method] = -1
			} else if num := fallbackCheckingMap[method]; num >= 0 {
				fallbackCheckingMap[method] = num + 1
			}
		}
	}
	for methodName, serviceNum := range fallbackCheckingMap {
		if serviceNum > 1 {
			return fmt.Errorf("method name [%s] is conflicted between services but no fallback service is specified", methodName)
		}
	}
	return nil
}

func (s *services) getService(svcName string) *service {
	if svc := s.knownSvcMap[svcName]; svc != nil {
		return svc
	}
	if s.unknownSvc != nil {
		return s.unknownSvc.getSvc(svcName)
	}
	return nil
}

func (s *services) searchByMethodName(methodName string) *serviceinfo.ServiceInfo {
	if s.fallbackSvc != nil {
		// check whether fallback service has the method
		if mi := s.fallbackSvc.svcInfo.MethodInfo(notAllowBinaryGenericCtx, methodName); mi != nil {
			return s.fallbackSvc.svcInfo
		}
	}
	// match other services
	for _, svc := range s.nonFallbackSvcs {
		if mi := svc.svcInfo.MethodInfo(notAllowBinaryGenericCtx, methodName); mi != nil {
			return svc.svcInfo
		}
	}
	return nil
}

// SearchService searches for a service by service/method name, also requires strict and codecType params.
//
//   - For gRPC, the request header will always carry the service + method name, so the strict setting is set to true,
//     and the service is looked up by the svc name only;
//
//   - For Thrift, when Kitex is below v0.9.0 or the THeader protocol is not enabled, requests do not carry the service name.
//     In this case, it is necessary to attempt to find the possible service through the method name.
//     Method lookup must disable binary lookup to prevent hitting the binary generic service.
//
//   - For unknown service, we dynamically create a new service info and store it in the unknown service map.
//
//   - Corner case handling:
//     1. Thrift binary generic v1 or combine service does not support multi services or unknown service, so can directly return;
//     2. In Kitex v0.9.0-v0.9.1, there was a bug that caused requests to carry incorrect service names ($GenericService and CombineService),
//     requiring additional lookup via the method.
func (s *services) SearchService(svcName, methodName string, strict bool, codecType serviceinfo.PayloadCodec) *serviceinfo.ServiceInfo {
	if strict || s.refuseTrafficWithoutServiceName {
		if svc := s.knownSvcMap[svcName]; svc != nil {
			return svc.svcInfo
		}
	} else {
		if s.binaryThriftGenericV1SvcInfo != nil {
			return s.binaryThriftGenericV1SvcInfo
		}
		if s.combineSvcInfo != nil {
			return s.combineSvcInfo
		}
		if svcName == "" {
			// for non ttheader traffic, service name might be empty, we must fall back to method searching.
			if svcInfo := s.searchByMethodName(methodName); svcInfo != nil {
				return svcInfo
			}
		} else {
			if svc := s.knownSvcMap[svcName]; svc != nil {
				return svc.svcInfo
			}
			if svcName == serviceinfo.GenericService || svcName == serviceinfo.CombineServiceName {
				// Maybe combine or generic service name,
				// because Kitex client will write these two service name if the version is between v0.9.0-v0.9.1
				// TODO: remove this logic if this version range is converged
				if svcInfo := s.searchByMethodName(methodName); svcInfo != nil {
					return svcInfo
				}
			}
		}
	}
	if s.unknownSvc != nil {
		return s.unknownSvc.getOrStoreSvc(svcName, codecType).svcInfo
	}
	return nil
}

// getTargetSvcInfo returns the service info if there is only one service registered.
func (s *services) getTargetSvcInfo() *serviceinfo.ServiceInfo {
	if len(s.knownSvcMap) != 1 {
		return nil
	}
	for _, svc := range s.knownSvcMap {
		return svc.svcInfo
	}
	return nil
}

// registerBinaryGenericMethodFunc register binary generic method func to the given service info.
func registerBinaryGenericMethodFunc(svcInfo *serviceinfo.ServiceInfo) *serviceinfo.ServiceInfo {
	if isBinaryGeneric, _ := svcInfo.Extra[generic.IsBinaryGeneric].(bool); isBinaryGeneric {
		// already binary generic, no need to register
		return nil
	}
	var g generic.Generic
	switch svcInfo.PayloadCodec {
	case serviceinfo.Thrift:
		g = generic.BinaryThriftGenericV2(svcInfo.ServiceName)
	case serviceinfo.Protobuf:
		g = generic.BinaryPbGeneric(svcInfo.ServiceName, svcInfo.GetPackageName())
	default:
		return nil
	}
	nSvcInfo := *svcInfo
	binaryGenericMethod := g.GenericMethod()
	if oldGenericMethod := svcInfo.GenericMethod; oldGenericMethod != nil {
		// If oldGenericMethod is not nil, it means the service info already has a generic method func,
		// such as a service info created by json generic.
		// We should wrap the old generic method func with the binary generic method func
		nSvcInfo.GenericMethod = func(ctx context.Context, methodName string) serviceinfo.MethodInfo {
			mi := oldGenericMethod(ctx, methodName)
			if mi != nil {
				return mi
			}
			return binaryGenericMethod(ctx, methodName)
		}
	} else {
		nSvcInfo.GenericMethod = binaryGenericMethod
	}
	return &nSvcInfo
}
