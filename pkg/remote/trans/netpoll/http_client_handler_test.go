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

package netpoll

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/mocks"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/netpoll"
)

var (
	cliTransHdlr     remote.ClientTransHandler
	httpCilTransHdlr remote.ClientTransHandler
	cilopt           *remote.ClientOption
)

func init() {
	cilopt = &remote.ClientOption{
		SvcInfo: mocks.ServiceInfo(),
		Codec: &MockCodec{
			EncodeFunc: nil,
			DecodeFunc: nil,
		},
		Dialer: NewDialer(),
	}

	cliTransHdlr, _ = NewCliTransHandlerFactory().NewTransHandler(cilopt)
	httpCilTransHdlr, _ = NewHTTPCliTransHandlerFactory().NewTransHandler(cilopt)
}

func TestHTTPWrite(t *testing.T) {
	// 1. prepare
	conn := &MockNetpollConn{}
	rwTimeout := time.Second
	cfg := rpcinfo.NewRPCConfig()
	rpcinfo.AsMutableRPCConfig(cfg).SetReadWriteTimeout(rwTimeout)
	to := rpcinfo.NewEndpointInfo("", "", nil, map[string]string{
		"http_url": "https://example.com",
	})
	ri := rpcinfo.NewRPCInfo(nil, to, nil, cfg, rpcinfo.NewRPCStats())
	ctx := context.Background()

	// 2. test
	msg := remote.NewMessage(nil, mocks.ServiceInfo(), ri, remote.Reply, remote.Client)
	err := httpCilTransHdlr.Write(ctx, conn, msg)
	test.Assert(t, err != nil)

	msg = remote.NewMessage("is no nil", mocks.ServiceInfo(), ri, remote.Reply, remote.Client)
	err = httpCilTransHdlr.Write(ctx, conn, msg)
	test.Assert(t, err != nil)
}

func TestHTTPRead(t *testing.T) {
	rwTimeout := time.Second

	var readTimeout time.Duration
	var isReaderBufReleased bool
	conn := &MockNetpollConn{
		SetReadTimeoutFunc: func(timeout time.Duration) (e error) {
			readTimeout = timeout
			return nil
		},
		ReaderFunc: func() (r netpoll.Reader) {
			reader := &MockNetpollReader{
				ReleaseFunc: func() (err error) {
					isReaderBufReleased = true
					return nil
				},
			}
			return reader
		},
	}

	cfg := rpcinfo.NewRPCConfig()
	rpcinfo.AsMutableRPCConfig(cfg).SetReadWriteTimeout(rwTimeout)
	ri := rpcinfo.NewRPCInfo(nil, nil, nil, cfg, rpcinfo.NewRPCStats())
	ctx := context.Background()

	var req interface{}
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Reply, remote.Client)

	err := httpCilTransHdlr.Read(ctx, conn, msg)
	httpCilTransHdlr.OnError(ctx, err, conn)
	test.Assert(t, err != nil)
	test.Assert(t, readTimeout == trans.GetReadTimeout(ri.Config()))
	test.Assert(t, isReaderBufReleased)
}

func TestHTTPPanicAfterRead(t *testing.T) {
	// 1. prepare mock data
	var isOnActive bool
	svcInfo := mocks.ServiceInfo()
	conn := &MockNetpollConn{
		Conn: mocks.Conn{
			RemoteAddrFunc: func() (r net.Addr) {
				return addr
			},
		},
		IsActiveFunc: func() (r bool) {
			return true
		},
	}

	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation(svcInfo.ServiceName, method), nil, rpcinfo.NewRPCStats())
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	recvMsg := remote.NewMessageWithNewer(svcInfo, ri, remote.Call, remote.Server)
	recvMsg.NewData(method)
	sendMsg := remote.NewMessage(svcInfo.MethodInfo(method).NewResult(), svcInfo, ri, remote.Reply, remote.Server)

	// pipeline nil panic
	httpCilTransHdlr.SetPipeline(nil)

	// 2. test
	_, err := httpCilTransHdlr.OnMessage(ctx, recvMsg, sendMsg)
	test.Assert(t, err == nil, err)

	httpCilTransHdlr.OnInactive(ctx, conn)
	test.Assert(t, !isOnActive)
}

func TestAddMetaInfo(t *testing.T) {
	cfg := rpcinfo.NewRPCConfig()
	ri := rpcinfo.NewRPCInfo(nil, nil, nil, cfg, rpcinfo.NewRPCStats())
	var req interface{}
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Reply, remote.Client)
	h := http.Header{}

	err := addMetaInfo(msg, h)
	test.Assert(t, err == nil)
}

func TestReadLine(t *testing.T) {
	wantHead := "HTTP/1.1 200 OK"
	body := "{\"code\":0,\"data\":[\"mobile\",\"xxxxxxx\"],\"msg\":\"ok\"}"
	resp := []byte(wantHead + "\r\nDate: Thu, 16 Aug 2018 03:10:03 GMT\r\nKeep-Alive: timeout=5, max=100\r\nConnection: Keep-Alive\r\nTransfer-Encoding: chunked\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + body)
	reader := remote.NewReaderBuffer(resp)
	getHead, _ := readLine(reader)
	if strings.Compare(string(getHead), wantHead) != 0 {
		t.Fatal("readLine wrong")
	}
}

func TestSkipToBody(t *testing.T) {
	head := "HTTP/1.1 200 OK"
	wantBody := "{\"code\":0,\"data\":[\"mobile\",\"xxxxxxx\"],\"msg\":\"ok\"}"
	resp := []byte(head + "\r\nDate: Thu, 16 Aug 2018 03:10:03 GMT\r\nKeep-Alive: timeout=5, max=100\r\nConnection: Keep-Alive\r\nTransfer-Encoding: chunked\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + wantBody)
	reader := remote.NewReaderBuffer(resp)
	err := skipToBody(reader)
	if err != nil {
		t.Fatal(err)
	}
	getBody, err := reader.ReadBinary(reader.ReadableLen())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Compare(string(getBody), wantBody) != 0 {
		t.Fatal("skipToBody wrong")
	}
}

func TestParseHTTPResponseHead(t *testing.T) {
	head := "HTTP/1.1 200 OK"
	major, minor, statusCode, err := parseHTTPResponseHead(head)
	if err != nil {
		t.Fatal(err)
	}
	if major != 1 || minor != 1 || statusCode != 200 {
		t.Fatal("ParseHTTPResponseHead wrong")
	}
}

func TestGetBodyBufReader(t *testing.T) {
	head := "HTTP/1.1 200 OK"
	body := "{\"code\":0,\"data\":[\"mobile\",\"xxxxxxx\"],\"msg\":\"ok\"}"
	resp := []byte(head + "\r\nDate: Thu, 16 Aug 2018 03:10:03 GMT\r\nKeep-Alive: timeout=5, max=100\r\nConnection: Keep-Alive\r\nTransfer-Encoding: chunked\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + body)
	reader := remote.NewReaderBuffer(resp)
	_, err := getBodyBufReader(reader)
	test.Assert(t, err != nil)
}
