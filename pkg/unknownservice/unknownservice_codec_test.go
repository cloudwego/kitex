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

package unknownservice

import (
	"context"
	"testing"

	gthrift "github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/kitex/internal/mocks"
	mt "github.com/cloudwego/kitex/internal/mocks/thrift"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	netpolltrans "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/unknownservice/service"
	"github.com/cloudwego/kitex/transport"
	"github.com/cloudwego/netpoll"
)

var (
	thr          = thrift.NewThriftCodec()
	payloadCodec = unknownServiceCodec{thr}
	svcInfo      = mocks.ServiceInfo()
)

func TestNormal(t *testing.T) {
	sendMsg := initSendMsg(transport.TTHeader)
	buf := netpolltrans.NewReaderWriterByteBuffer(netpoll.NewLinkBuffer(1024))
	ctx := context.Background()
	err := payloadCodec.Marshal(ctx, sendMsg, buf)
	test.Assert(t, err == nil, err)
	err = buf.Flush()
	test.Assert(t, err == nil, err)
	recvMsg := initRecvMsg()
	recvMsg.SetPayloadLen(buf.ReadableLen())
	_, size, err := peekMethod(buf)
	test.Assert(t, err == nil, err)
	err = payloadCodec.Unmarshal(ctx, recvMsg, buf)
	test.Assert(t, err == nil, err)

	req := (sendMsg.Data()).(*service.Result).Success
	resp := (recvMsg.Data()).(*service.Args).Request
	req = req[size+4:]
	resp = resp[size+4:]
	for i, item := range req {
		test.Assert(t, item == resp[i])
	}
	var _req mt.MockTestArgs
	var _resp mt.MockTestArgs
	reqMsg, err := _req.FastRead(req)
	test.Assert(t, err == nil, err)
	respMsg, err := _resp.FastRead(resp)
	test.Assert(t, err == nil && reqMsg == respMsg, err)
	test.Assert(t, len(_req.Req.StrList) == len(_resp.Req.StrList))
	test.Assert(t, len(_req.Req.StrMap) == len(_resp.Req.StrList))
	for i, item := range _req.Req.StrList {
		test.Assert(t, item == _resp.Req.StrList[i])
	}
	for k := range _resp.Req.StrMap {
		test.Assert(t, _req.Req.StrMap[k] == _resp.Req.StrMap[k])
	}
}

func initSendMsg(tp transport.Protocol) remote.Message {
	var _args mt.MockTestArgs
	_args.Req = prepareReq()
	length := _args.BLength()
	method := "mock"
	size := gthrift.Binary.MessageBeginLength(method)
	bytes := make([]byte, length+size)
	offset := gthrift.Binary.WriteMessageBegin(bytes, method, int32(remote.Call), 1)
	_args.FastWriteNocopy(bytes[offset:], nil)
	arg := service.Result{Success: bytes, Method: method, ServiceName: ""}
	ink := rpcinfo.NewInvocation("", service.UnknownMethod)
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)

	msg := remote.NewMessage(&arg, svcInfo, ri, remote.Call, remote.Client)

	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

func initRecvMsg() remote.Message {
	arg := service.Args{Request: make([]byte, 0), Method: "mock", ServiceName: ""}
	ink := rpcinfo.NewInvocation("", service.UnknownMethod)
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
	svc := service.NewServiceInfo(svcInfo.PayloadCodec, service.UnknownService, service.UnknownMethod)
	msg := remote.NewMessage(&arg, svc, ri, remote.Call, remote.Server)
	return msg
}

func prepareReq() *mt.MockReq {
	strMap := make(map[string]string)
	strMap["key1"] = "val1"
	strMap["key2"] = "val2"
	strList := []string{"str1", "str2"}
	req := &mt.MockReq{
		Msg:     "MockReq",
		StrMap:  strMap,
		StrList: strList,
	}
	return req
}
