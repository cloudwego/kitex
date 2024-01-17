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
	"testing"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
)

func TestAddService(t *testing.T) {
	svcs := newServices()
	err := svcs.addService(mocks.ServiceInfo(), mocks.MyServiceHandler())
	test.Assert(t, err == nil)
	test.Assert(t, len(svcs.svcMap) == 1)
	test.Assert(t, len(svcs.svcSearchMap) == 8)
	test.Assert(t, len(svcs.conflictingMethodHasFallbackSvcMap) == 0)
	test.Assert(t, svcs.fallbackSvc == nil)

	err = svcs.addService(mocks.Service3Info(), mocks.MyServiceHandler(), true)
	test.Assert(t, err == nil)
	test.Assert(t, len(svcs.svcMap) == 2)
	test.Assert(t, len(svcs.svcSearchMap) == 9)
	test.Assert(t, len(svcs.conflictingMethodHasFallbackSvcMap) == 1)
	test.Assert(t, svcs.conflictingMethodHasFallbackSvcMap["mock"])

	err = svcs.addService(mocks.Service2Info(), mocks.MyServiceHandler(), true)
	test.Assert(t, err != nil)
	test.Assert(t, err.Error() == "multiple fallback services cannot be registered. [MockService3] is already registered as a fallback service")
}
