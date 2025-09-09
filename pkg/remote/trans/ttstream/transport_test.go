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

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/pkg/streaming"
	"github.com/cloudwego/kitex/pkg/utils"
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

func TestMain(m *testing.M) {
	// reduce cleanup interval to speed up unit tests
	ticker = utils.NewSyncSharedTicker(50 * time.Millisecond)
	m.Run()
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
	ctrans := newClientTransport(cconn, nil)
	defer ctrans.Close(nil)
	ctx := context.Background()
	cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
	test.Assert(t, err == nil, err)
	strans := newServerTransport(sconn)
	ss, err := strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)

	var wg sync.WaitGroup
	wg.Add(1)
	// client
	go func() {
		defer wg.Done()
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
	ctrans := newClientTransport(cconn, nil)
	defer ctrans.Close(nil)
	ctx := context.Background()
	cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
	test.Assert(t, err == nil, err)
	strans := newServerTransport(sconn)
	ss, err := strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)

	var wg sync.WaitGroup
	wg.Add(1)
	// client
	go func() {
		defer wg.Done()
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
	ctrans := newClientTransport(cconn, nil)
	defer ctrans.Close(nil)
	ctx := context.Background()
	cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, cs, make(IntHeader), make(streaming.Header))
	test.Assert(t, err == nil, err)
	strans := newServerTransport(sconn)
	ss, err := strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)
	res := new(testResponse)
	res.A = 123
	err = ss.SendMsg(context.Background(), res)
	test.Assert(t, err == nil, err)
	res = new(testResponse)
	err = cs.RecvMsg(context.Background(), res)
	test.Assert(t, err == nil, err)
	test.Assert(t, res.A == 123, res)

	// server send exception
	targetException := thrift.NewApplicationException(remote.InternalError, "test")
	err = ss.CloseSend(targetException)
	test.Assert(t, err == nil, err)
	// client recv exception
	res = new(testResponse)
	err = cs.RecvMsg(context.Background(), res)
	test.Assert(t, err != nil, err)
	test.Assert(t, strings.Contains(err.Error(), targetException.Msg()), err.Error())
	err = cs.CloseSend(context.Background())
	test.Assert(t, err == nil, err)
	time.Sleep(time.Millisecond * 50)

	// server send illegal frame
	ctx = context.Background()
	cs = newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, cs, make(IntHeader), make(streaming.Header))
	test.Assert(t, err == nil, err)
	ss, err = strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)
	test.Assert(t, ss != nil)
	_, err = sconn.Writer().WriteBinary([]byte("helloxxxxxxxxxxxxxxxxxxxxxx"))
	test.Assert(t, err == nil, err)
	err = sconn.Writer().Flush()
	test.Assert(t, err == nil, err)
	err = cs.RecvMsg(context.Background(), res)
	test.Assert(t, err != nil, err)
	test.Assert(t, errors.Is(err, errIllegalFrame), err)
}

