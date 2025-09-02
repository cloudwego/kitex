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
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/gopkg/protocol/thrift"
	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/netpoll"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/streaming"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

var testServiceInfo = &serviceinfo.ServiceInfo{
	ServiceName: "kitex.service.streaming",
	Methods: map[string]serviceinfo.MethodInfo{
		"Bidi": serviceinfo.NewMethodInfo(
			func(ctx context.Context, handler, arg, res interface{}) error {
				return nil
			},
			nil,
			nil,
			false,
			serviceinfo.WithStreamingMode(serviceinfo.StreamingBidirectional),
		),
	},
}

func TestTransportBasic(t *testing.T) {
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)

	intHeader := make(IntHeader)
	intHeader[0] = "test"
	strHeader := make(streaming.Header)
	strHeader["key"] = "val"
	ctrans := newTransport(clientTransport, cconn, nil)
	ctx := context.Background()
	rawClientStream := newStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, rawClientStream, intHeader, strHeader)
	test.Assert(t, err == nil, err)
	strans := newServerTransport(sconn, nil)
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
		err := cs.SendMsg(context.Background(), req)
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
	err = ss.CloseSend(nil)
	test.Assert(t, err == nil, err)
	t.Log("server handler return")
	wg.Wait()
}

func TestTransportServerStreaming(t *testing.T) {
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)

	intHeader := make(IntHeader)
	strHeader := make(streaming.Header)
	ctrans := newTransport(clientTransport, cconn, nil)
	ctx := context.Background()
	rawClientStream := newStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, rawClientStream, intHeader, strHeader)
	test.Assert(t, err == nil, err)
	strans := newServerTransport(sconn, nil)
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
		err := cs.SendMsg(context.Background(), req)
		test.Assert(t, err == nil, err)
		t.Logf("client stream send msg: %v", req)

		err = cs.CloseSend(context.Background())
		test.Assert(t, err == nil, err)
		t.Logf("client stream close send")

		res := new(testResponse)
		for {
			err = cs.RecvMsg(context.Background(), res)
			if err == io.EOF {
				break
			}
			test.Assert(t, err == nil, err)
		}
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
	for i := 0; i < 3; i++ {
		res := new(testResponse)
		res.B = req.B
		err = ss.SendMsg(context.Background(), res)
		test.Assert(t, err == nil, err)
		t.Logf("server stream send msg: %v", req)
	}
	err = ss.CloseSend(nil)
	test.Assert(t, err == nil, err)
	t.Log("server handler return")
	wg.Wait()
}

func TestTransportException(t *testing.T) {
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)

	// server send data
	ctrans := newTransport(clientTransport, cconn, nil)
	ctx := context.Background()
	rawClientStream := newStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, rawClientStream, make(IntHeader), make(streaming.Header))
	test.Assert(t, err == nil, err)
	strans := newServerTransport(sconn, nil)
	rawServerStream, err := strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)
	cStream := newClientStream(rawClientStream)
	sStream := newServerStream(rawServerStream)
	res := new(testResponse)
	res.A = 123
	err = sStream.SendMsg(context.Background(), res)
	test.Assert(t, err == nil, err)
	res = new(testResponse)
	err = cStream.RecvMsg(context.Background(), res)
	test.Assert(t, err == nil, err)
	test.Assert(t, res.A == 123, res)

	// server send exception
	targetException := thrift.NewApplicationException(remote.InternalError, "test")
	err = sStream.CloseSend(targetException)
	test.Assert(t, err == nil, err)
	// client recv exception
	res = new(testResponse)
	err = cStream.RecvMsg(context.Background(), res)
	test.Assert(t, err != nil, err)
	test.Assert(t, strings.Contains(err.Error(), targetException.Msg()), err.Error())
	err = cStream.CloseSend(context.Background())
	test.Assert(t, err == nil, err)
	time.Sleep(time.Millisecond * 50)

	// server send illegal frame
	ctx = context.Background()
	rawClientStream = newStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, rawClientStream, make(IntHeader), make(streaming.Header))
	test.Assert(t, err == nil, err)
	rawServerStream, err = strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)
	test.Assert(t, rawServerStream != nil, rawServerStream)
	_, err = sconn.Writer().WriteBinary([]byte("helloxxxxxxxxxxxxxxxxxxxxxx"))
	test.Assert(t, err == nil, err)
	err = sconn.Writer().Flush()
	test.Assert(t, err == nil, err)
	cStream = newClientStream(rawClientStream)
	err = cStream.RecvMsg(context.Background(), res)
	test.Assert(t, err != nil, err)
	test.Assert(t, errors.Is(err, errIllegalFrame), err)
}

func TestStreamID(t *testing.T) {
	atomic.StoreInt32(&clientStreamID, math.MaxInt32-1)
	id := genStreamID()
	test.Assert(t, id == math.MaxInt32)
	id = genStreamID()
	test.Assert(t, id == math.MinInt32)
}

