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
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/grpc"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

// CheckPooledRPCInfoAtStreamStart checks the next borrower sees a clean invocation, stats and peer.
func CheckPooledRPCInfoAtStreamStart(t *testing.T, reset func(rpcinfo.RPCInfo, net.Addr) rpcinfo.RPCInfo) {
	t.Helper()
	setRPCInfoPoolEnabled(t, true)
	connA, connB := newMockNpConn(mockAddr0), newMockNpConn(mockAddr1)
	var previous rpcinfo.RPCInfo
	opt := newMockServerOption()
	opt.GRPCSharedRPCInfoPool = true
	opt.InitOrResetRPCInfoFunc = reset
	opt.SvcSearcher = mockremote.NewMockSvcSearcher(map[string]*serviceinfo.ServiceInfo{
		"Greeter": {Methods: map[string]serviceinfo.MethodInfo{
			"Previous": serviceinfo.NewMethodInfo(nil, nil, nil, false, serviceinfo.WithStreamingMode(serviceinfo.StreamingUnary)),
			"Current":  serviceinfo.NewMethodInfo(nil, nil, nil, false, serviceinfo.WithStreamingMode(serviceinfo.StreamingNone)),
		}},
	})

	var observedCurrent bool
	opt.StreamingMetaHandlers = append(opt.StreamingMetaHandlers, &mockMetaHandler{
		onReadStream: func(ctx context.Context) (context.Context, error) {
			ri := rpcinfo.GetRPCInfo(ctx)
			if ri.Invocation().MethodName() != "Current" {
				return ctx, nil
			}
			observedCurrent = true
			if previous == nil {
				t.Fatal("first request did not run")
			}
			if ri != previous {
				t.Fatal("second connection did not borrow the first request's RPCInfo")
			}
			if got := ri.From().Address(); got == nil || got.String() != mockAddr1 {
				t.Errorf("current stream peer = %v, want %s", got, mockAddr1)
			}
			if n := ri.Stats().LastRecvSize(); n != 0 {
				t.Errorf("OnReadStream LastRecvSize = %d, want 0", n)
			}
			if n := ri.Stats().LastSendSize(); n != 0 {
				t.Errorf("OnReadStream LastSendSize = %d, want 0", n)
			}
			if mode := ri.Invocation().StreamingMode(); mode != serviceinfo.StreamingNone {
				t.Errorf("OnReadStream StreamingMode = %d, want StreamingNone", mode)
			}
			return ctx, nil
		},
	})
	handler, err := newSvrTransHandler(opt)
	if err != nil {
		t.Fatal(err)
	}
	handler.sharedRPCInfoPool = new(handoffPool)
	handler.SetInvokeHandleFunc(func(ctx context.Context, _, _ interface{}) error {
		ri := rpcinfo.GetRPCInfo(ctx)
		if ri.Invocation().MethodName() == "Previous" {
			previous = ri
			if ri.Invocation().StreamingMode() != serviceinfo.StreamingUnary {
				t.Fatal("previous stream did not obtain the expected RPCInfo and method mode")
			}
			stats := rpcinfo.AsMutableRPCStats(ri.Stats())
			stats.IncrRecvSize(6)
			stats.IncrSendSize(9)
		}
		return nil
	})
	for _, call := range []struct {
		method string
		conn   *mockNetpollConn
	}{{"Previous", connA}, {"Current", connB}} {
		stream := grpc.CreateStream(context.Background(), 1, func(int) {}, "/Greeter/"+call.method)
		handler.handleFunc(stream, activateRPCInfoTestConn(t, handler, call.conn), call.conn)
	}
	if !observedCurrent {
		t.Fatal("current stream did not reach OnReadStream")
	}
}
