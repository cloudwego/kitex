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
	"errors"
	"testing"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/mocks"
	mt "github.com/cloudwego/kitex/internal/mocks/thrift/fast"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	netpolltrans "github.com/cloudwego/kitex/pkg/remote/trans/netpoll"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

var (
	payloadCodec = &thriftCodec{FastWrite | FastRead}
	svcInfo      = mocks.ServiceInfo()

	transportBuffers = []struct {
		Name      string
		NewBuffer func() remote.ByteBuffer
	}{
		{
			Name: "BytesBuffer",
			NewBuffer: func() remote.ByteBuffer {
				return remote.NewReaderWriterBuffer(1024)
			},
		},
		{
			Name: "NetpollBuffer",
			NewBuffer: func() remote.ByteBuffer {
				return netpolltrans.NewReaderWriterByteBuffer(netpoll.NewLinkBuffer(1024))
			},
		},
	}
)

func init() {
	svcInfo.Methods["mock"] = serviceinfo.NewMethodInfo(nil, newMockTestArgs, nil, false)
}

type mockWithContext struct {
	ReadFunc  func(ctx context.Context, method string, oprot thrift.TProtocol) error
	WriteFunc func(ctx context.Context, oprot thrift.TProtocol) error
}

func (m *mockWithContext) Read(ctx context.Context, method string, oprot thrift.TProtocol) error {
	if m.ReadFunc != nil {
		return m.ReadFunc(ctx, method, oprot)
	}
	return nil
}

func (m *mockWithContext) Write(ctx context.Context, oprot thrift.TProtocol) error {
	if m.WriteFunc != nil {
		return m.WriteFunc(ctx, oprot)
	}
	return nil
}

func TestWithContext(t *testing.T) {
	for _, tb := range transportBuffers {
		t.Run(tb.Name, func(t *testing.T) {
			ctx := context.Background()

			req := &mockWithContext{WriteFunc: func(ctx context.Context, oprot thrift.TProtocol) error {
				return nil
			}}
			ink := rpcinfo.NewInvocation("", "mock")
			ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
			msg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
			msg.SetProtocolInfo(remote.NewProtocolInfo(transport.TTHeader, svcInfo.PayloadCodec))
			buf := tb.NewBuffer()
			err := payloadCodec.Marshal(ctx, msg, buf)
			test.Assert(t, err == nil, err)
			buf.Flush()

			{
				resp := &mockWithContext{ReadFunc: func(ctx context.Context, method string, oprot thrift.TProtocol) error { return nil }}
				ink := rpcinfo.NewInvocation("", "mock")
				ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
				msg := remote.NewMessage(resp, svcInfo, ri, remote.Call, remote.Client)
				msg.SetProtocolInfo(remote.NewProtocolInfo(transport.TTHeader, svcInfo.PayloadCodec))
				msg.SetPayloadLen(buf.ReadableLen())
				err = payloadCodec.Unmarshal(ctx, msg, buf)
				test.Assert(t, err == nil, err)
			}
		})
	}
}

func TestNormal(t *testing.T) {
	for _, tb := range transportBuffers {
		t.Run(tb.Name, func(t *testing.T) {
			ctx := context.Background()

			// encode client side
			sendMsg := initSendMsg(transport.TTHeader)
			buf := tb.NewBuffer()
			err := payloadCodec.Marshal(ctx, sendMsg, buf)
			test.Assert(t, err == nil, err)
			buf.Flush()

			// decode server side
			recvMsg := initRecvMsg()
			recvMsg.SetPayloadLen(buf.ReadableLen())
			test.Assert(t, err == nil, err)
			err = payloadCodec.Unmarshal(ctx, recvMsg, buf)
			test.Assert(t, err == nil, err)

			// compare Req Arg
			compare(t, sendMsg, recvMsg)
		})
	}
}

func BenchmarkNormalParallel(b *testing.B) {
	for _, tb := range transportBuffers {
		b.Run(tb.Name, func(b *testing.B) {
			ctx := context.Background()

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					// encode // client side
					sendMsg := initSendMsg(transport.TTHeader)
					buf := tb.NewBuffer()
					err := payloadCodec.Marshal(ctx, sendMsg, buf)
					test.Assert(b, err == nil, err)
					buf.Flush()

					// decode server side
					recvMsg := initRecvMsg()
					recvMsg.SetPayloadLen(buf.ReadableLen())
					test.Assert(b, err == nil, err)
					err = payloadCodec.Unmarshal(ctx, recvMsg, buf)
					test.Assert(b, err == nil, err)

					// compare Req Arg
					sendReq := (sendMsg.Data()).(*mt.MockTestArgs).Req
					recvReq := (recvMsg.Data()).(*mt.MockTestArgs).Req
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
		})
	}
}