func TestStreamID(t *testing.T) {
	oriId := atomic.LoadInt32(&clientStreamID)
	atomic.StoreInt32(&clientStreamID, math.MaxInt32-1)
	id := genStreamID()
	test.Assert(t, id == math.MaxInt32)
	id = genStreamID()
	test.Assert(t, id == math.MinInt32)
	atomic.StoreInt32(&clientStreamID, oriId)
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
	ctrans := newClientTransport(cconn, nil)
	defer ctrans.Close(nil)
	ctx := context.Background()
	cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: "Bidi"})
	err = ctrans.WriteStream(ctx, cs, intHeader, strHeader)
	test.Assert(t, err == nil, err)
	strans := newServerTransport(sconn)
	ss, err := strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)

	var wg sync.WaitGroup
	wg.Add(1)
	// client
	go func() {
		defer wg.Done()
		var csErr error
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
	ctrans := newClientTransport(cconn, nil)
	defer ctrans.Close(nil)

	testMethod := "Bidi"
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), rpcinfo.NewRPCInfo(
		nil, remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{Tags: make(map[string]string)}, testMethod), nil, nil, nil,
	))
	finishCh := make(chan struct{})
	cs := newClientStream(ctx, ctrans, streamFrame{sid: genStreamID(), method: testMethod})
	cs.setMetaFrameHandler(&mockMetaFrameHandler{
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
	err = ctrans.WriteStream(ctx, cs, make(IntHeader), make(streaming.Header))
	test.Assert(t, err == nil, err)

	wBuf := newWriterBuffer(sconn.Writer())
	err = EncodeFrame(context.Background(), wBuf, &Frame{
		streamFrame: streamFrame{
			// this sid should be the same as rawClientStream
			sid:    cs.sid,
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

type mockProxy struct {
	// for upstream
	upSt *serverStream
	// for downstream
	downSt     *clientStream
	nodeName   string
	finishedCh chan struct{}
}

func newMockProxy(upSt *serverStream, downSt *clientStream, nodeName string) *mockProxy {
	return &mockProxy{
		upSt:       upSt,
		downSt:     downSt,
		nodeName:   nodeName,
		finishedCh: make(chan struct{}),
	}
}

func (m *mockProxy) SendRstToUpstream(t *testing.T, ex error) {
	err := m.upSt.closeTest(ex, m.nodeName)
	test.Assert(t, err == nil, err)
}

func (m *mockProxy) SendRstToDownstream(t *testing.T, ex error) {
	m.downSt.close(ex, true, m.nodeName)
}

func (m *mockProxy) Run(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			req := new(testRequest)
			rErr := m.upSt.RecvMsg(m.upSt.ctx, req)
			if rErr != nil {
				// client -> server transition ended
				if rErr == io.EOF {
					t.Logf("[%s] upstream client -> server finished normally", m.nodeName)
					m.downSt.CloseSend(m.downSt.ctx)
				} else {
					// cascading cancel, there is no need to control the downSt
					t.Logf("[%s] upstream client -> server err: %v", m.nodeName, rErr)
				}
				return
			}
			sErr := m.downSt.SendMsg(m.downSt.ctx, req)
			if sErr == nil {
				continue
			}
			t.Logf("[%s] downstream client -> server err: %v", m.nodeName, sErr)
		}
	}()
	go func() {
		defer wg.Done()
		for {
			resp := new(testResponse)
			rErr := m.downSt.RecvMsg(m.downSt.ctx, resp)
			if rErr != nil {
				if errors.Is(rErr, errDownstreamCancel) {
					t.Logf("[%s] downstream server -> client transit rst, err: %v", m.nodeName, rErr)
					m.SendRstToUpstream(t, rErr)
					return
				}
				t.Logf("[%s] downstream server -> client finished, err: %v", m.nodeName, rErr)
				return
			}
			sErr := m.upSt.SendMsg(m.upSt.ctx, resp)
			if sErr == nil {
				continue
			}
			t.Logf("[%s] upstream server -> client err: %v", m.nodeName, sErr)
		}
	}()
	go func() {
		wg.Wait()
		t.Log("mockProxy run finished")
		close(m.finishedCh)
	}()
}

func (m *mockProxy) Wait() {
	<-m.finishedCh
}

type proxyTestSuite struct {
	aCliSt  *clientStream
	bSrvSt  *serverStream
	egress  *mockProxy
	ingress *mockProxy
	aCancel context.CancelFunc
}

// initTestStreams initializes a pair of clientStream and serverStream for test
func initTestStreams(t *testing.T, cCtx context.Context, method, cliNodeName, srvNodeName string) (*clientStream, *serverStream) {
	cfd, sfd := netpoll.GetSysFdPairs()
	cconn, err := netpoll.NewFDConnection(cfd)
	test.Assert(t, err == nil, err)
	sconn, err := netpoll.NewFDConnection(sfd)
	test.Assert(t, err == nil, err)
	t.Logf("method: %s cliNodeName: %s srvNodeName: %s | client: %s, server: %s", method, cliNodeName, srvNodeName, cconn.LocalAddr(), sconn.LocalAddr())

	intHeader := make(IntHeader)
	strHeader := make(streaming.Header)
	ctrans := newClientTransport(cconn, nil)
	cs := newClientStream(cCtx, ctrans, streamFrame{sid: genStreamID(), method: method})
	cs.rpcInfo = rpcinfo.NewRPCInfo(
		rpcinfo.NewEndpointInfo(cliNodeName, method, nil, nil), nil, nil, nil, nil)
	err = ctrans.WriteStream(cCtx, cs, intHeader, strHeader)
	test.Assert(t, err == nil, err)
	strans := newServerTransport(sconn)
	ss, err := strans.ReadStream(context.Background())
	test.Assert(t, err == nil, err)
	ss.rpcInfo = rpcinfo.NewRPCInfo(
		rpcinfo.NewEndpointInfo(srvNodeName, method, nil, nil), nil, nil, nil, nil)
	return cs, ss
}

func sendClientStreaming(t *testing.T, cliSt *clientStream) context.Context {
	return cliSt.ctx
}

func recvClientStreaming(t *testing.T, srvSt *serverStream) context.Context {
	return srvSt.ctx
}

func sendServerStreaming(t *testing.T, cliSt *clientStream) context.Context {
	req := new(testRequest)
	req.A = 1
	cCtx := cliSt.ctx
	cErr := cliSt.SendMsg(cCtx, req)
	test.Assert(t, cErr == nil, cErr)
	cErr = cliSt.CloseSend(cCtx)
	test.Assert(t, cErr == nil, cErr)
	return cCtx
}

func recvServerStreaming(t *testing.T, srvSt *serverStream) context.Context {
	req := new(testRequest)
	sCtx := srvSt.ctx
	sErr := srvSt.RecvMsg(sCtx, req)
	test.Assert(t, sErr == nil, sErr)
	return sCtx
}

var bizCancelMatrix = map[string]func(t *testing.T, cliSt *clientStream, srvSt *serverStream, cancel context.CancelFunc){
	"ClientStreaming - cancel without sending any req":          bizCancelClientStreamingCancelWithoutSendingAnyReq,
	"ClientStreaming - cancel during normal interaction":        bizCancelClientStreamingCancelDuringNormalInteraction,
	"ClientStreaming - defer cancel()":                          bizCancelClientStreamingDeferCancel,
	"ServerStreaming - cancel remote service responding slowly": bizCancelServerStreamingCancelRemoteServiceRespondingSlowly,
	"ServerStreaming - cancel during normal interaction":        bizCancelServerStreamingCancelDuringNormalInteraction,
	"ServerStreaming - defer cancel()":                          bizCancelServerStreamingDeferCancel,
	"BidiStreaming - cancel serial send recv":                   bizCancelBidiStreamingCancelSerialSendRecv,
	"BidiStreaming - cancel independent send recv":              bizCancelBidiStreamingCancelIndependentSendRecv,
}

func bizCancelClientStreamingCancelWithoutSendingAnyReq(t *testing.T, cliSt *clientStream, srvSt *serverStream, cancel context.CancelFunc) {
	var wg sync.WaitGroup
	wg.Add(1)
	sendCh := make(chan struct{})
	go func() {
		defer wg.Done()
		cCtx := sendClientStreaming(t, cliSt)
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		<-sendCh
		req := new(testRequest)
		req.A = 1
		cErr := cliSt.SendMsg(cCtx, req)
		test.Assert(t, cErr != nil)
		test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingCanceled), cErr)
		test.Assert(t, errors.Is(cErr, errBizCancel), cErr)
		t.Logf("client-side stream Send err: %v", cErr)
	}()
	sCtx := recvClientStreaming(t, srvSt)
	req := new(testRequest)
	sErr := srvSt.RecvMsg(sCtx, req)
	test.Assert(t, sErr != nil)
	test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
	test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
	t.Logf("server-side stream Recv err: %v", sErr)
	close(sendCh)
	wg.Wait()
}

func bizCancelClientStreamingCancelDuringNormalInteraction(t *testing.T, cliSt *clientStream, srvSt *serverStream, cancel context.CancelFunc) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cCtx := sendClientStreaming(t, cliSt)
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		for {
			req := new(testRequest)
			req.A = 1
			cErr := cliSt.SendMsg(cCtx, req)
			if cErr == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			t.Logf("client-side stream Send err: %v", cErr)
			return
		}
	}()
	sCtx := recvClientStreaming(t, srvSt)
	for {
		req := new(testRequest)
		sErr := srvSt.RecvMsg(sCtx, req)
		if sErr == nil {
			continue
		}
		test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
		test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
		t.Logf("server-side stream Recv err: %v", sErr)
		break
	}
	wg.Wait()
}

