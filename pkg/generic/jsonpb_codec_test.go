/*
 * Copyright 2023 CloudWeGo Authors
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
	"io/ioutil"
	"os"
	"testing"

	"github.com/cloudwego/dynamicgo/conv"
	"github.com/cloudwego/dynamicgo/proto"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

const (
	echoIDLPath = "./jsonpb_test/idl/echo.proto"
)

func TestRun(t *testing.T) {
	t.Run("TestJsonPbCodec", TestJsonPbCodec)
}

func TestJsonPbCodec(t *testing.T) {
	p, err := initPbDescriptorProviderByIDL(echoIDLPath)
	test.Assert(t, err == nil)

	svc, err := initDynamicGoServiceDescriptorByIDL(echoIDLPath)
	test.Assert(t, err == nil)

	gOpts := &Options{dynamicgoConvOpts: conv.Options{}}

	jpc, err := newJsonPbCodec(p, svc, pbCodec, gOpts)
	test.Assert(t, err == nil)

	defer jpc.Close()
	test.Assert(t, jpc.Name() == "JSONPb")

	method, err := jpc.getMethod(nil, "Echo")
	test.Assert(t, err == nil)
	test.Assert(t, method.Name == "Echo")

	ctx := context.Background()
	sendMsg := initJsonPbSendMsg(transport.TTHeaderFramed)

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = jpc.Marshal(ctx, sendMsg, out)

	test.Assert(t, err == nil)

	// UnMarshal side
	recvMsg := initJsonPbRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = jpc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil)
}

func initJsonPbSendMsg(tp transport.Protocol) remote.Message {
	req := &Args{
		Request: `{"message": "send hello"}`,
		Method:  "Echo",
	}
	svcInfo := mocks.ServiceInfo()
	ink := rpcinfo.NewInvocation("", "Echo")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

func initJsonPbRecvMsg() remote.Message {
	req := &Args{
		Request: `{"message": "recv hello"}`,
		Method:  "Echo",
	}
	ink := rpcinfo.NewInvocation("", "Echo")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
	return msg
}

// Helper Methods
func initPbDescriptorProviderByIDL(idlpath string) (PbDescriptorProvider, error) {
	pbf, err := os.Open(idlpath)
	if err != nil {
		return nil, err
	}

	pbContent, err := ioutil.ReadAll(pbf)
	if err != nil {
		return nil, err
	}
	pbf.Close()

	pbp, err := NewPbContentProvider(idlpath, map[string]string{idlpath: string(pbContent)})
	if err != nil {
		return nil, err
	}

	return pbp, nil
}

func initDynamicGoServiceDescriptorByIDL(idlpath string) (*proto.ServiceDescriptor, error) {
	// initialise dynamicgo proto.ServiceDescriptor
	opts := proto.Options{}
	svc, err := opts.NewDescriptorFromPath(context.Background(), idlpath)
	if err != nil {
		return nil, err
	}

	return svc, nil
}
