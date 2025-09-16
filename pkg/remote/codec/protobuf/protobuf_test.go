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

package protobuf

import (
	"context"
	"encoding/binary"
	"errors"
	"math/bits"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/cloudwego/kitex/internal/mocks"
	mocksremote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

var (
	payloadCodec = &protobufCodec{}
	svcInfo      = mocks.ServiceInfo()
)

func init() {
	svcInfo.Methods["mock"] = serviceinfo.NewMethodInfo(nil, newMockReqArgs, nil, false)
}

func TestNormal(t *testing.T) {
	ctx := context.Background()

	// encode // client side
	sendMsg := initMockReqArgsSendMsg()
	out := remote.NewWriterBuffer(256)
	err := payloadCodec.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil, err)

	// decode server side
	ctx, recvMsg := initMockReqArgsRecvMsg(ctx)
	buf, err := out.Bytes()
	recvMsg.SetPayloadLen(len(buf))
	test.Assert(t, err == nil, err)
	in := remote.NewReaderBuffer(buf)
	err = payloadCodec.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil, err)

	// compare Req Arg
	sendReq := (sendMsg.Data()).(*MockReqArgs).Req
	recvReq := (recvMsg.Data()).(*MockReqArgs).Req
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

type mockFastCodecReq struct {
	num int32
	v   string
}

// sizeVarint returns the encoded size of a varint.
// The size is guaranteed to be within 1 and 10, inclusive.
func sizeVarint(v uint64) int {
	// This computes 1 + (bits.Len64(v)-1)/7.
	// 9/64 is a good enough approximation of 1/7
	return int(9*uint32(bits.Len64(v))+64) / 64
}

func (p *mockFastCodecReq) Size() (n int) {
	n += sizeVarint(uint64(p.num)<<3 | uint64(2))
	n += sizeVarint(uint64(len(p.v)))
	n += len(p.v)
	return n
}

func (p *mockFastCodecReq) FastWrite(in []byte) (n int) {
	n += binary.PutUvarint(in, uint64(p.num)<<3|uint64(2)) // Tag
	n += binary.PutUvarint(in[n:], uint64(len(p.v)))       // varint len of string
	n += copy(in[n:], p.v)
	return
}

func (p *mockFastCodecReq) FastRead(buf []byte, t int8, number int32) (int, error) {
	if t != 2 {
		panic(t)
	}
	p.num = number
	sz, n := binary.Uvarint(buf)
	buf = buf[n:]
	p.v = string(buf[:sz])
	return int(sz) + n, nil
}

func (p *mockFastCodecReq) Marshal(_ []byte) ([]byte, error) { panic("not in use") }
func (p *mockFastCodecReq) Unmarshal(_ []byte) error         { panic("not in use") }

func TestFastCodec(t *testing.T) {
	ctx := context.Background()

	req0 := &mockFastCodecReq{num: 7, v: "hello"}
	send := initSendMsg(transport.TTHeader, req0)

	buf := remote.NewReaderWriterBuffer(256)
	p := protobufCodec{}
	err := p.Marshal(ctx, send, buf)
	test.Assert(t, err == nil, err)

	b, err := buf.Bytes()
	test.Assert(t, err == nil, err)

	req1 := &mockFastCodecReq{}
	ctx, recv := initRecvMsg(ctx, req1)
	recv.SetPayloadLen(len(b))
	err = payloadCodec.Unmarshal(ctx, recv, buf)
	test.Assert(t, err == nil, err)
	test.Assert(t, req0.num == req1.num, req1.num)
	test.Assert(t, req0.v == req1.v, req1.v)
}

func TestException(t *testing.T) {
	ctx := context.Background()
	ink := rpcinfo.NewInvocation("", "mock")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, rpcinfo.NewRPCConfig(), nil)
	errInfo := "mock exception"
	transErr := remote.NewTransErrorWithMsg(remote.UnknownMethod, errInfo)
	// encode server side
	errMsg := initServerErrorMsg(transport.TTHeader, ri, transErr)
	out := remote.NewWriterBuffer(256)
	err := payloadCodec.Marshal(ctx, errMsg, out)
	test.Assert(t, err == nil, err)

	// decode client side
	recvMsg := initClientRecvMsg(ri)
	buf, err := out.Bytes()
	recvMsg.SetPayloadLen(len(buf))
	test.Assert(t, err == nil, err)
	in := remote.NewReaderBuffer(buf)
	err = payloadCodec.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err != nil)
	transErr, ok := err.(*remote.TransError)
	test.Assert(t, ok)
	test.Assert(t, err.Error() == errInfo)
	test.Assert(t, transErr.Error() == errInfo)
	test.Assert(t, transErr.TypeID() == remote.UnknownMethod)
}

func TestTransErrorUnwrap(t *testing.T) {
	errMsg := "mock err"
	transErr := remote.NewTransError(remote.InternalError, NewPbError(1000, errMsg))
	uwErr, ok := transErr.Unwrap().(PBError)
	test.Assert(t, ok)
	test.Assert(t, uwErr.TypeID() == 1000)
	test.Assert(t, transErr.Error() == errMsg)

	uwErr2, ok := errors.Unwrap(transErr).(PBError)
	test.Assert(t, ok)
	test.Assert(t, uwErr2.TypeID() == 1000)
	test.Assert(t, uwErr2.Error() == errMsg)
}

