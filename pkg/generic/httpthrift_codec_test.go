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

	"github.com/cloudwego/dynamicgo/conv"
	dhttp "github.com/cloudwego/dynamicgo/http"
	jsoniter "github.com/json-iterator/go"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/generic/descriptor"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

var json = jsoniter.Config{
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

func TestHttpThriftDynamicgoCodec(t *testing.T) {
	p, err := NewThriftFileProvider("./http_test/idl/binary_echo.thrift")
	test.Assert(t, err == nil)
	convOpts := conv.Options{EnableValueMapping: true}
	dOpts := DynamicgoOptions{ConvOpts: convOpts, EnableDynamicgoHTTPResp: true}
	htc, err := newHTTPThriftCodec(p, thriftCodec, dOpts)
	test.Assert(t, err == nil)
	defer htc.Close()
	test.Assert(t, htc.Name() == "HttpDynamicgoThrift")

	url := "http://example.com/BinaryEcho"
	// []byte value for binary field
	body := map[string]interface{}{
		"msg":        []byte("hello"),
		"got_base64": true,
		"num":        0,
	}
	data, err := json.Marshal(body)
	test.Assert(t, err == nil)
	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(data))
	test.Assert(t, err == nil)
	customReq, err := FromHTTPRequest(req, dOpts)
	test.Assert(t, err == nil)

	// wrong
	method, err := htc.getMethod("test")
	test.Assert(t, err.Error() == "req is invalid, need descriptor.HTTPRequest" && method == nil)
	// right
	method, err = htc.getMethod(customReq)
	test.Assert(t, err == nil && method.Name == "BinaryEcho")

	ctx := context.Background()
	sendMsg := initDynamicgoHttpSendMsg(transport.TTHeader)

	// Marshal side
	out := remote.NewWriterBuffer(256)
	err = htc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil)

	// UnMarshal side
	recvMsg := initDynamicgoHttpRecvMsg(transport.TTHeader)
	buf, err := out.Bytes()
	test.Assert(t, err == nil)
	recvMsg.SetPayloadLen(len(buf))
	in := remote.NewReaderBuffer(buf)
	err = htc.Unmarshal(ctx, recvMsg, in)
	test.Assert(t, err == nil)

	sendMsg = initDynamicgoHttpSendMsg(transport.PurePayload)

	// Marshal side
	out = remote.NewWriterBuffer(256)
	err = htc.Marshal(ctx, sendMsg, out)
	test.Assert(t, err == nil)

	// UnMarshal side
	recvMsg = initDynamicgoHttpRecvMsg(transport.PurePayload)
	buf, err = out.Bytes()
	test.Assert(t, err == nil)
	in = remote.NewReaderBuffer(buf)
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

func initDynamicgoHttpSendMsg(tp transport.Protocol) remote.Message {
	body := map[string]interface{}{
		"msg":        []byte("hello"),
		"got_base64": true,
		"num":        "",
	}
	data, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	stdReq, err := http.NewRequest(http.MethodGet, "/BinaryEcho", bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	stdReq.Header.Set(dhttp.HeaderContentType, descriptor.MIMEApplicationJson)
	dReq, err := dhttp.NewHTTPRequestFromStdReq(stdReq)
	if err != nil {
		panic(err)
	}
	cookies := descriptor.Cookies{}
	for _, cookie := range stdReq.Cookies() {
		cookies[cookie.Name] = cookie.Value
	}
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
			Header:      stdReq.Header,
			Query:       stdReq.URL.Query(),
			Cookies:     cookies,
			Method:      stdReq.Method,
			Host:        stdReq.Host,
			Path:        stdReq.URL.Path,
			PostForm:    stdReq.PostForm,
			Uri:         stdReq.URL.String(),
			RawBody:     rawBody,
			GeneralBody: dReq.BodyMap,
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

func initDynamicgoHttpRecvMsg(tp transport.Protocol) remote.Message {
	req := &Result{}
	ink := rpcinfo.NewInvocation("", "BinaryEcho")
	ri := rpcinfo.NewRPCInfo(nil, nil, ink, nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Call, remote.Server)
	svcInfo := mocks.ServiceInfo()
	msg.SetProtocolInfo(remote.NewProtocolInfo(tp, svcInfo.PayloadCodec))
	return msg
}