func bizCancelClientStreamingDeferCancel(t *testing.T, cliSt *clientStream, srvSt *serverStream, cancel context.CancelFunc) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cCtx := sendClientStreaming(t, cliSt)
		defer cancel()
		for i := 0; i < 5; i++ {
			req := new(testRequest)
			req.A = 1
			cErr := cliSt.SendMsg(cCtx, req)
			test.Assert(t, cErr == nil, cErr)
		}
	}()
	sCtx := recvClientStreaming(t, srvSt)
	for {
		req := new(testRequest)
		sErr := srvSt.RecvMsg(sCtx, req)
		if sErr == nil {
			continue
		}
		test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
		test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
		t.Logf("server-side stream Recv err: %v", sErr)
		break
	}
	wg.Wait()
}

func bizCancelServerStreamingCancelRemoteServiceRespondingSlowly(t *testing.T, cliSt *clientStream, srvSt *serverStream, cancel context.CancelFunc) {
	var wg sync.WaitGroup
	wg.Add(1)
	sendCh := make(chan struct{})
	go func() {
		defer wg.Done()
		cCtx := sendServerStreaming(t, cliSt)
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		resp := new(testResponse)
		cErr := cliSt.RecvMsg(cCtx, resp)
		test.Assert(t, cErr != nil)
		test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingCanceled), cErr)
		test.Assert(t, errors.Is(cErr, errBizCancel), cErr)
		t.Logf("client-side stream Recv err: %v", cErr)
		close(sendCh)
	}()
	sCtx := recvServerStreaming(t, srvSt)
	<-sendCh
	for {
		resp := new(testResponse)
		resp.A = 1
		sErr := srvSt.SendMsg(sCtx, resp)
		if sErr == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		t.Logf("server-side stream Send err: %v", sErr)
		test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
		test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
		break
	}
	wg.Wait()
}

