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

	"github.com/cloudwego/kitex/pkg/remote"
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
}

func newServices() *services {
	return &services{
		svcSearchMap:                       map[string]*service{},
		svcMap:                             map[string]*service{},
		conflictingMethodHasFallbackSvcMap: map[string]bool{},
	}
}

func (s *services) addService(svcInfo *serviceinfo.ServiceInfo, handler interface{}, registerOpts *RegisterOptions) error {
	svc := newService(svcInfo, handler)

	if err := s.checkCombineServiceWithOtherService(svcInfo); err != nil {
		return err
	}

	if err := s.checkMultipleFallbackService(registerOpts, svc); err != nil {
		return err
	}

	s.svcMap[svcInfo.ServiceName] = svc
	s.createSearchMap(svcInfo, svc, registerOpts)
	return nil
}

// when registering combine service, it does not allow the registration of other services
func (s *services) checkCombineServiceWithOtherService(svcInfo *serviceinfo.ServiceInfo) error {
	if len(s.svcMap) > 0 {
		if _, ok := s.svcMap["CombineService"]; ok || svcInfo.ServiceName == "CombineService" {
			return errors.New("only one service can be registered when registering combine service")
		}
	}
	return nil
}

func (s *services) checkMultipleFallbackService(registerOpts *RegisterOptions, svc *service) error {
	if registerOpts.IsFallbackService {
		if s.fallbackSvc != nil {
			return fmt.Errorf("multiple fallback services cannot be registered. [%s] is already registered as a fallback service", s.fallbackSvc.svcInfo.ServiceName)
		}
		s.fallbackSvc = svc
	}
	return nil
}

func (s *services) createSearchMap(svcInfo *serviceinfo.ServiceInfo, svc *service, registerOpts *RegisterOptions) {
	for methodName := range svcInfo.Methods {
		s.svcSearchMap[remote.CreateConcatKey(svcInfo.ServiceName, methodName)] = svc
		if svcFromMap, ok := s.svcSearchMap[methodName]; ok {
			s.handleConflictingMethod(svcFromMap, svc, methodName, registerOpts)
		} else {
			s.svcSearchMap[methodName] = svc
		}
	}
}

func (s *services) handleConflictingMethod(svcFromMap, svc *service, methodName string, registerOpts *RegisterOptions) {
	s.registerConflictingMethodHasFallbackSvcMap(svcFromMap, methodName)
	s.updateWithFallbackSvc(registerOpts, svc, methodName)
}

func (s *services) registerConflictingMethodHasFallbackSvcMap(svcFromMap *service, methodName string) {
	if _, ok := s.conflictingMethodHasFallbackSvcMap[methodName]; !ok {
		if s.fallbackSvc != nil && svcFromMap.svcInfo.ServiceName == s.fallbackSvc.svcInfo.ServiceName {
			// svc which is already registered is a fallback service
			s.conflictingMethodHasFallbackSvcMap[methodName] = true
		} else {
			s.conflictingMethodHasFallbackSvcMap[methodName] = false
		}
	}
}

func (s *services) updateWithFallbackSvc(registerOpts *RegisterOptions, svc *service, methodName string) {
	if registerOpts.IsFallbackService {
		s.svcSearchMap[methodName] = svc
		s.conflictingMethodHasFallbackSvcMap[methodName] = true
	}
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
