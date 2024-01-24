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
	svcSearchMap                       map[string]*service // key: "svcName.methodName" and "methodName", value: svcInfo
	svcMap                             map[string]*service // key: service name, value: svcInfo
	conflictingMethodHasFallbackSvcMap map[string]bool
	fallbackSvc                        *service
	hasCombineService                  bool
}

func newServices() *services {
	return &services{
		svcSearchMap:                       map[string]*service{},
		svcMap:                             map[string]*service{},
		conflictingMethodHasFallbackSvcMap: map[string]bool{},
	}
}

func (s *services) addService(svcInfo *serviceinfo.ServiceInfo, handler interface{}, isFallbackService bool) error {
	svc := newService(svcInfo, handler)

	// combine service check
	if svcInfo.ServiceName == "CombineService" {
		s.hasCombineService = true
	} else if s.hasCombineService {
		return errors.New("combine service cannot be registered with other services. please register services without using combine service or register only combine service")
	}

	if isFallbackService {
		if s.fallbackSvc != nil {
			return fmt.Errorf("multiple fallback services cannot be registered. [%s] is already registered as a fallback service", s.fallbackSvc.svcInfo.ServiceName)
		}
		s.fallbackSvc = svc
	}
	s.svcMap[svcInfo.ServiceName] = svc
	for methodName := range svcInfo.Methods {
		s.svcSearchMap[fmt.Sprintf("%s.%s", svcInfo.ServiceName, methodName)] = svc
		// conflicting method check
		if svcFromMap, ok := s.svcSearchMap[methodName]; ok {
			if _, ok := s.conflictingMethodHasFallbackSvcMap[methodName]; !ok {
				if s.fallbackSvc != nil && svcFromMap.svcInfo.ServiceName == s.fallbackSvc.svcInfo.ServiceName {
					s.conflictingMethodHasFallbackSvcMap[methodName] = true
				} else {
					s.conflictingMethodHasFallbackSvcMap[methodName] = false
				}
			}
			if isFallbackService {
				s.svcSearchMap[methodName] = svc
				s.conflictingMethodHasFallbackSvcMap[methodName] = true
			}
		} else {
			s.svcSearchMap[methodName] = svc
		}
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

func (s *services) getSvcInfoSearchMap() map[string]*serviceinfo.ServiceInfo {
	svcInfoSearchMap := map[string]*serviceinfo.ServiceInfo{}
	for name, svc := range s.svcSearchMap {
		svcInfoSearchMap[name] = svc.svcInfo
	}
	return svcInfoSearchMap
}