func TestException(t *testing.T) {
	for _, tb := range transportBuffers {
		t.Run(tb.Name, func(t *testing.T) {
			ctx := context.Background()
			ink := rpcinfo.NewInvocation("", "mock")
			ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
			errInfo := "mock exception"
			transErr := remote.NewTransErrorWithMsg(remote.UnknownMethod, errInfo)
			// encode server side
			errMsg := initServerErrorMsg(transport.TTHeader, ri, transErr)
			buf := tb.NewBuffer()
			err := payloadCodec.Marshal(ctx, errMsg, buf)
			test.Assert(t, err == nil, err)
			buf.Flush()

			// decode client side
			recvMsg := initClientRecvMsg(ri)
			recvMsg.SetPayloadLen(buf.ReadableLen())
			test.Assert(t, err == nil, err)
			err = payloadCodec.Unmarshal(ctx, recvMsg, buf)
			test.Assert(t, err != nil)
			transErr, ok := err.(*remote.TransError)
			test.Assert(t, ok)
			test.Assert(t, err.Error() == errInfo)
			test.Assert(t, transErr.TypeID() == remote.UnknownMethod)
		})
	}
}

func TestTransErrorUnwrap(t *testing.T) {
	errMsg := "mock err"
	transErr := remote.NewTransError(remote.InternalError, thrift.NewTApplicationException(1000, errMsg))
	uwErr, ok := transErr.Unwrap().(thrift.TApplicationException)
	test.Assert(t, ok)
	test.Assert(t, uwErr.TypeId() == 1000)
	test.Assert(t, transErr.Error() == errMsg)

	uwErr2, ok := errors.Unwrap(transErr).(thrift.TApplicationException)
	test.Assert(t, ok)
	test.Assert(t, uwErr2.TypeId() == 1000)
	test.Assert(t, uwErr2.Error() == errMsg)
}

func TestSkipDecoder(t *testing.T) {
	testcases := []struct {
		desc     string
		codec    remote.PayloadCodec
		protocol transport.Protocol
	}{
		{
			desc:     "Disable SkipDecoder, fallback to Apache Thrift Codec for Buffer Protocol",
			codec:    NewThriftCodec(),
			protocol: transport.PurePayload,
		},
		{
			desc:     "Disable SkipDecoder, using FastCodec for TTHeader Protocol",
			codec:    NewThriftCodec(),
			protocol: transport.TTHeader,
		},
		{
			desc:     "Enable SkipDecoder, using FastCodec for Buffer Protocol",
			codec:    NewThriftCodecWithConfig(FastRead | FastWrite | EnableSkipDecoder),
			protocol: transport.PurePayload,
		},
		{
			desc:     "Enable SkipDecoder, using FastCodec for TTHeader Protocol",
			codec:    NewThriftCodecWithConfig(FastRead | FastWrite | EnableSkipDecoder),
			protocol: transport.TTHeader,
		},
	}

	for _, tc := range testcases {
		for _, tb := range transportBuffers {
			t.Run(tc.desc+"#"+tb.Name, func(t *testing.T) {
				// encode client side
				sendMsg := initSendMsg(tc.protocol)
				buf := tb.NewBuffer()
				err := tc.codec.Marshal(context.Background(), sendMsg, buf)
				test.Assert(t, err == nil, err)
				buf.Flush()

				// decode server side
				recvMsg := initRecvMsg()
				if tc.protocol != transport.PurePayload {
					recvMsg.SetPayloadLen(buf.ReadableLen())
				}
				err = tc.codec.Unmarshal(context.Background(), recvMsg, buf)
				test.Assert(t, err == nil, err)

				// compare Req Arg
				compare(t, sendMsg, recvMsg)
			})
		}
	}
}

func initSendMsg(tp transport.Protocol) remote.Message {
	var _args mt.MockTestArgs
	_args.Req = prepareReq()
	ink := rpcinfo.NewInvocation("", "mock")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
	msg := remote.NewMessage(&_args, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

func initRecvMsg() remote.Message {
	var _args mt.MockTestArgs
	ink := rpcinfo.NewInvocation("", "mock")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, nil)
	msg := remote.NewMessage(&_args, svcInfo, ri, remote.Call, remote.Server)
	return msg
}

func compare(t *testing.T, sendMsg, recvMsg remote.Message) {
	sendReq := (sendMsg.Data()).(*mt.MockTestArgs).Req
	recvReq := (recvMsg.Data()).(*mt.MockTestArgs).Req
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

func initServerErrorMsg(tp transport.Protocol, ri rpcinfo.RPCInfo, transErr *remote.TransError) remote.Message {
	errMsg := remote.NewMessage(transErr, svcInfo, ri, remote.Exception, remote.Server)
	errMsg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return errMsg
}

func initClientRecvMsg(ri rpcinfo.RPCInfo) remote.Message {
	var resp interface{}
	clientRecvMsg := remote.NewMessage(resp, svcInfo, ri, remote.Reply, remote.Client)
	return clientRecvMsg
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

func newMockTestArgs() interface{} {
	return mt.NewMockTestArgs()
}
