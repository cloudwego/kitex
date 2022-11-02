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

package generic

import (
	"context"
	"testing"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

func TestMapThriftCodec(t *testing.T) {
	p, err := NewThriftFileProvider("./map_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	mtc, err := newMapThriftCodec(p, thriftCodec)
	test.Assert(t, err == nil)
	defer mtc.Close()
	test.Assert(t, mtc.Name() == "MapThrift")

	method, err := mtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")

	ctx := context.Background()
	sendMsg := initMapSendMsg(transport.TTHeader)

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = mtc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil)

	// UnMarshal side
	recvMsg := initMapRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = mtc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil)
}

func TestMapThriftCodecForJSON(t *testing.T) {
	p, err := NewThriftFileProvider("./map_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	mtc, err := newMapThriftCodecForJSON(p, thriftCodec)
	test.Assert(t, err == nil)
	defer mtc.Close()
	test.Assert(t, mtc.Name() == "MapThrift")

	method, err := mtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")

	ctx := context.Background()
	sendMsg := initMapSendMsg(transport.TTHeader)

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = mtc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil)

	// UnMarshal side
	recvMsg := initMapRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = mtc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil)
	args, ok := recvMsg.Data().(*Args)
	test.Assert(t, ok)
	fieldMap, ok := args.Request.(map[string]interface{})
	test.Assert(t, ok)
	_, ok = fieldMap["strMap"].(map[string]interface{})
	test.Assert(t, ok)
}

func TestMapExceptionError(t *testing.T) {
	p, err := NewThriftFileProvider("./map_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	mtc, err := newMapThriftCodec(p, thriftCodec)
	test.Assert(t, err == nil)

	ctx := context.Background()
	out := remote.NewWriterBuffer(256)
	// empty method test
	emptyMethodInk := rpcinfo.NewInvocation("", "")
	emptyMethodRi := rpcinfo.NewRPCInfo(nil, nil, emptyMethodInk, nil, nil)
	emptyMethodMsg := remote.NewMessage(nil, nil, emptyMethodRi, remote.Exception, remote.Client)
	// Marshal side
	err = mtc.Marshal(ctx, emptyMethodMsg, out)
	test.Assert(t, err.Error() == "empty methodName in thrift Marshal")

	// Exception MsgType test
	exceptionMsgTypeInk := rpcinfo.NewInvocation("", "Test")
	exceptionMsgTypeRi := rpcinfo.NewRPCInfo(nil, nil, exceptionMsgTypeInk, nil, nil)
	exceptionMsgTypeMsg := remote.NewMessage(&remote.TransError{}, nil, exceptionMsgTypeRi, remote.Exception, remote.Client)
	// Marshal side
	err = mtc.Marshal(ctx, exceptionMsgTypeMsg, out)
	test.Assert(t, err == nil)
}

func initMapSendMsg(tp transport.Protocol) remote.Message {
	req := &Args{
		Request: &descriptor.HTTPRequest{
			Body: map[string]interface{}{
				"strMap": map[string]interface{}{
					"Test1": "Test2",
				},
			},
			Method: "Test",
		},
		Method: "Test",
	}
	svcInfo := mocks.ServiceInfo()
	ink := rpcinfo.NewInvocation("", "Test")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

func initMapRecvMsg() remote.Message {
	req := &Args{
		Request: "Test",
		Method:  "Test",
	}
	ink := rpcinfo.NewInvocation("", "Test")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
	return msg
}
