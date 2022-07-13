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
)

func TestSetOrCheckMethodName(t *testing.T) {
	var req interface{}
	ri := rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("", "mock", nil, nil),
		rpcinfo.NewServerInvocation(), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
	err := SetOrCheckMethodName("mock", msg)
	test.Assert(t, err == nil)
	ri = msg.RPCInfo()
	test.Assert(t, ri.Invocation().ServiceName() == mocks.MockServiceName)
	test.Assert(t, ri.Invocation().PackageName() == "mock")
	test.Assert(t, ri.Invocation().MethodName() == "mock")
	test.Assert(t, ri.To().Method() == "mock")
}

// test  SetOrCheckSeqID with  uint32
func TestSetOrCheckSeqID(t *testing.T) {
	var requset interface{}
	ri := rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("", "mock", nil, nil),
		rpcinfo.NewServerInvocation(), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	msg := remote.NewMessage(requset, mocks.ServiceInfo(), ri, remote.Oneway, remote.Server)
	err := SetOrCheckSeqID(2, msg)
	test.Assert(t, err == nil)
	ri = msg.RPCInfo()
	test.Assert(t, ri.Invocation().SeqID() == 2)
	msg2 := remote.NewMessage(requset, mocks.ServiceInfo(), ri, remote.Reply, remote.Server)
	err2 := SetOrCheckSeqID(2, msg2)
	test.Assert(t, err2 == nil)
}

// test  UpdateMsgType with  uint32
func TestUpdateMsgType(t *testing.T) {
	var req interface{}
	ri := rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("", "mock", nil, nil),
		rpcinfo.NewServerInvocation(), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
	err := UpdateMsgType(uint32(remote.Oneway), msg)
	test.Assert(t, err == nil)
	test.Assert(t, msg.MessageType() == remote.Oneway)
	msg1 := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.InternalError, remote.Server)
	err2 := UpdateMsgType(uint32(remote.Reply), msg1)
	test.Assert(t, err2 != nil)
}

// test is used to create the data if not exist.
func TestNewDataIfNeeded(t *testing.T) {
	var req interface{}
	ri := rpcinfo.NewRPCInfo(nil, rpcinfo.NewEndpointInfo("", "", nil, nil),
		rpcinfo.NewServerInvocation(), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Oneway, remote.Server)
	data := msg.Data()
	fmt.Println(data)
	err := NewDataIfNeeded("mock", msg)
	test.Assert(t, err == nil)
}
