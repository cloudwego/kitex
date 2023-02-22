//go:build amd64 && !windows && go1.16
// +build amd64,!windows,go1.16

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

package thrift

import (
	"context"
	"testing"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

type MockFrugalTagReq struct {
	Msg     string            `frugal:"1,default,string"`
	StrMap  map[string]string `frugal:"2,default,map<string:string>"`
	StrList []string          `frugal:"3,default,list<string>"`
}

type MockNoTagArgs struct {
	Req *MockFrugalTagReq
}

func initNoTagSendMsg(tp transport.Protocol) remote.Message {
	var _args MockNoTagArgs
	ink := rpcinfo.NewInvocation("", "mock")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
	msg := remote.NewMessage(&_args, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

type MockFrugalTagArgs struct {
	Req *MockFrugalTagReq `frugal:"1,default,MockFrugalTagReq"`
}

func initFrugalTagSendMsg(tp transport.Protocol) remote.Message {
	var _args MockFrugalTagArgs
	_args.Req = &MockFrugalTagReq{
		Msg:     "MockReq",
		StrMap:  map[string]string{"0": "0", "1": "1", "2": "2"},
		StrList: []string{"0", "1", "2"},
	}
	ink := rpcinfo.NewInvocation("", "mock")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
	msg := remote.NewMessage(&_args, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

func initFrugalTagRecvMsg() remote.Message {
	var _args MockFrugalTagArgs
	ink := rpcinfo.NewInvocation("", "mock")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(&_args, svcInfo, ri, remote.Call, remote.Server)
	return msg
}

func TestHyperCodecCheck(t *testing.T) {
	msg := initFrugalTagRecvMsg()
	msg.SetPayloadLen(0)
	codec := &thriftCodec{}

	// test CodecType check
	test.Assert(t, codec.hyperMarshalEnabled() == false)
	msg.SetPayloadLen(1)
	test.Assert(t, codec.hyperMessageUnmarshalEnabled() == false)
	msg.SetPayloadLen(0)

	// test hyperMarshal check
	codec = &thriftCodec{FrugalWrite}
	test.Assert(t, hyperMarshalAvailable(&MockNoTagArgs{}) == false)
	test.Assert(t, hyperMarshalAvailable(&MockFrugalTagArgs{}) == true)

	// test hyperMessageUnmarshal check
	codec = &thriftCodec{FrugalRead}
	test.Assert(t, hyperMessageUnmarshalAvailable(&MockNoTagArgs{}, msg) == false)
	test.Assert(t, hyperMessageUnmarshalAvailable(&MockFrugalTagArgs{}, msg) == false)
	msg.SetPayloadLen(1)
	test.Assert(t, hyperMessageUnmarshalAvailable(&MockFrugalTagArgs{}, msg) == true)
}

func TestFrugalCodec(t *testing.T) {
	ctx := context.Background()
	frugalCodec := &thriftCodec{FrugalRead | FrugalWrite}

	// encode client side
	// MockNoTagArgs cannot be marshaled
	sendMsg := initNoTagSendMsg(transport.TTHeader)
	out := remote.NewWriterBuffer(256)
	err := frugalCodec.Marshal(ctx, sendMsg, out)
	test.Assert(t, err != nil)

	// MockFrugalTagArgs can be marshaled by frugal
	sendMsg = initFrugalTagSendMsg(transport.TTHeader)
	out = remote.NewWriterBuffer(256)
	err = frugalCodec.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil, err)

	// decode server side
	recvMsg := initFrugalTagRecvMsg()
	buf, err := out.Bytes()
	recvMsg.SetPayloadLen(len(buf))
	test.Assert(t, err == nil, err)
	in := remote.NewReaderBuffer(buf)
	err = frugalCodec.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil, err)

	// compare Args
	sendReq := (sendMsg.Data()).(*MockFrugalTagArgs).Req
	recvReq := (recvMsg.Data()).(*MockFrugalTagArgs).Req
	test.Assert(t, sendReq.Msg == recvReq.Msg)
	test.Assert(t, len(sendReq.StrList) == len(recvReq.StrList))
	test.Assert(t, len(sendReq.StrMap) == len(recvReq.StrMap))
	for i, item := range sendReq.StrList {
		test.Assert(t, item == recvReq.StrList[i])
	}
	for k := range sendReq.StrMap {
		test.Assert(t, sendReq.StrMap[k] == recvReq.StrMap[k])
	}
}
