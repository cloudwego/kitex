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
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

var customJson = sonic.Config{
	EscapeHTML: true,
	UseNumber:  true,
}.Froze()

func TestFromHTTPRequest(t *testing.T) {
	jsonBody := `{"a": 1111111111111, "b": "hello"}`
	req, err := http.NewRequest(http.MethodPost, "http://example.com", bytes.NewBufferString(jsonBody))
	test.Assert(t, err == nil)
	customReq, err := FromHTTPRequest(req)
	test.Assert(t, err == nil)
	test.DeepEqual(t, string(customReq.RawBody), jsonBody)
}

func TestHttpThriftCodec(t *testing.T) {
	// without dynamicgo
	p, err := NewThriftFileProvider("./http_test/idl/binary_echo.thrift")
	test.Assert(t, err == nil)
	var opts []Option
	htc, err := newHTTPThriftCodec(p, thriftCodec, NewOptions(opts))
	test.Assert(t, err == nil)
	test.Assert(t, !htc.dynamicgoEnabled)
	defer htc.Close()
	test.Assert(t, htc.Name() == "HttpThrift")

	req := &HTTPRequest{Request: getStdHttpRequest()}
	// wrong
	method, err := htc.getMethod("test")
	test.Assert(t, err.Error() == "req is invalid, need descriptor.HTTPRequest" && method == nil)
	// right
	method, err = htc.getMethod(req)
	test.Assert(t, err == nil && method.Name == "BinaryEcho")

	ctx := context.Background()
	sendMsg := initHttpSendMsg()

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = htc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil)

	// Unmarshal side
	recvMsg := initHttpRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = htc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil)
}

func TestHttpThriftCodecWithDynamicGo(t *testing.T) {
	// with dynamicgo
	p, err := NewThriftFileProviderWithDynamicGo("./http_test/idl/binary_echo.thrift")
	test.Assert(t, err == nil)
	var opts []Option
	opts = append(opts, EnableDynamicGoHTTPResp(true))
	htc, err := newHTTPThriftCodec(p, thriftCodec, NewOptions(opts))
	test.Assert(t, err == nil)
	test.Assert(t, htc.dynamicgoEnabled)
	defer htc.Close()
	test.Assert(t, htc.Name() == "HttpThrift")

	req := &HTTPRequest{Request: getStdHttpRequest()}
	// wrong
	method, err := htc.getMethod("test")
	test.Assert(t, err.Error() == "req is invalid, need descriptor.HTTPRequest" && method == nil)
	// right
	method, err = htc.getMethod(req)
	test.Assert(t, err == nil && method.Name == "BinaryEcho")

	ctx := context.Background()
	sendMsg := initHttpSendMsg()

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = htc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil)

	// Unmarshal side
	recvMsg := initHttpRecvMsg()
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = htc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil)
}

func initHttpSendMsg() remote.Message {
	stdReq := getStdHttpRequest()
	b, err := stdReq.GetBody()
	if err != nil {
		panic(err)
	}
	rawBody, err := ioutil.ReadAll(b)
	if err != nil {
		panic(err)
	}
	req := &Args{
		Request: &descriptor.HTTPRequest{
			Request: stdReq,
			RawBody: rawBody,
		},
		Method: "BinaryEcho",
	}
	svcInfo := mocks.ServiceInfo()
	ink := rpcinfo.NewInvocation("", "BinaryEcho")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, svcInfo, ri, remote.Call, remote.Client)
	msg.SetProtocolInfo(remote.NewProtocolInfo(transport.TTHeader, svcInfo.PayloadCodec))
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

func getStdHttpRequest() *http.Request {
	body := map[string]interface{}{
		"msg":        []byte("hello"),
		"got_base64": true,
		"num":        "",
	}
	data, err := customJson.Marshal(body)
	if err != nil {
		panic(err)
	}
	stdReq, err := http.NewRequest(http.MethodGet, "/BinaryEcho", bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	return stdReq
}
