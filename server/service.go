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
	"errors"
	"fmt"
	"sync"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/generic"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

type serviceMiddlewares struct {
	MW endpoint.Middleware
}

type service struct {
	svcInfo *serviceinfo.ServiceInfo
	handler interface{}
	serviceMiddlewares
}

func newService(svcInfo *serviceinfo.ServiceInfo, handler interface{}, smw serviceMiddlewares) *service {
	return &service{svcInfo: svcInfo, handler: handler, serviceMiddlewares: smw}
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
		u.svcs[svcName] = nil
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
	methodSvcsMap map[string][]*service // key: method name
	svcMap        map[string]*service   // key: service name
	fallbackSvc   *service
	unknownSvc    *unknownService

	refuseTrafficWithoutServiceName bool
}

func newServices() *services {
	return &services{
		methodSvcsMap: map[string][]*service{},
		svcMap:        map[string]*service{},
	}
}

func (s *services) addService(svcInfo *serviceinfo.ServiceInfo, handler interface{}, registerOpts *RegisterOptions) error {
	// prepare serviceMiddlewares
	var serviceMWs serviceMiddlewares
	if len(registerOpts.Middlewares) > 0 {
		serviceMWs.MW = endpoint.Chain(registerOpts.Middlewares...)
	}

	// unknown service
	if registerOpts.IsUnknownService {
		if s.unknownSvc != nil {
			return errors.New("multiple unknown services cannot be registered")
		}
		s.unknownSvc = &unknownService{svcs: map[string]*service{}}
		return nil
	}

	svc := newService(svcInfo, handler, serviceMWs)
	if registerOpts.IsFallbackService {
		if s.fallbackSvc != nil {
			return fmt.Errorf("multiple fallback services cannot be registered. [%s] is already registered as a fallback service", s.fallbackSvc.svcInfo.ServiceName)
		}
		s.fallbackSvc = svc
	}
	if _, ok := s.svcMap[svcInfo.ServiceName]; ok {
		return fmt.Errorf("service [%s] has already been registered", svcInfo.ServiceName)
	}
	s.svcMap[svcInfo.ServiceName] = svc
	// method search map
	for methodName := range svcInfo.Methods {
		svcs := s.methodSvcsMap[methodName]
		if registerOpts.IsFallbackService {
			svcs = append([]*service{svc}, svcs...)
		} else {
			svcs = append(svcs, svc)
		}
		s.methodSvcsMap[methodName] = svcs
	}
	return nil
}

func (s *services) getSvcInfoMap() map[string]*serviceinfo.ServiceInfo {
	svcInfoMap := map[string]*serviceinfo.ServiceInfo{}
	for name, svc := range s.svcMap {
		svcInfoMap[name] = svc.svcInfo
	}
	return svcInfoMap
}

func (s *services) check(refuseTrafficWithoutServiceName bool) error {
	if len(s.svcMap) == 0 {
		return errors.New("run: no service. Use RegisterService to set one")
	}
	if refuseTrafficWithoutServiceName {
		s.refuseTrafficWithoutServiceName = true
		return nil
	}
	for name, svcs := range s.methodSvcsMap {
		if len(svcs) > 1 && svcs[0] != s.fallbackSvc {
			return fmt.Errorf("method name [%s] is conflicted between services but no fallback service is specified", name)
		}
	}
	return nil
}

func (s *services) getService(svcName string) *service {
	if svc := s.svcMap[svcName]; svc != nil {
		return svc
	}
	if s.unknownSvc != nil {
		return s.unknownSvc.getSvc(svcName)
	}
	return nil
}

func (s *services) SearchService(svcName, methodName string, strict bool, codecType serviceinfo.PayloadCodec) *serviceinfo.ServiceInfo {
	if strict || s.refuseTrafficWithoutServiceName {
		if svc := s.svcMap[svcName]; svc != nil {
			return svc.svcInfo
		}
		if s.unknownSvc != nil {
			return s.unknownSvc.getOrStoreSvc(svcName, codecType).svcInfo
		}
		return nil
	}
	var svc *service
	if svcName == "" {
		if svcs := s.methodSvcsMap[methodName]; len(svcs) > 0 {
			svc = svcs[0]
		}
	} else {
		svc = s.svcMap[svcName]
		if svc == nil {
			svcs := s.methodSvcsMap[methodName]
			// 1. no conflicting method, allow method routing
			// 2. $ generally means generic service, maybe mismatch the real service name
			if len(svcs) == 1 || len(svcs) > 1 && svcName[0] == '$' {
				svc = svcs[0]
			}
		}
	}
	if svc != nil {
		return svc.svcInfo
	}
	if s.unknownSvc != nil {
		return s.unknownSvc.getOrStoreSvc(svcName, codecType).svcInfo
	}
	return nil
}

func (s *services) hasRegisteredService() bool {
	return len(s.svcMap) > 0 || s.unknownSvc != nil
}

// getTargetSvcInfo returns the service info if there is only one service registered.
func (s *services) getTargetSvcInfo() *serviceinfo.ServiceInfo {
	if len(s.svcMap) != 1 {
		return nil
	}
	for _, svc := range s.svcMap {
		return svc.svcInfo
	}
	return nil
}