func bizCancelServerStreamingCancelDuringNormalInteraction(t *testing.T, cliSt *clientStream, srvSt *serverStream, cancel context.CancelFunc) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cCtx := sendServerStreaming(t, cliSt)
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		for {
			resp := new(testResponse)
			cErr := cliSt.RecvMsg(cCtx, resp)
			if cErr == nil {
				continue
			}
			test.Assert(t, cErr != nil)
			test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingCanceled), cErr)
			test.Assert(t, errors.Is(cErr, errBizCancel), cErr)
			t.Logf("client-side stream Recv err: %v", cErr)
			return
		}
	}()
	sCtx := recvServerStreaming(t, srvSt)
	for {
		resp := new(testResponse)
		resp.A = 1
		sErr := srvSt.SendMsg(sCtx, resp)
		if sErr == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
		test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
		t.Logf("server-side stream Send err: %v", sErr)
		break
	}
	wg.Wait()
}

func bizCancelServerStreamingDeferCancel(t *testing.T, cliSt *clientStream, srvSt *serverStream, cancel context.CancelFunc) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cCtx := sendServerStreaming(t, cliSt)
		defer cancel()
		for i := 0; i < 5; i++ {
			resp := new(testResponse)
			cErr := cliSt.RecvMsg(cCtx, resp)
			test.Assert(t, cErr == nil, cErr)
		}
	}()
	sCtx := recvServerStreaming(t, srvSt)
	for {
		resp := new(testResponse)
		resp.A = 1
		sErr := srvSt.SendMsg(sCtx, resp)
		if sErr == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
		test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
		t.Logf("server-side stream Send err: %v", sErr)
		break
	}
	wg.Wait()
}