func Test_clientStreamReceiveTrailer(t *testing.T) {
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)

	intHeader := make(IntHeader)
	intHeader[0] = "test"
	strHeader := make(streaming.Header)
	strHeader["key"] = "val"
	ctrans := newTransport(clientTransport, cconn, nil)
	ctx := context.Background()
	rawClientStream := newStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, rawClientStream, intHeader, strHeader)
	test.Assert(t, err == nil, err)
	strans := newServerTransport(sconn, nil)
	rawServerStream, err := strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)

	var wg sync.WaitGroup
	wg.Add(1)
	// client
	go func() {
		defer wg.Done()
		var csErr error
		cs := newClientStream(rawClientStream)
		csRes := new(testResponse)
		csErr = cs.RecvMsg(context.Background(), csRes)
		test.Assert(t, csErr != nil, csErr)

		csReq := new(testRequest)
		csReq.B = "hello"
		csErr = cs.SendMsg(context.Background(), csReq)
		test.Assert(t, csErr != nil, csErr)
	}()

	// server
	var ssErr error
	ss := newServerStream(rawServerStream)
	ssErr = ss.SendHeader(strHeader)
	test.Assert(t, ssErr == nil, ssErr)
	ssErr = ss.CloseSend(nil)
	test.Assert(t, ssErr == nil, ssErr)

	ssRes := new(testResponse)
	ssErr = ss.RecvMsg(context.Background(), ssRes)
	test.Assert(t, ssErr != nil, ssErr)
	wg.Wait()
}

type mockMetaFrameHandler struct {
	onMetaFrame func(ctx context.Context, intHeader IntHeader, header streaming.Header, payload []byte) error
}

func (hdl *mockMetaFrameHandler) OnMetaFrame(ctx context.Context, intHeader IntHeader, header streaming.Header, payload []byte) error {
	return hdl.onMetaFrame(ctx, intHeader, header, payload)
}

func Test_clientStreamReceiveMetaFrame(t *testing.T) {
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)
	ctrans := newTransport(clientTransport, cconn, nil)

	testMethod := "Bidi"
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), rpcinfo.NewRPCInfo(
		nil, remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{Tags: make(map[string]string)}, testMethod), nil, nil, nil,
	))
	finishCh := make(chan struct{})
	rawClientStream := newStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: testMethod})
	rawClientStream.setMetaFrameHandler(&mockMetaFrameHandler{
		onMetaFrame: func(ctx context.Context, intHeader IntHeader, header streaming.Header, payload []byte) error {
			ri := rpcinfo.GetRPCInfo(ctx)
			test.Assert(t, ri != nil)
			rmt := remoteinfo.AsRemoteInfo(ri.To())
			if rip := header[ttheader.HeaderTransRemoteAddr]; rip != "" {
				rmt.ForceSetTag(ttheader.HeaderTransRemoteAddr, rip)
			}
			if tc := header[ttheader.HeaderTransToCluster]; tc != "" {
				rmt.ForceSetTag(ttheader.HeaderTransToCluster, tc)
			}
			if ti := header[ttheader.HeaderTransToIDC]; ti != "" {
				rmt.ForceSetTag(ttheader.HeaderTransToIDC, ti)
			}
			close(finishCh)
			return nil
		},
	})
	err = ctrans.WriteStream(ctx, rawClientStream, make(IntHeader), make(streaming.Header))
	test.Assert(t, err == nil, err)

	wBuf := newWriterBuffer(sconn.Writer())
	err = EncodeFrame(context.Background(), wBuf, &Frame{
		streamFrame: streamFrame{
			// this sid should be the same as rawClientStream
			sid:    rawClientStream.sid,
			method: testMethod,
			header: map[string]string{
				ttheader.HeaderTransRemoteAddr: "127.0.0.1",
				ttheader.HeaderTransToCluster:  "cluster",
				ttheader.HeaderTransToIDC:      "idc",
			},
		},
		typ:     metaFrameType,
		payload: nil,
	})
	test.Assert(t, err == nil, err)
	err = wBuf.Flush()
	test.Assert(t, err == nil, err)

	<-finishCh
	// wait for MetaFrame parsed
	time.Sleep(5 * time.Millisecond)

	ri := rpcinfo.GetRPCInfo(ctx)
	rip, ok := ri.To().Tag(ttheader.HeaderTransRemoteAddr)
	test.Assert(t, ok)
	test.Assert(t, rip == "127.0.0.1")

	tc, ok := ri.To().Tag(ttheader.HeaderTransToCluster)
	test.Assert(t, ok)
	test.Assert(t, tc == "cluster")

	ti, ok := ri.To().Tag(ttheader.HeaderTransToIDC)
	test.Assert(t, ok)
	test.Assert(t, ti == "idc")
}
