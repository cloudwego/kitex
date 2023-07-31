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

	gproto "google.golang.org/protobuf/proto"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/proto"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/codec/protobuf"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

var content = map[string]string{
	"./proto/echo.proto": `syntax='proto3';
	package proto;
	
	
	option go_package = "proto";
	
	enum TestEnum {
	  Zero = 0;
	  First = 1;
	}
	
	message Elem {
	  bool ok = 1;
	}
	
	message Message {
	  int32 tiny = 1;
	  int64 large = 2;
	  TestEnum tenum = 3;
	  string str = 4;
	  map<string, Elem> elems = 5;
	  repeated Elem els = 6;
	  // map<int32, Elem> els = 7;
	}
	
	service ExampleService {
	  rpc Echo(Message) returns(Message);
	}
	
	// protoc --go_out=$GOPATH/src kitex/pkg/generic/httppb_test/idl/echo.proto
	`,
}

func TestJsonProtobufCodec(t *testing.T) {
	protocodec := protobuf.NewProtobufCodec()
	p, err := NewPbContentProvider("./proto/echo.proto", content)
	test.Assert(t, err == nil)
	jpc, err := NewJSONToProtobufCodec(p, protocodec)
	test.Assert(t, err == nil)
	defer jpc.Close()
	test.Assert(t, jpc.Name() == "JSONToProtobuf")

	method, err := jpc.getMethod(nil, "Echo")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Echo")

	ctx := context.Background()
	sendMsg := initSendMsg(transport.TTHeader)

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = jpc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil, err)

	// Unmarshal side
	recvMsg := initRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = jpc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil, err)
}

func TestJsonProtobufCodec_SelfRef(t *testing.T) {
	protocodec := protobuf.NewProtobufCodec()
	t.Run("old way", func(t *testing.T) {
		p, err := NewPbContentProvider("./proto/echo.proto", content)
		test.Assert(t, err == nil)
		jpc, err := NewJSONToProtobufCodec(p, protocodec)
		test.Assert(t, err == nil)
		defer jpc.Close()
		test.Assert(t, jpc.Name() == "JSONToProtobuf")

		method, err := jpc.getMethod(nil, "Echo")
		test.Assert(t, err == nil)
		test.Assert(t, method.Name == "Echo")

		ctx := context.Background()
		sendMsg := initSendMsg(transport.TTHeader)

		// Marshal side
		out := remote.NewWriterBuffer(256)
		err = jpc.Marshal(ctx, sendMsg, out)
		test.Assert(t, err == nil, err)

		// Unmarshal side
		recvMsg := initRecvMsg()
		buf, err := out.Bytes()
		test.Assert(t, err == nil)
		recvMsg.SetPayloadLen(len(buf))
		in := remote.NewReaderBuffer(buf)
		err = jpc.Unmarshal(ctx, recvMsg, in)
		test.Assert(t, err == nil, err)
	})

	t.Run("old way", func(t *testing.T) {
		p, err := NewPbContentProvider("./proto/echo.proto", content)
		test.Assert(t, err == nil)
		jpc, err := NewJSONToProtobufCodec(p, protocodec)
		test.Assert(t, err == nil)
		defer jpc.Close()
		test.Assert(t, jpc.Name() == "JSONToProtobuf")

		method, err := jpc.getMethod(nil, "Echo")
		test.Assert(t, err == nil)
		test.Assert(t, method.Name == "Echo")

		ctx := context.Background()
		sendMsg := initSendMsg(transport.TTHeader)

		// Marshal side
		out := remote.NewWriterBuffer(256)
		err = jpc.Marshal(ctx, sendMsg, out)
		test.Assert(t, err == nil, err)

		// Unmarshal side
		recvMsg := initRecvMsg()
		buf, err := out.Bytes()
		test.Assert(t, err == nil)
		recvMsg.SetPayloadLen(len(buf))
		in := remote.NewReaderBuffer(buf)
		err = jpc.Unmarshal(ctx, recvMsg, in)
		test.Assert(t, err == nil, err)
	})
}