func bizCancelBidiStreamingCancelSerialSendRecv(t *testing.T, cliSt *clientStream, srvSt *serverStream, cancel context.CancelFunc) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx := cliSt.ctx
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		for {
			req := new(testRequest)
			req.A = 1
			cErr := cliSt.SendMsg(ctx, req)
			if cErr != nil {
				t.Logf("client-side stream Send err: %v", cErr)
				test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingCanceled), cErr)
				test.Assert(t, errors.Is(cErr, errBizCancel), cErr)
				return
			}
			resp := new(testResponse)
			cErr = cliSt.RecvMsg(ctx, resp)
			if cErr != nil {
				t.Logf("client-side stream Recv err: %v", cErr)
				test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingCanceled), cErr)
				test.Assert(t, errors.Is(cErr, errBizCancel), cErr)
				return
			}
		}
	}()

	ctx := srvSt.ctx
	for {
		req := new(testRequest)
		sErr := srvSt.RecvMsg(ctx, req)
		if sErr != nil {
			t.Logf("server-side stream Recv err: %v", sErr)
			test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
			test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
			break
		}
		resp := new(testResponse)
		resp.A = req.A
		sErr = srvSt.SendMsg(ctx, resp)
		if sErr != nil {
			t.Logf("server SendMsg err: %v", sErr)
			test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
			test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
			break
		}
	}

	wg.Wait()
}

func bizCancelBidiStreamingCancelIndependentSendRecv(t *testing.T, cliSt *clientStream, srvSt *serverStream, cancel context.CancelFunc) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var cliWg sync.WaitGroup
		cliWg.Add(2)
		ctx := cliSt.ctx
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		go func() {
			defer cliWg.Done()
			for {
				req := new(testRequest)
				req.A = 1
				sErr := cliSt.SendMsg(ctx, req)
				if sErr == nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				t.Logf("client-side stream Send err: %v", sErr)
				test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
				test.Assert(t, errors.Is(sErr, errBizCancel), sErr)
				break
			}
		}()
		go func() {
			defer cliWg.Done()
			for {
				resp := new(testResponse)
				rErr := cliSt.RecvMsg(ctx, resp)
				if rErr == nil {
					continue
				}
				t.Logf("client-side stream Recv err: %v", rErr)
				test.Assert(t, errors.Is(rErr, kerrors.ErrStreamingCanceled), rErr)
				test.Assert(t, errors.Is(rErr, errBizCancel), rErr)
				break
			}
		}()
		cliWg.Wait()
	}()

	ctx := srvSt.ctx
	var srvWg sync.WaitGroup
	srvWg.Add(2)
	go func() {
		defer srvWg.Done()
		for {
			req := new(testRequest)
			rErr := srvSt.RecvMsg(ctx, req)
			if rErr == nil {
				continue
			}
			t.Logf("server-side stream Recv err: %v", rErr)
			test.Assert(t, errors.Is(rErr, kerrors.ErrStreamingCanceled), rErr)
			test.Assert(t, errors.Is(rErr, errUpstreamCancel), rErr)
			break
		}
	}()
	go func() {
		defer srvWg.Done()
		for {
			resp := new(testResponse)
			resp.A = 2
			sErr := srvSt.SendMsg(ctx, resp)
			if sErr == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			t.Logf("server SendMsg err: %v", sErr)
			test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
			test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
			break
		}
	}()
	srvWg.Wait()

	wg.Wait()
}

