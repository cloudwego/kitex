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
	svcInfo    *serviceinfo.ServiceInfo
	handler    interface{}
	isFallback bool
}

func newService(svcInfo *serviceinfo.ServiceInfo, handler interface{}) *service {
	return &service{svcInfo: svcInfo, handler: handler}
}

type services struct {
	svcMap                             map[string]*service
	methodSvcMap                       map[string]*service
	conflictingMethodHasFallbackSvcMap map[string]bool
	fallbackSvc                        *service
}

func newServices() *services {
	return &services{svcMap: map[string]*service{}, methodSvcMap: map[string]*service{}, conflictingMethodHasFallbackSvcMap: map[string]bool{}}
}

func (s *services) addService(svcInfo *serviceinfo.ServiceInfo, handler interface{}, isFallbackService ...bool) error {
	svc := newService(svcInfo, handler)

	if len(isFallbackService) > 0 && isFallbackService[0] {
		if s.fallbackSvc != nil {
			return fmt.Errorf("multiple fallback services cannot be registered. [%s] is already registered as a fallback service", s.fallbackSvc.svcInfo.ServiceName)
		}
		s.fallbackSvc = svc
		svc.isFallback = true
	}
	s.svcMap[svcInfo.ServiceName] = svc
	for methodName := range svcInfo.Methods {
		// conflicting method check
		if svcFromMap, ok := s.methodSvcMap[methodName]; ok {
			if _, ok := s.conflictingMethodHasFallbackSvcMap[methodName]; !ok {
				s.conflictingMethodHasFallbackSvcMap[methodName] = svcFromMap.isFallback
			}
			if svc.isFallback {
				s.methodSvcMap[methodName] = svc
				s.conflictingMethodHasFallbackSvcMap[methodName] = true
			}
		} else {
			s.methodSvcMap[methodName] = svc
		}
	}
	return nil
}

func (s *services) getService(svcName string) *service {
	return s.svcMap[svcName]
}

func (s *services) getServiceByMethodName(methodName string) *service {
	return s.methodSvcMap[methodName]
}

func (s *services) getSvcInfoMap() map[string]*serviceinfo.ServiceInfo {
	svcInfoMap := map[string]*serviceinfo.ServiceInfo{}
	for name, svc := range s.svcMap {
		svcInfoMap[name] = svc.svcInfo
	}
	return svcInfoMap
}

func (s *services) getMethodNameSvcInfoMap() map[string]*serviceinfo.ServiceInfo {
	methodNameSvcInfoMap := map[string]*serviceinfo.ServiceInfo{}
	for methodName, svc := range s.methodSvcMap {
		methodNameSvcInfoMap[methodName] = svc.svcInfo
	}
	return methodNameSvcInfoMap
}
