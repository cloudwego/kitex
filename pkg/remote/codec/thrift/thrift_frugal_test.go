//go:build (amd64 || arm64) && !windows && go1.16 && !go1.23 && !disablefrugal
// +build amd64 arm64
// +build !windows
// +build go1.16
// +build !go1.23
// +build !disablefrugal

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
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/kitex/internal/mocks/thrift/fast"
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
	test.Assert(t, codec.hyperMessageUnmarshalAvailable(&MockNoTagArgs{}, msg.PayloadLen()) == false)
	test.Assert(t, codec.hyperMessageUnmarshalAvailable(&MockFrugalTagArgs{}, msg.PayloadLen()) == false)
	msg.SetPayloadLen(1)
	test.Assert(t, codec.hyperMessageUnmarshalAvailable(&MockFrugalTagArgs{}, msg.PayloadLen()) == true)
}

func TestFrugalCodec(t *testing.T) {
	t.Run("configure frugal but data has not tag", func(t *testing.T) {
		ctx := context.Background()
		codec := &thriftCodec{FrugalRead | FrugalWrite}

		// MockNoTagArgs cannot be marshaled
		sendMsg := initNoTagSendMsg(transport.TTHeader)
		out := remote.NewWriterBuffer(256)
		err := codec.Marshal(ctx, sendMsg, out)
		test.Assert(t, err != nil)
	})
	t.Run("configure frugal and data has tag", func(t *testing.T) {
		ctx := context.Background()
		codec := &thriftCodec{FrugalRead | FrugalWrite}

		testFrugalDataConversion(t, ctx, codec)
	})
	t.Run("fallback to frugal and data has tag", func(t *testing.T) {
		ctx := context.Background()
		codec := NewThriftCodec()

		testFrugalDataConversion(t, ctx, codec)
	})
	t.Run("configure BasicCodec to disable frugal fallback", func(t *testing.T) {
		ctx := context.Background()
		codec := NewThriftCodecWithConfig(Basic)

		// MockNoTagArgs cannot be marshaled
		sendMsg := initNoTagSendMsg(transport.TTHeader)
		out := remote.NewWriterBuffer(256)
		err := codec.Marshal(ctx, sendMsg, out)
		test.Assert(t, err != nil)
	})
}

func testFrugalDataConversion(t *testing.T, ctx context.Context, codec remote.PayloadCodec) {
	// encode client side
	sendMsg := initFrugalTagSendMsg(transport.TTHeader)
	out := remote.NewWriterBuffer(256)
	err := codec.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil, err)

	// decode server side
	recvMsg := initFrugalTagRecvMsg()
	buf, err := out.Bytes()
	recvMsg.SetPayloadLen(len(buf))
	test.Assert(t, err == nil, err)
	in := remote.NewReaderBuffer(buf)
	err = codec.Unmarshal(ctx, recvMsg, in)
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

func TestMarshalThriftDataFrugal(t *testing.T) {
	mockReqFrugal := &MockFrugalTagReq{
		Msg: "hello",
	}
	successfulCodecs := []remote.PayloadCodec{
		NewThriftCodecWithConfig(FrugalWrite),
		// fallback to frugal
		nil,
		// fallback to frugal
		NewThriftCodec(),
	}
	for _, codec := range successfulCodecs {
		buf, err := MarshalThriftData(context.Background(), codec, mockReqFrugal)
		test.Assert(t, err == nil, err)
		test.Assert(t, reflect.DeepEqual(buf, mockReqThrift), buf)
	}

	// Basic can be used for disabling frugal
	_, err := MarshalThriftData(context.Background(), NewThriftCodecWithConfig(Basic), mockReqFrugal)
	test.Assert(t, err != nil, err)
}

func TestUnmarshalThriftDataFrugal(t *testing.T) {
	req := &MockFrugalTagReq{}
	successfulCodecs := []remote.PayloadCodec{
		NewThriftCodecWithConfig(FrugalRead),
		// fallback to frugal
		nil,
		// fallback to frugal
		NewThriftCodec(),
	}
	for _, codec := range successfulCodecs {
		err := UnmarshalThriftData(context.Background(), codec, "mock", mockReqThrift, req)
		checkDecodeResult(t, err, &fast.MockReq{
			Msg:     req.Msg,
			StrList: req.StrList,
			StrMap:  req.StrMap,
		})

	}

	// Basic can be used for disabling frugal
	err := UnmarshalThriftData(context.Background(), NewThriftCodecWithConfig(Basic), "mock", mockReqThrift, req)
	test.Assert(t, err != nil, err)
}

func TestThriftCodec_unmarshalThriftDataFrugal(t *testing.T) {
	t.Run("Frugal with SkipDecoder enabled", func(t *testing.T) {
		req := &MockFrugalTagReq{}
		codec := &thriftCodec{FrugalRead | EnableSkipDecoder}
		tProt := NewBinaryProtocol(remote.NewReaderBuffer(mockReqThrift))
		defer tProt.Recycle()
		// specify dataLen with 0 so that skipDecoder works
		err := codec.unmarshalThriftData(context.Background(), tProt, "mock", req, 0)
		checkDecodeResult(t, err, &fast.MockReq{
			Msg:     req.Msg,
			StrList: req.StrList,
			StrMap:  req.StrMap,
		})
	})

	t.Run("Frugal with SkipDecoder enabled, failed in using SkipDecoder Buffer", func(t *testing.T) {
		req := &MockFrugalTagReq{}
		codec := &thriftCodec{FrugalRead | EnableSkipDecoder}
		// these bytes are mapped to
		//  Msg     string            `thrift:"Msg,1" json:"Msg"`
		//	StrMap  map[string]string `thrift:"strMap,2" json:"strMap"`
		//	I16List []int16           `thrift:"I16List,3" json:"i16List"`
		faultMockReqThrift := []byte{
			11 /* string */, 0, 1 /* id=1 */, 0, 0, 0, 5 /* length=5 */, 104, 101, 108, 108, 111, /* "hello" */
			13 /* map */, 0, 2 /* id=2 */, 11 /* key:string*/, 11 /* value:string */, 0, 0, 0, 0, /* map size=0 */
			15 /* list */, 0, 3 /* id=3 */, 6 /* item:I16 */, 0, 0, 0, 1 /* length=1 */, 0, 1, /* I16=1 */
			0, /* end of struct */
		}
		tProt := NewBinaryProtocol(remote.NewReaderBuffer(faultMockReqThrift))
		defer tProt.Recycle()
		// specify dataLen with 0 so that skipDecoder works
		err := codec.unmarshalThriftData(context.Background(), tProt, "mock", req, 0)
		test.Assert(t, err != nil, err)
		test.Assert(t, strings.Contains(err.Error(), "caught in Frugal using SkipDecoder Buffer"))
	})
}

func Test_verifyMarshalThriftDataFrugal(t *testing.T) {
	err := verifyMarshalBasicThriftDataType(&MockFrugalTagArgs{})
	test.Assert(t, err == errEncodeMismatchMsgType, err)
	err = verifyMarshalBasicThriftDataType(&MockNoTagArgs{})
	test.Assert(t, err == errEncodeMismatchMsgType, err)
}

func Test_verifyUnmarshalThriftDataFrugal(t *testing.T) {
	err := verifyUnmarshalBasicThriftDataType(&MockFrugalTagArgs{})
	test.Assert(t, err == errDecodeMismatchMsgType, err)
	err = verifyUnmarshalBasicThriftDataType(&MockNoTagArgs{})
	test.Assert(t, err == errDecodeMismatchMsgType, err)
}
