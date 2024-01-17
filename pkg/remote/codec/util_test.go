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

package codec

import (
	"fmt"
	"testing"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestSetOrCheckMethodName(t *testing.T) {
	ri := rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("", "mock", nil, nil),
		rpcinfo.NewServerInvocation(), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	svcInfo := mocks.ServiceInfo()
	svcSearchMap := map[string]*serviceinfo.ServiceInfo{
		fmt.Sprintf("%s.%s", mocks.MockServiceName, mocks.MockMethod):          svcInfo,
		fmt.Sprintf("%s.%s", mocks.MockServiceName, mocks.MockExceptionMethod): svcInfo,
		fmt.Sprintf("%s.%s", mocks.MockServiceName, mocks.MockErrorMethod):     svcInfo,
		fmt.Sprintf("%s.%s", mocks.MockServiceName, mocks.MockOnewayMethod):    svcInfo,
		mocks.MockMethod:          svcInfo,
		mocks.MockExceptionMethod: svcInfo,
		mocks.MockErrorMethod:     svcInfo,
		mocks.MockOnewayMethod:    svcInfo,
	}
	msg := remote.NewMessageWithNewer(svcInfo, svcSearchMap, ri, remote.Call, remote.Server)
	err := SetOrCheckMethodName("mock", msg)
	test.Assert(t, err == nil)
	ri = msg.RPCInfo()
	test.Assert(t, ri.Invocation().ServiceName() == mocks.MockServiceName)
	test.Assert(t, ri.Invocation().PackageName() == "mock")
	test.Assert(t, ri.Invocation().MethodName() == "mock")
	test.Assert(t, ri.To().Method() == "mock")

	msg = remote.NewMessageWithNewer(svcInfo, map[string]*serviceinfo.ServiceInfo{}, ri, remote.Call, remote.Server)
	err = SetOrCheckMethodName("dummy", msg)
	test.Assert(t, err != nil)
	test.Assert(t, err.Error() == "unknown method dummy")
}
