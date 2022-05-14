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
	// init
	cilopt = &remote.ClientOption{
		SvcInfo: mocks.ServiceInfo(),
		Codec: &MockCodec{
			EncodeFunc: nil,
			DecodeFunc: nil,
		},
		Dialer: NewDialer(),
	}

	// test NewCliTransHandlerFactory()
	cliTransHdlrFct := NewCliTransHandlerFactory()
	// test NewTransHandler()
	cliTransHdlr, _ = cliTransHdlrFct.NewTransHandler(cilopt)

	// test NewHTTPCliTransHandlerFactory()
	httpCilTransHdlrFct := NewHTTPCliTransHandlerFactory()
	// test NewTransHandler()
	httpCilTransHdlr, _ = httpCilTransHdlrFct.NewTransHandler(cilopt)
}

func TestHTTPWrite(t *testing.T) {
	// init
	conn := &MockNetpollConn{}
	rwTimeout := time.Second
	cfg := rpcinfo.NewRPCConfig()
	rpcinfo.AsMutableRPCConfig(cfg).SetReadWriteTimeout(rwTimeout)
	to := rpcinfo.NewEndpointInfo("", "", nil, map[string]string{
		"http_url": "https://example.com",
	})
	ri := rpcinfo.NewRPCInfo(nil, to, nil, cfg, rpcinfo.NewRPCStats())
	ctx := context.Background()

	// test Write()
	msg := remote.NewMessage(nil, mocks.ServiceInfo(), ri, remote.Reply, remote.Client)
	err := httpCilTransHdlr.Write(ctx, conn, msg)
	test.Assert(t, err != nil)

	msg = remote.NewMessage("is no nil", mocks.ServiceInfo(), ri, remote.Reply, remote.Client)
	err = httpCilTransHdlr.Write(ctx, conn, msg)
	test.Assert(t, err != nil)
}

func TestHTTPRead(t *testing.T) {
	// init
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

	// test Read()
	msg := remote.NewMessage(nil, mocks.ServiceInfo(), ri, remote.Reply, remote.Client)
	err := httpCilTransHdlr.Read(ctx, conn, msg)

	// test OnError()
	httpCilTransHdlr.OnError(ctx, err, conn)
	test.Assert(t, err != nil)
	test.Assert(t, readTimeout == trans.GetReadTimeout(ri.Config()))
	test.Assert(t, isReaderBufReleased)
}

func TestHTTPPanicAfterRead(t *testing.T) {
	// init
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

	// test SetPipeline()
	httpCilTransHdlr.SetPipeline(nil)

	// test OnMessage()
	_, err := httpCilTransHdlr.OnMessage(ctx, recvMsg, sendMsg)
	test.Assert(t, err == nil, err)

	// test OnInactive()
	httpCilTransHdlr.OnInactive(ctx, conn)
	test.Assert(t, !isOnActive)
}

func TestAddMetaInfo(t *testing.T) {
	// init
	cfg := rpcinfo.NewRPCConfig()
	ri := rpcinfo.NewRPCInfo(nil, nil, nil, cfg, rpcinfo.NewRPCStats())
	var req interface{}
	msg := remote.NewMessage(req, mocks.ServiceInfo(), ri, remote.Reply, remote.Client)
	h := http.Header{}

	// test addMetaInfo()
	err := addMetaInfo(msg, h)
	test.Assert(t, err == nil)
}

func TestReadLine(t *testing.T) {
	// init
	wantHead := "HTTP/1.1 200 OK"
	body := "{\"code\":0,\"data\":[\"mobile\",\"xxxxxxx\"],\"msg\":\"ok\"}"
	resp := []byte(wantHead + "\r\nDate: Thu, 16 Aug 2018 03:10:03 GMT\r\nKeep-Alive: timeout=5, max=100\r\nConnection: Keep-Alive\r\nTransfer-Encoding: chunked\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + body)
	reader := remote.NewReaderBuffer(resp)

	// test readLine()
	getHead, _ := readLine(reader)
	test.Assert(t, strings.Compare(string(getHead), wantHead) == 0)
}

func TestSkipToBody(t *testing.T) {
	// init
	head := "HTTP/1.1 200 OK"
	wantBody := "{\"code\":0,\"data\":[\"mobile\",\"xxxxxxx\"],\"msg\":\"ok\"}"
	resp := []byte(head + "\r\nDate: Thu, 16 Aug 2018 03:10:03 GMT\r\nKeep-Alive: timeout=5, max=100\r\nConnection: Keep-Alive\r\nTransfer-Encoding: chunked\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + wantBody)
	reader := remote.NewReaderBuffer(resp)

	// test skipToBody()
	err := skipToBody(reader)
	test.Assert(t, err == nil)
	getBody, err := reader.ReadBinary(reader.ReadableLen())
	test.Assert(t, err == nil)
	test.Assert(t, strings.Compare(string(getBody), wantBody) == 0)
}

func TestParseHTTPResponseHead(t *testing.T) {
	// init
	head := "HTTP/1.1 200 OK"

	// test parseHTTPResponseHead()
	major, minor, statusCode, err := parseHTTPResponseHead(head)
	test.Assert(t, err == nil)
	test.Assert(t, !(major != 1 || minor != 1 || statusCode != 200))
}

func TestGetBodyBufReader(t *testing.T) {
	// init
	head := "HTTP/1.1 200 OK"
	body := "{\"code\":0,\"data\":[\"mobile\",\"xxxxxxx\"],\"msg\":\"ok\"}"
	resp := []byte(head + "\r\nDate: Thu, 16 Aug 2018 03:10:03 GMT\r\nKeep-Alive: timeout=5, max=100\r\nConnection: Keep-Alive\r\nTransfer-Encoding: chunked\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + body)
	reader := remote.NewReaderBuffer(resp)

	// test getBodyBufReader
	_, err := getBodyBufReader(reader)
	test.Assert(t, err != nil)
}