func TestBizCancel(t *testing.T) {
	cliNodeName := "ttstream client"
	srvNodeName := "ttstream server"
	for desc, testFunc := range bizCancelMatrix {
		t.Run(desc, func(t *testing.T) {
			cCtx, cancel := context.WithCancel(context.Background())
			cliSt, srvSt := initTestStreams(t, cCtx, desc, cliNodeName, srvNodeName)
			testFunc(t, cliSt, srvSt, cancel)
		})
	}
}

func initA2Egress2B(t *testing.T, method string, withCancel bool) proxyTestSuite {
	egressName := "Egress"
	// a to egress
	var aCancel context.CancelFunc
	aCtx := context.Background()
	if withCancel {
		aCtx, aCancel = context.WithCancel(aCtx)
	}
	aCliSt, egressSrvSt := initTestStreams(t, aCtx, method, "A", egressName)
	egressCliSt, bSrvSt := initTestStreams(t, egressSrvSt.ctx, method, egressName, "B")
	proxy := newMockProxy(egressSrvSt, egressCliSt, egressName)
	proxy.Run(t)
	return proxyTestSuite{
		aCliSt:  aCliSt,
		bSrvSt:  bSrvSt,
		egress:  proxy,
		aCancel: aCancel,
	}
}

func initA2Ingress2B(t *testing.T, method string, withCancel bool) proxyTestSuite {
	ingressName := "Ingress"
	// a to ingress
	var aCancel context.CancelFunc
	aCtx := context.Background()
	if withCancel {
		aCtx, aCancel = context.WithCancel(aCtx)
	}
	aCliSt, ingressSrvSt := initTestStreams(t, aCtx, method, "A", ingressName)
	ingressCliSt, bSrvSt := initTestStreams(t, ingressSrvSt.ctx, method, ingressName, "B")
	proxy := newMockProxy(ingressSrvSt, ingressCliSt, ingressName)
	proxy.Run(t)
	return proxyTestSuite{
		aCliSt:  aCliSt,
		bSrvSt:  bSrvSt,
		ingress: proxy,
		aCancel: aCancel,
	}
}

func initA2Egress2Ingress2B(t *testing.T, method string, withCancel bool) proxyTestSuite {
	egressName := "Egress"
	ingressName := "Ingress"
	// a to egress
	var aCancel context.CancelFunc
	aCtx := context.Background()
	if withCancel {
		aCtx, aCancel = context.WithCancel(aCtx)
	}
	aCliSt, egressSrvSt := initTestStreams(t, aCtx, method, "A", egressName)
	// egress to ingress
	egressCliSt, ingressSrvSt := initTestStreams(t, egressSrvSt.ctx, method, egressName, ingressName)
	egress := newMockProxy(egressSrvSt, egressCliSt, egressName)
	egress.Run(t)
	// ingress to b
	ingressCliSt, bSrvSt := initTestStreams(t, ingressSrvSt.ctx, method, ingressName, "B")
	ingress := newMockProxy(ingressSrvSt, ingressCliSt, ingressName)
	ingress.Run(t)
	return proxyTestSuite{
		aCliSt:  aCliSt,
		bSrvSt:  bSrvSt,
		egress:  egress,
		ingress: ingress,
		aCancel: aCancel,
	}
}

var proxyCancelMatrix = map[string]func(t *testing.T, cliSt *clientStream, srvSt *serverStream){
	"ClientStreaming": proxyCancelClientStreaming,
}

func proxyCancelClientStreaming(t *testing.T, cliSt *clientStream, srvSt *serverStream) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		cCtx := sendClientStreaming(t, cliSt)
		go func() {
			time.Sleep(100 * time.Millisecond)
		}()
		for {
			req := new(testRequest)
			req.A = 1
			cErr := cliSt.SendMsg(cCtx, req)
			if cErr == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			t.Logf("client-side stream Send err: %v", cErr)
			test.Assert(t, errors.Is(cErr, kerrors.ErrStreamingCanceled), cErr)
			test.Assert(t, errors.Is(cErr, errDownstreamCancel), cErr)
			return
		}
	}()
	sCtx := recvClientStreaming(t, srvSt)
	req := new(testRequest)
	for {
		sErr := srvSt.RecvMsg(sCtx, req)
		if sErr == nil {
			continue
		}
		test.Assert(t, errors.Is(sErr, kerrors.ErrStreamingCanceled), sErr)
		test.Assert(t, errors.Is(sErr, errUpstreamCancel), sErr)
		t.Logf("server-side stream Recv err: %v", sErr)
		break
	}
	wg.Wait()
}

