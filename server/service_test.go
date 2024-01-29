/*
 * Copyright 2024 CloudWeGo Authors
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
	"testing"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestAddService(t *testing.T) {
	svcs := newServices()
	err := svcs.addService(mocks.ServiceInfo(), mocks.MyServiceHandler(), &RegisterOptions{})
	test.Assert(t, err == nil)
	test.Assert(t, len(svcs.svcMap) == 1)
	fmt.Println(svcs.svcSearchMap)
	test.Assert(t, len(svcs.svcSearchMap) == 10)
	test.Assert(t, len(svcs.conflictingMethodHasFallbackSvcMap) == 0)
	test.Assert(t, svcs.fallbackSvc == nil)

	err = svcs.addService(mocks.Service3Info(), mocks.MyServiceHandler(), &RegisterOptions{IsFallbackService: true})
	test.Assert(t, err == nil)
	test.Assert(t, len(svcs.svcMap) == 2)
	test.Assert(t, len(svcs.svcSearchMap) == 11)
	test.Assert(t, len(svcs.conflictingMethodHasFallbackSvcMap) == 1)
	test.Assert(t, svcs.conflictingMethodHasFallbackSvcMap["mock"])

	err = svcs.addService(mocks.Service2Info(), mocks.MyServiceHandler(), &RegisterOptions{IsFallbackService: true})
	test.Assert(t, err != nil)
	test.Assert(t, err.Error() == "multiple fallback services cannot be registered. [MockService3] is already registered as a fallback service")
}

func TestCheckCombineServiceWithOtherService(t *testing.T) {
	svcs := newServices()
	combineSvcInfo := &serviceinfo.ServiceInfo{ServiceName: "CombineService"}
	svcs.svcMap[combineSvcInfo.ServiceName] = newService(combineSvcInfo, nil)
	err := svcs.checkCombineServiceWithOtherService(mocks.ServiceInfo())
	test.Assert(t, err != nil)
	test.Assert(t, err.Error() == "only one service can be registered when registering combine service")

	svcs = newServices()
	svcs.svcMap[mocks.MockServiceName] = newService(mocks.ServiceInfo(), mocks.MyServiceHandler())
	err = svcs.checkCombineServiceWithOtherService(combineSvcInfo)
	test.Assert(t, err != nil)
	test.Assert(t, err.Error() == "only one service can be registered when registering combine service")
}

func TestCheckMultipleFallbackService(t *testing.T) {
	svcs := newServices()
	svc := newService(mocks.ServiceInfo(), mocks.MyServiceHandler())
	registerOpts := &RegisterOptions{IsFallbackService: true}
	err := svcs.checkMultipleFallbackService(registerOpts, svc)
	test.Assert(t, err == nil)
	test.Assert(t, svcs.fallbackSvc == svc)

	err = svcs.checkMultipleFallbackService(registerOpts, newService(mocks.Service2Info(), nil))
	test.Assert(t, err != nil)
	test.Assert(t, err.Error() == "multiple fallback services cannot be registered. [MockService] is already registered as a fallback service", err)
}

func TestRegisterConflictingMethodHasFallbackSvcMap(t *testing.T) {
	svcs := newServices()
	svcFromMap := newService(mocks.ServiceInfo(), mocks.MyServiceHandler())
	svcs.registerConflictingMethodHasFallbackSvcMap(svcFromMap, mocks.MockMethod)
	test.Assert(t, !svcs.conflictingMethodHasFallbackSvcMap[mocks.MockMethod])

	svcs = newServices()
	svcs.fallbackSvc = svcFromMap
	svcs.registerConflictingMethodHasFallbackSvcMap(svcFromMap, mocks.MockMethod)
	test.Assert(t, svcs.conflictingMethodHasFallbackSvcMap[mocks.MockMethod])
}
