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

	"github.com/cloudwego/dynamicgo/conv"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

func TestJsonThriftCodec(t *testing.T) {
	// without dynamicgo
	p, err := NewThriftFileProvider("./json_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	gOpts := &Options{dynamicgoConvOpts: DefaultJSONDynamicGoConvOpts}
	jtc, err := newJsonThriftCodec(p, thriftCodec, gOpts)
	test.Assert(t, err == nil)
	test.Assert(t, !jtc.dynamicgoEnabled)
	test.DeepEqual(t, jtc.convOpts, conv.Options{})
	test.DeepEqual(t, jtc.convOptsWithThriftBase, conv.Options{})
	test.DeepEqual(t, jtc.convOptsWithException, conv.Options{})
	defer jtc.Close()
	test.Assert(t, jtc.Name() == "JSONThrift")

	method, err := jtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")

	ctx := context.Background()
	sendMsg := initJsonSendMsg(transport.TTHeader)

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = jtc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil, err)

	// Unmarshal side
	recvMsg := initJsonRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = jtc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil, err)
}

func TestJsonThriftCodec_SelfRef_Old(t *testing.T) {
	t.Run("old way", func(t *testing.T) {
		p, err := NewThriftFileProvider("./json_test/idl/mock.thrift")
		test.Assert(t, err == nil)
		gOpts := &Options{dynamicgoConvOpts: DefaultJSONDynamicGoConvOpts}
		jtc, err := newJsonThriftCodec(p, thriftCodec, gOpts)
		test.Assert(t, err == nil)
		defer jtc.Close()
		test.Assert(t, jtc.Name() == "JSONThrift")

		method, err := jtc.getMethod(nil, "Test")
		test.Assert(t, err == nil)
		test.Assert(t, method.Name == "Test")

		ctx := context.Background()
		sendMsg := initJsonSendMsg(transport.TTHeader)

		// Marshal side
		out := remote.NewWriterBuffer(256)
		err = jtc.Marshal(ctx, sendMsg, out)
		test.Assert(t, err == nil, err)

		// Unmarshal side
		recvMsg := initJsonRecvMsg()
		buf, err := out.Bytes()
		test.Assert(t, err == nil)
		recvMsg.SetPayloadLen(len(buf))
		in := remote.NewReaderBuffer(buf)
		err = jtc.Unmarshal(ctx, recvMsg, in)
		test.Assert(t, err == nil, err)
	})

	t.Run("old way", func(t *testing.T) {
		p, err := NewThriftFileProviderWithDynamicGo("./json_test/idl/mock.thrift")
		test.Assert(t, err == nil)
		gOpts := &Options{dynamicgoConvOpts: DefaultJSONDynamicGoConvOpts}
		jtc, err := newJsonThriftCodec(p, thriftCodec, gOpts)
		test.Assert(t, err == nil)
		defer jtc.Close()
		test.Assert(t, jtc.Name() == "JSONThrift")

		method, err := jtc.getMethod(nil, "Test")
		test.Assert(t, err == nil)
		test.Assert(t, method.Name == "Test")

		ctx := context.Background()
		sendMsg := initJsonSendMsg(transport.TTHeader)

		// Marshal side
		out := remote.NewWriterBuffer(256)
		err = jtc.Marshal(ctx, sendMsg, out)
		test.Assert(t, err == nil, err)

		// Unmarshal side
		recvMsg := initJsonRecvMsg()
		buf, err := out.Bytes()
		test.Assert(t, err == nil)
		recvMsg.SetPayloadLen(len(buf))
		in := remote.NewReaderBuffer(buf)
		err = jtc.Unmarshal(ctx, recvMsg, in)
		test.Assert(t, err == nil, err)
	})
}

