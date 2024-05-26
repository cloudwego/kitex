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

package unknown

import (
	"context"
	"github.com/cloudwego/kitex/internal/mocks"
	mt "github.com/cloudwego/kitex/internal/mocks/thrift/fast"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	unknown "github.com/cloudwego/kitex/pkg/remote/codec/unknown/service"
	netpolltrans "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
	"github.com/cloudwego/netpoll"
	"testing"
)

var (
	thr          = thrift.NewThriftCodec()
	payloadCodec = unknownCodec{thr}
	svcInfo      = mocks.ServiceInfo()
)

func TestNormal(t *testing.T) {
	sendMsg := initSendMsg(transport.TTHeader)
	buf := netpolltrans.NewReaderWriterByteBuffer(netpoll.NewLinkBuffer(1024))
	ctx := context.Background()
	err := payloadCodec.Marshal(ctx, sendMsg, buf)
	test.Assert(t, err == nil, err)
	buf.Flush()
	recvMsg := initRecvMsg()
	recvMsg.SetPayloadLen(buf.ReadableLen())
	err = payloadCodec.Unmarshal(ctx, recvMsg, buf)
	test.Assert(t, err == nil, err)

	req := (sendMsg.Data()).(*unknown.Result).Success
	resp := (recvMsg.Data()).(*unknown.Args).Request
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
	bytes := make([]byte, length)
	_args.FastWriteNocopy(bytes, nil)
	arg := unknown.Result{Success: bytes, Method: "mock", ServiceName: ""}

	ink := rpcinfo.NewInvocation("", unknown.UnknownMethod)
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)

	msg := remote.NewMessage(&arg, svcInfo, ri, remote.Call, remote.Client)

	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg

}

func initRecvMsg() remote.Message {
	arg := unknown.Args{Request: make([]byte, 0), Method: "mock", ServiceName: ""}
	ink := rpcinfo.NewInvocation("", unknown.UnknownMethod)
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
	svc := unknown.NewServiceInfo(svcInfo.PayloadCodec, unknown.UnknownService, unknown.UnknownMethod)
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