func BenchmarkNormalParallel(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// encode // client side
			sendMsg := initMockReqArgsSendMsg()
			out := remote.NewWriterBuffer(256)
			err := payloadCodec.Marshal(ctx, sendMsg, out)
			test.Assert(b, err == nil, err)

			// decode server side
			ctx, recvMsg := initMockReqArgsRecvMsg(ctx)
			buf, err := out.Bytes()
			recvMsg.SetPayloadLen(len(buf))
			test.Assert(b, err == nil, err)
			in := remote.NewReaderBuffer(buf)
			err = payloadCodec.Unmarshal(ctx, recvMsg, in)
			test.Assert(b, err == nil, err)

			// compare Req Arg
			sendReq := (sendMsg.Data()).(*MockReqArgs).Req
			recvReq := (recvMsg.Data()).(*MockReqArgs).Req
			test.Assert(b, sendReq.Msg == recvReq.Msg)
			test.Assert(b, len(sendReq.StrList) == len(recvReq.StrList))
			test.Assert(b, len(sendReq.StrMap) == len(recvReq.StrMap))
			for i, item := range sendReq.StrList {
				test.Assert(b, item == recvReq.StrList[i])
			}
			for k := range sendReq.StrMap {
				test.Assert(b, sendReq.StrMap[k] == recvReq.StrMap[k])
			}
		}
	})
}

func initMockReqArgsSendMsg() remote.Message {
	m := &MockReqArgs{}
	m.Req = prepareReq()
	return initSendMsg(transport.TTHeader, m)
}

func initSendMsg(tp transport.Protocol, m any) remote.Message {
	ink := rpcinfo.NewInvocation("", "mock")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, rpcinfo.NewRPCConfig(), nil)
	msg := remote.NewMessage(m, ri, remote.Call, remote.Client)
	mcfg := rpcinfo.AsMutableRPCConfig(ri.Config())
	mcfg.SetTransportProtocol(tp)
	mcfg.SetPayloadCodec(svcInfo.PayloadCodec)
	return msg
}

func initMockReqArgsRecvMsg(ctx context.Context) (context.Context, remote.Message) {
	m := &MockReqArgs{}
	return initRecvMsg(ctx, m)
}

func initRecvMsg(ctx context.Context, m any) (context.Context, remote.Message) {
	ink := rpcinfo.NewInvocation("", "mock")
	ri := rpcinfo.NewRPCInfo(nil, rpcinfo.EmptyEndpointInfo(), ink, rpcinfo.NewRPCConfig(), nil)
	ctx = remote.WithServiceSearcher(ctx, mocksremote.NewMockSvcSearcher(map[string]*serviceinfo.ServiceInfo{
		svcInfo.ServiceName: svcInfo,
	}))
	msg := remote.NewMessage(m, ri, remote.Call, remote.Server)
	return ctx, msg
}

func initServerErrorMsg(tp transport.Protocol, ri rpcinfo.RPCInfo, transErr *remote.TransError) remote.Message {
	errMsg := remote.NewMessage(transErr, ri, remote.Exception, remote.Server)
	mcfg := rpcinfo.AsMutableRPCConfig(ri.Config())
	mcfg.SetTransportProtocol(tp)
	mcfg.SetPayloadCodec(svcInfo.PayloadCodec)
	return errMsg
}

func initClientRecvMsg(ri rpcinfo.RPCInfo) remote.Message {
	var resp interface{}
	clientRecvMsg := remote.NewMessage(resp, ri, remote.Reply, remote.Client)
	return clientRecvMsg
}

func prepareReq() *MockReq {
	strMap := make(map[string]string)
	strMap["key1"] = "val1"
	strMap["key2"] = "val2"
	strList := []string{"str1", "str2"}
	req := &MockReq{
		Msg:     "MockReq",
		StrMap:  strMap,
		StrList: strList,
	}
	return req
}

func newMockReqArgs() interface{} {
	return &MockReqArgs{}
}

type MockReqArgs struct {
	Req *MockReq
}

func (p *MockReqArgs) Marshal(out []byte) ([]byte, error) {
	if !p.IsSetReq() {
		return out, nil
	}
	return proto.Marshal(p.Req)
}

func (p *MockReqArgs) Unmarshal(in []byte) error {
	msg := new(MockReq)
	if err := proto.Unmarshal(in, msg); err != nil {
		return err
	}
	p.Req = msg
	return nil
}

var STServiceTestObjReqArgsReqDEFAULT *MockReq

func (p *MockReqArgs) GetReq() *MockReq {
	if !p.IsSetReq() {
		return STServiceTestObjReqArgsReqDEFAULT
	}
	return p.Req
}

func (p *MockReqArgs) IsSetReq() bool {
	return p.Req != nil
}
