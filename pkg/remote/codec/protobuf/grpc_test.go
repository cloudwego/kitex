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
	"testing"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

func Test_grpcCodec_Name(t *testing.T) {
	newGrpcCodec := &grpcCodec{}
	test.Assert(t, newGrpcCodec.Name() == "grpc")
}

func Test_grpcCodec_Encode_Decode(t *testing.T) {
	////remote.PutPayloadCode(serviceinfo.Protobuf, mpc)
	//newGRPCCodec := NewGRPCCodec()
	//ctx := context.Background()
	//intKVInfo := prepareIntKVInfo()
	//strKVInfo := prepareStrKVInfo()
	//sendMsg := initSendMsg(transport.GRPC)
	//sendMsg.TransInfo().TransStrInfo()
	//sendMsg.TransInfo().PutTransIntInfo(intKVInfo)
	//sendMsg.TransInfo().PutTransStrInfo(strKVInfo)
	//fmt.Println(sendMsg.Data())
	//// encode
	//out := remote.NewWriterBuffer(256)
	//err := newGRPCCodec.Encode(ctx, sendMsg, out)
	//test.Assert(t, err == nil, err)
	//
	//// decode
	//recvMsg := initRecvMsg()
	//buf, err := out.Bytes()
	//test.Assert(t, err == nil, err)
	//in := remote.NewReaderBuffer(buf)
	//err = newGRPCCodec.Decode(ctx, recvMsg, in)
	//test.Assert(t, err == nil, err)
	//
	//intKVInfoRecv := recvMsg.TransInfo().TransIntInfo()
	//strKVInfoRecv := recvMsg.TransInfo().TransStrInfo()
	//test.DeepEqual(t, intKVInfoRecv, intKVInfo)
	//test.DeepEqual(t, strKVInfoRecv, strKVInfo)
	//test.Assert(t, sendMsg.RPCInfo().Invocation().SeqID() == recvMsg.RPCInfo().Invocation().SeqID())
}

func initSendMsg2(tp transport.Protocol) remote.Message {
	var req interface{}
	svcInfo := mocks.ServiceInfo()
	ink := rpcinfo.NewInvocation("", "mock")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

func prepareIntKVInfo() map[uint16]string {
	kvInfo := map[uint16]string{
		transmeta.FromService: "mockFromService",
		transmeta.FromMethod:  "mockFromMethod",
		transmeta.ToService:   "mockToService",
		transmeta.ToMethod:    "mockToMethod",
	}
	return kvInfo
}

func prepareStrKVInfo() map[string]string {
	kvInfo := map[string]string{}
	return kvInfo
}