func TestProtobufExceptionError(t *testing.T) {
	protocodec := protobuf.NewProtobufCodec()
	// 	// without dynamicgo
	p, err := NewPbContentProvider("./proto/echo.proto", content)
	test.Assert(t, err == nil)
	jpc, err := NewJSONToProtobufCodec(p, protocodec)
	test.Assert(t, err == nil)

	ctx := context.Background()
	out := remote.NewWriterBuffer(256)
	// empty method test
	emptyMethodInk := rpcinfo.NewInvocation("", "")
	emptyMethodRi := rpcinfo.NewRPCInfo(nil, nil, emptyMethodInk, nil, nil)
	emptyMethodMsg := remote.NewMessage(nil, nil, emptyMethodRi, remote.Exception, remote.Client)
	err = jpc.Marshal(ctx, emptyMethodMsg, out)
	test.Assert(t, err.Error() == "empty methodName in protobuf Marshal")

	// 	// Exception MsgType test
	exceptionMsgTypeInk := rpcinfo.NewInvocation("", "Echo")
	exceptionMsgTypeRi := rpcinfo.NewRPCInfo(nil, nil, exceptionMsgTypeInk, nil, nil)
	exceptionMsgTypeMsg := remote.NewMessage(&remote.TransError{}, nil, exceptionMsgTypeRi, remote.Exception, remote.Client)
	err = jpc.Marshal(ctx, exceptionMsgTypeMsg, out)
	test.Assert(t, err == nil)
}

func initSendMsg(tp transport.Protocol) remote.Message {
	var _args MsgArgs
	_args.Req = prepareReq()
	svcInfo := newServiceInfo(serviceinfo.Protobuf)
	svcInfo.Methods["Echo"] = serviceinfo.NewMethodInfo(nil, newMsgReqArgs, nil, false)
	ink := rpcinfo.NewInvocation("", "Echo")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(&_args, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

func initRecvMsg() remote.Message {
	var _args MsgArgs
	req := &Args{
		Request: _args,
		Method:  `Echo`,
	}
	svcInfo := newServiceInfo(serviceinfo.Protobuf)
	svcInfo.Methods["Echo"] = serviceinfo.NewMethodInfo(nil, newMsgReqArgs, nil, false)
	ink := rpcinfo.NewInvocation("", "Echo")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
	msg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Server)
	return msg
}

func prepareReq() *proto.Msg {
	strMap := make(map[string]*proto.Elem)
	strMap["key1"] = &proto.Elem{Ok: true}
	elmList := []*proto.Elem{}
	req := &proto.Msg{
		Tiny:  int32(1),
		Large: int64(64),
		Tenum: proto.TestEnum(proto.TestEnum_First.Number()),
		Str:   "Hello",
		Elems: strMap,
		Els:   elmList,
	}
	return req
}

func newMsgReqArgs() interface{} {
	return &MsgArgs{}
}

type MsgArgs struct {
	Req *proto.Msg
}

func (p *MsgArgs) Marshal(out []byte) ([]byte, error) {
	// if !p.IsSetReq() {
	// 	return out, fmt.Errorf("No req in MsgArgs")
	// }
	// return gproto.Marshal(p.Req)
	return nil, nil
}

func (p *MsgArgs) Unmarshal(in []byte) error {
	msg := new(proto.Msg)
	if err := gproto.Unmarshal(in, msg); err != nil {
		return err
	}
	p.Req = msg
	return nil
}

var STServiceTestObjReqArgsReqDEFAULT *proto.Msg

func (p *MsgArgs) GetReq() *proto.Msg {
	if !p.IsSetReq() {
		return STServiceTestObjReqArgsReqDEFAULT
	}
	return p.Req
}

func (p *MsgArgs) IsSetReq() bool {
	return p.Req != nil
}
