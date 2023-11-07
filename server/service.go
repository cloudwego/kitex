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

package server

import "github.com/cloudwego/kitex/pkg/serviceinfo"

type service struct {
	svcInfo *serviceinfo.ServiceInfo
	handler interface{}
}

func newService(svcInfo *serviceinfo.ServiceInfo, handler interface{}) *service {
	return &service{svcInfo: svcInfo, handler: handler}
}

type services struct {
	svcMap     map[string]*service
	defaultSvc *service
}

func newServices() *services {
	return &services{svcMap: map[string]*service{}}
}

func (s *services) addService(svcInfo *serviceinfo.ServiceInfo, handler interface{}) {
	svc := newService(svcInfo, handler)
	if s.defaultSvc == nil {
		s.defaultSvc = svc
	}
	s.svcMap[svcInfo.ServiceName] = svc
}

func (s *services) getService(svcName string) *service {
	return s.svcMap[svcName]
}

func (s *services) getSvcInfoMap() map[string]*serviceinfo.ServiceInfo {
	svcInfoMap := map[string]*serviceinfo.ServiceInfo{}
	for name, svc := range s.svcMap {
		svcInfoMap[name] = svc.svcInfo
	}
	return svcInfoMap
}
