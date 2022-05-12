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
	"bytes"
	"context"
	stdjson "encoding/json"
	"net/http"
	"testing"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

func TestFromHTTPRequest(t *testing.T) {
	jsonBody := `{"a": 1111111111111, "b": "hello"}`
	req, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString(jsonBody))
	test.Assert(t, err == nil)
	customReq, err := FromHTTPRequest(req)
	test.Assert(t, err == nil)
	test.DeepEqual(t, customReq.Body, map[string]interface{}{
		"a": stdjson.Number("1111111111111"),
		"b": "hello",
	})
}

func TestHttpThriftCodec(t *testing.T) {
	p, err := NewThriftFileProvider("./http_test/idl/binary_echo.thrift")
	test.Assert(t, err == nil)
	htc, err := newHTTPThriftCodec(p, thriftCodec)
	test.Assert(t, err == nil)
	defer htc.Close()
	test.Assert(t, htc.Name() == "HttpThrift")

	req := &HTTPRequest{
		Method: http.MethodGet,
		Path:   "/BinaryEcho",
	}
	// wrong
	method, err := htc.getMethod("test")
	test.Assert(t, err.Error() == "req is invalid, need descriptor.HTTPRequest" && method == nil)
	// right
	method, err = htc.getMethod(req)
	test.Assert(t, err == nil && method.Name == "BinaryEcho")

	ctx := context.Background()
	sendMsg := initHttpSendMsg(transport.TTHeader)

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = htc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil)

	// UnMarshal side
	recvMsg := initHttpRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = htc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil)
}

func initHttpSendMsg(tp transport.Protocol) remote.Message {
	req := &Args{
		Request: &descriptor.HTTPRequest{
			Method: http.MethodGet,
			Path:   "/BinaryEcho",
		},
		Method: "BinaryEcho",
	}
	svcInfo := mocks.ServiceInfo()
	ink := rpcinfo.NewInvocation("", "BinaryEcho")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}

func initHttpRecvMsg() remote.Message {
	req := &Args{
		Request: "Test",
		Method:  "BinaryEcho",
	}
	ink := rpcinfo.NewInvocation("", "BinaryEcho")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
	return msg
}
