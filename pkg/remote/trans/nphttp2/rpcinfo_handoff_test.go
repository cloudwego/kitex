/*
 * Copyright 2026 CloudWeGo Authors
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

package nphttp2

import (
	"context"
	"net"
	"testing"

	mockremote "github.com/cloudwego/kitex/internal/mocks/remote"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// handoffPool publishes the first entry and lets another request borrow it
// before Put returns. The test synchronizes both requests at that boundary.
type handoffPool struct {
	entry         interface{}
	afterFirstPut func()
}

func (p *handoffPool) Get() interface{} {
	entry := p.entry
	p.entry = nil
	return entry
}

func (p *handoffPool) Put(entry interface{}) {
	p.entry = entry
	if handoff := p.afterFirstPut; handoff != nil {
		p.afterFirstPut = nil
		handoff()
	}
}

type handoffFinishTracer func(rpcinfo.RPCInfo)

func (handoffFinishTracer) Start(ctx context.Context) context.Context { return ctx }
func (observe handoffFinishTracer) Finish(ctx context.Context)        { observe(rpcinfo.GetRPCInfo(ctx)) }

type handoffResponse struct {
	grpc.ServerTransport
	status *status.Status
	bizErr kerrors.BizStatusErrorIface
}

func (r *handoffResponse) WriteStatus(s *grpc.Stream, st *status.Status) error {
	r.status, r.bizErr = st, s.BizStatusErr()
	return nil
}

func CheckSharedRPCInfoHandoff(t *testing.T, reset func(rpcinfo.RPCInfo, net.Addr) rpcinfo.RPCInfo) {
	t.Helper()
	if !rpcinfo.PoolEnabled() {
		t.Skip("requires RPCInfo pooling")
	}
	opt := newMockServerOption()
	opt.InitOrResetRPCInfoFunc = reset
	unary := serviceinfo.NewMethodInfo(nil, nil, nil, false, serviceinfo.WithStreamingMode(serviceinfo.StreamingUnary))
	opt.SvcSearcher = mockremote.NewMockSvcSearcher(map[string]*serviceinfo.ServiceInfo{
		"Greeter": {Methods: map[string]serviceinfo.MethodInfo{"First": unary, "Second": unary}},
	})
	h, err := newSvrTransHandler(opt)
	if err != nil {
		t.Fatal(err)
	}

	type trace struct {
		method     string
		mode       serviceinfo.StreamingMode
		recv, send uint64
	}
	var finishes []trace
	opt.TracerCtl.Append(handoffFinishTracer(func(ri rpcinfo.RPCInfo) {
		finishes = append(finishes, trace{
			method: ri.Invocation().MethodName(), mode: ri.Invocation().StreamingMode(),
			recv: ri.Stats().LastRecvSize(), send: ri.Stats().LastSendSize(),
		})
	}))

	var firstRI, secondRI rpcinfo.RPCInfo
	const bizCode = 12345
	const bizMessage = "second request failed"
	secondActive := make(chan struct{})
	firstReturned := make(chan struct{})
	h.SetInvokeHandleFunc(func(ctx context.Context, _, _ interface{}) error {
		ri := rpcinfo.GetRPCInfo(ctx)
		switch ri.Invocation().MethodName() {
		case "First":
			firstRI = ri
		case "Second":
			secondRI = ri
			stats := rpcinfo.AsMutableRPCStats(ri.Stats())
			stats.IncrRecvSize(17)
			stats.IncrSendSize(23)
			ri.Invocation().(rpcinfo.InvocationSetter).SetBizStatusErr(kerrors.NewBizStatusError(bizCode, bizMessage))
			close(secondActive)
			<-firstReturned
		}
		return nil
	})
	call := func(method, addr string, transport grpc.ServerTransport) {
		s := grpc.CreateStream(context.Background(), 1, func(int) {}, "/Greeter/"+method)
		h.handleFunc(s, &SvrTrans{tr: transport}, newMockNpConn(addr))
	}

	var response handoffResponse
	secondDone := make(chan struct{})
	secondStarted := false
	h.rpcInfoPool = &handoffPool{afterFirstPut: func() {
		secondStarted = true
		go func() {
			defer close(secondDone)
			call("Second", mockAddr1, &response)
		}()
		// Let First finish its defer only after Second has written its state.
		select {
		case <-secondActive:
		case <-secondDone: // Second exited before reaching its handler.
		}
	}}
	call("First", mockAddr0, &handoffResponse{})
	close(firstReturned)
	if !secondStarted {
		t.Fatal("first request did not return its RPCInfo to the pool")
	}
	<-secondDone
	if firstRI == nil || firstRI != secondRI {
		t.Fatal("second request did not borrow the first request's RPCInfo")
	}
	want := trace{method: "Second", mode: serviceinfo.StreamingUnary, recv: 17, send: 23}
	if len(finishes) != 2 || finishes[1] != want {
		t.Errorf("Tracer.Finish recorded %+v; want second trace %+v", finishes, want)
	}
	if response.status == nil {
		t.Error("second request never called WriteStatus")
	} else if response.status.Code() != codes.Internal || response.status.Message() != bizMessage {
		t.Errorf("WriteStatus got %v %q; want Internal %q", response.status.Code(), response.status.Message(), bizMessage)
	}
	if response.bizErr == nil || response.bizErr.BizStatusCode() != bizCode {
		t.Errorf("WriteStatus got business error %v; want code %d", response.bizErr, bizCode)
	}
}