func TestProxyCancel(t *testing.T) {
	t.Run("A biz cancel => Egress => C", func(t *testing.T) {
		for desc, testFunc := range bizCancelMatrix {
			t.Run(desc, func(t *testing.T) {
				suite := initA2Egress2B(t, desc, true)
				testFunc(t, suite.aCliSt, suite.bSrvSt, suite.aCancel)
				suite.egress.Wait()
			})
		}
	})
	t.Run("A biz cancel => Ingress => B", func(t *testing.T) {
		for desc, testFunc := range bizCancelMatrix {
			t.Run(desc, func(t *testing.T) {
				suite := initA2Ingress2B(t, desc, true)
				testFunc(t, suite.aCliSt, suite.bSrvSt, suite.aCancel)
				suite.ingress.Wait()
			})
		}
	})
	t.Run("A biz cancel => Egress => Ingress => C", func(t *testing.T) {
		for desc, testFunc := range bizCancelMatrix {
			suite := initA2Egress2Ingress2B(t, desc, true)
			testFunc(t, suite.aCliSt, suite.bSrvSt, suite.aCancel)
			suite.egress.Wait()
			suite.ingress.Wait()
		}
	})
	t.Run("A <= Egress cancel => B", func(t *testing.T) {
		for desc, testFunc := range proxyCancelMatrix {
			suite := initA2Egress2B(t, desc, false)
			go func() {
				time.Sleep(100 * time.Millisecond)
				ex := thrift.NewApplicationException(1204, "Egress canceled")
				suite.egress.SendRstToDownstream(t, ex)
				suite.egress.SendRstToUpstream(t, ex)
			}()
			testFunc(t, suite.aCliSt, suite.bSrvSt)
			suite.egress.Wait()
		}
	})
	t.Run("A <= Ingress cancel => B", func(t *testing.T) {
		for desc, testFunc := range proxyCancelMatrix {
			suite := initA2Ingress2B(t, desc, false)
			go func() {
				time.Sleep(100 * time.Millisecond)
				ex := thrift.NewApplicationException(1204, "Ingress canceled")
				suite.ingress.SendRstToDownstream(t, ex)
				suite.ingress.SendRstToUpstream(t, ex)
			}()
			testFunc(t, suite.aCliSt, suite.bSrvSt)
			suite.ingress.Wait()
		}
	})
	t.Run("A <= Egress cancel >= Ingress => B", func(t *testing.T) {
		for desc, testFunc := range proxyCancelMatrix {
			suite := initA2Egress2Ingress2B(t, desc, false)
			go func() {
				time.Sleep(100 * time.Millisecond)
				ex := thrift.NewApplicationException(1204, "Egress canceled")
				suite.egress.SendRstToDownstream(t, ex)
				suite.egress.SendRstToUpstream(t, ex)
			}()
			testFunc(t, suite.aCliSt, suite.bSrvSt)
			suite.egress.Wait()
			suite.ingress.Wait()
		}
	})
	t.Run("A <= Egress <= Ingress cancel => B", func(t *testing.T) {
		for desc, testFunc := range proxyCancelMatrix {
			suite := initA2Egress2Ingress2B(t, desc, false)
			go func() {
				time.Sleep(100 * time.Millisecond)
				ex := thrift.NewApplicationException(1204, "Ingress canceled")
				suite.ingress.SendRstToDownstream(t, ex)
				suite.ingress.SendRstToUpstream(t, ex)
			}()
			testFunc(t, suite.aCliSt, suite.bSrvSt)
			suite.egress.Wait()
			suite.ingress.Wait()
		}
	})
}
