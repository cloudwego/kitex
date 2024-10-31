//go:build !windows

/*
 * Copyright 2024 CloudWeGo Authors
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

package ttstream

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streamx"
	terrors "github.com/cloudwego/kitex/pkg/streamx/provider/ttstream/errors"
	"github.com/cloudwego/kitex/server/streamxserver"
)

var testServiceInfo = &serviceinfo.ServiceInfo{
	ServiceName: "kitex.service.streaming",
	Methods: map[string]serviceinfo.MethodInfo{
		"Bidi": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, reqArgs, resArgs interface{}) error {
				return streamxserver.InvokeStream[testRequest, testResponse](
					ctx, serviceinfo.StreamingBidirectional, handler.(streamx.StreamHandler), reqArgs.(streamx.StreamReqArgs), resArgs.(streamx.StreamResArgs))
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
		),
	},
	Extra: map[string]interface{}{"streaming": true, "streamx": true},
}

func TestTransport(t *testing.T) {
	klog.SetLevel(klog.LevelDebug)
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)

	intHeader := make(IntHeader)
	intHeader[0] = "test"
	strHeader := make(streamx.Header)
	strHeader["key"] = "val"
	ctrans := newTransport(clientTransport, testServiceInfo, cconn, nil)
	rawClientStream, err := ctrans.WriteStream(context.Background(), "Bidi", intHeader, strHeader)
	test.Assert(t, err == nil, err)
	strans := newTransport(serverTransport, testServiceInfo, sconn, nil)
	rawServerStream, err := strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)

	var wg sync.WaitGroup
	wg.Add(1)
	// client
	go func() {
		defer wg.Done()
		cs := newClientStream(rawClientStream)
		req := new(testRequest)
		req.B = "hello"
		err = cs.SendMsg(context.Background(), req)
		test.Assert(t, err == nil, err)
		t.Logf("client stream send msg: %v", req)

		res := new(testResponse)
		err = cs.RecvMsg(context.Background(), res)
		t.Logf("client stream recv msg: %v", res)
		test.Assert(t, err == nil, err)
		test.DeepEqual(t, req.B, res.B)

		hd, err := cs.Header()
		test.Assert(t, err == nil, err)
		test.DeepEqual(t, hd["key"], strHeader["key"])
		t.Logf("client stream recv header: %v", hd)

		err = cs.CloseSend(context.Background())
		test.Assert(t, err == nil, err)
		t.Logf("client stream close send")
	}()

	// server
	ss := newServerStream(rawServerStream)
	err = ss.SendHeader(strHeader)
	test.Assert(t, err == nil, err)
	t.Logf("server stream send header: %v", strHeader)

	req := new(testRequest)
	err = ss.RecvMsg(context.Background(), req)
	test.Assert(t, err == nil, err)
	t.Logf("server stream recv msg: %v", req)
	res := new(testResponse)
	res.B = req.B
	err = ss.SendMsg(context.Background(), res)
	test.Assert(t, err == nil, err)
	t.Logf("server stream send msg: %v", req)
	err = ss.RecvMsg(context.Background(), req)
	test.Assert(t, err == io.EOF, err)
	t.Logf("server stream recv msg: %v", res)
	wg.Wait()
}

func TestTransportException(t *testing.T) {
	klog.SetLevel(klog.LevelDebug)
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)

	ctrans := newTransport(clientTransport, testServiceInfo, cconn, nil)
	rawClientStream, err := ctrans.WriteStream(context.Background(), "Bidi", make(IntHeader), make(streamx.Header))
	test.Assert(t, err == nil, err)
	strans := newTransport(serverTransport, testServiceInfo, sconn, nil)
	rawServerStream, err := strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)

	// server send exception
	ss := newServerStream(rawServerStream)
	targetException := thrift.NewApplicationException(remote.InternalError, "test")
	err = ss.CloseSend(targetException)
	test.Assert(t, err == nil, err)
	// client recv exception
	cs := newClientStream(rawClientStream)
	res := new(testResponse)
	err = cs.RecvMsg(context.Background(), res)
	test.Assert(t, err != nil, err)
	test.Assert(t, strings.Contains(err.Error(), targetException.Msg()), err.Error())

	// server send illegal frame
	rawClientStream, err = ctrans.WriteStream(context.Background(), "Bidi", make(IntHeader), make(streamx.Header))
	test.Assert(t, err == nil, err)
	rawServerStream, err = strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)
	test.Assert(t, rawServerStream != nil, rawServerStream)
	_, err = sconn.Write([]byte("helloxxxxxxxxxxxxxxxxxxxxxx"))
	test.Assert(t, err == nil, err)
	cs = newClientStream(rawClientStream)
	err = cs.RecvMsg(context.Background(), res)
	test.Assert(t, err != nil, err)
	t.Logf("client stream send msg: %v %v", err, errors.Is(err, terrors.ErrIllegalFrame))
}