func TestJsonThriftCodecWithDynamicGo(t *testing.T) {
	// with dynamicgo
	p, err := NewThriftFileProviderWithDynamicGo("./json_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	gOpts := &Options{dynamicgoConvOpts: DefaultJSONDynamicGoConvOpts}
	jtc, err := newJsonThriftCodec(p, thriftCodec, gOpts)
	test.Assert(t, err == nil)
	test.Assert(t, jtc.dynamicgoEnabled)
	test.DeepEqual(t, jtc.convOpts, DefaultJSONDynamicGoConvOpts)
	convOptsWithThriftBase := DefaultJSONDynamicGoConvOpts
	convOptsWithThriftBase.EnableThriftBase = true
	test.DeepEqual(t, jtc.convOptsWithThriftBase, convOptsWithThriftBase)
	convOptsWithException := DefaultJSONDynamicGoConvOpts
	convOptsWithException.ConvertException = true
	test.DeepEqual(t, jtc.convOptsWithException, convOptsWithException)
	defer jtc.Close()
	test.Assert(t, jtc.Name() == "JSONThrift")

	method, err := jtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")

	ctx := context.Background()
	sendMsg := initJsonSendMsg(transport.TTHeader)

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = jtc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil, err)

	// Unmarshal side
	recvMsg := initJsonRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = jtc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil, err)

	// disable unmarshal With dynamicgo because the payload length is 0
	sendMsg = initJsonSendMsg(transport.PurePayload)

	// Marshal side
	out = remote.NewWriterBuffer(256)
	err = jtc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil)

	// Unmarshal side
	recvMsg = initJsonRecvMsg()
	buf, err = out.Bytes()
	test.Assert(t, err == nil)
	in = remote.NewReaderBuffer(buf)
	err = jtc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil)
}

func TestJsonThriftCodec_SelfRef(t *testing.T) {
	p, err := NewThriftFileProvider("./json_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	jtc, err := newJsonThriftCodec(p, thriftCodec, nil)
	test.Assert(t, err == nil)
	defer jtc.Close()
	test.Assert(t, jtc.Name() == "JSONThrift")

	method, err := jtc.getMethod(nil, "Test")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Test")

	ctx := context.Background()
	sendMsg := initJsonSendMsg(transport.TTHeader)

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = jtc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil, err)

	// Unmarshal side
	recvMsg := initJsonRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = jtc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil, err)
}

func TestJsonExceptionError(t *testing.T) {
	// without dynamicgo
	p, err := NewThriftFileProvider("./json_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	gOpts := &Options{dynamicgoConvOpts: DefaultJSONDynamicGoConvOpts}
	jtc, err := newJsonThriftCodec(p, thriftCodec, gOpts)
	test.Assert(t, err == nil)

	ctx := context.Background()
	out := remote.NewWriterBuffer(256)
	// empty method test
	emptyMethodInk := rpcinfo.NewInvocation("", "")
	emptyMethodRi := rpcinfo.NewRPCInfo(nil, nil, emptyMethodInk, nil, nil)
	emptyMethodMsg := remote.NewMessage(nil, nil, emptyMethodRi, remote.Exception, remote.Client)
	err = jtc.Marshal(ctx, emptyMethodMsg, out)
	test.Assert(t, err.Error() == "empty methodName in thrift Marshal")

	// Exception MsgType test
	exceptionMsgTypeInk := rpcinfo.NewInvocation("", "Test")
	exceptionMsgTypeRi := rpcinfo.NewRPCInfo(nil, nil, exceptionMsgTypeInk, nil, nil)
	exceptionMsgTypeMsg := remote.NewMessage(&remote.TransError{}, nil, exceptionMsgTypeRi, remote.Exception, remote.Client)
	err = jtc.Marshal(ctx, exceptionMsgTypeMsg, out)
	test.Assert(t, err == nil)

	// with dynamicgo
	p, err = NewThriftFileProviderWithDynamicGo("./json_test/idl/mock.thrift")
	test.Assert(t, err == nil)
	jtc, err = newJsonThriftCodec(p, thriftCodec, gOpts)
	test.Assert(t, err == nil)
	// empty method test
	err = jtc.Marshal(ctx, emptyMethodMsg, out)
	test.Assert(t, err.Error() == "empty methodName in thrift Marshal")
	// Exception MsgType test
	err = jtc.Marshal(ctx, exceptionMsgTypeMsg, out)
	test.Assert(t, err == nil)
}

func initJsonSendMsg(tp transport.Protocol) remote.Message {
	req := &Args{
		Request: `{"extra": "Hello"}`,
		Method:  "Test",
	}
	svcInfo := mocks.ServiceInfo()
	ink := rpcinfo.NewInvocation("", "Test")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

func initJsonRecvMsg() remote.Message {
	req := &Args{
		Request: "Test",
		Method:  "Test",
	}
	ink := rpcinfo.NewInvocation("", "Test")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
	return msg
}
