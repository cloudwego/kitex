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
	"fmt"

	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

type service struct {
	svcInfo *serviceinfo.ServiceInfo
	handler interface{}
}

func newService(svcInfo *serviceinfo.ServiceInfo, handler interface{}) *service {
	return &service{svcInfo: svcInfo, handler: handler}
}

type services struct {
	methodSvcMap         map[string]*service // key: method name, value: svcInfo
	svcMap               map[string]*service // key: service name, value: svcInfo
	conflictingMethodMap map[string]bool
	fallbackSvc          *service

	refuseTrafficWithoutServiceName bool
}

func newServices() *services {
	return &services{
		methodSvcMap:         map[string]*service{},
		svcMap:               map[string]*service{},
		conflictingMethodMap: map[string]bool{},
	}
}

func (s *services) addService(svcInfo *serviceinfo.ServiceInfo, handler interface{}, registerOpts *RegisterOptions) error {
	svc := newService(svcInfo, handler)
	if registerOpts.IsFallbackService {
		if s.fallbackSvc != nil {
			return fmt.Errorf("multiple fallback services cannot be registered. [%s] is already registered as a fallback service", s.fallbackSvc.svcInfo.ServiceName)
		}
		s.fallbackSvc = svc
	}
	s.svcMap[svcInfo.ServiceName] = svc
	// method search map
	for methodName := range svcInfo.Methods {
		_, existMethod := s.methodSvcMap[methodName]
		if existMethod {
			// true means has conflicting method
			if _, ok := s.conflictingMethodMap[methodName]; !ok {
				s.conflictingMethodMap[methodName] = true
			}
		}
		if !existMethod || registerOpts.IsFallbackService {
			s.methodSvcMap[methodName] = svc
		}
		if registerOpts.IsFallbackService {
			s.conflictingMethodMap[methodName] = false
		}
	}
	return nil
}

func (s *services) handleConflictingMethod(svc *service, methodName string, registerOpts *RegisterOptions) {
}

func (s *services) getSvcInfoMap() map[string]*serviceinfo.ServiceInfo {
	svcInfoMap := map[string]*serviceinfo.ServiceInfo{}
	for name, svc := range s.svcMap {
		svcInfoMap[name] = svc.svcInfo
	}
	return svcInfoMap
}

func (s *services) SearchService(svcName, methodName string, strict bool) *serviceinfo.ServiceInfo {
	if strict || s.refuseTrafficWithoutServiceName {
		if svc := s.svcMap[svcName]; svc != nil {
			return svc.svcInfo
		}
		return nil
	}
	var svc *service
	if svcName == "" {
		svc = s.methodSvcMap[methodName]
	} else {
		svc = s.svcMap[svcName]
		if svc == nil {
			if _, ok := s.conflictingMethodMap[methodName]; !ok {
				// no conflicting method
				svc = s.methodSvcMap[methodName]
			}
		}
	}
	if svc != nil {
		return svc.svcInfo
	}
	return nil
}
