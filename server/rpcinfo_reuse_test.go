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

package server

import (
	"net"
	"testing"

	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestPooledRPCInfoStartsCleanOnAnotherConnection(t *testing.T) {
	if !rpcinfo.PoolEnabled() {
		t.Skip("requires RPCInfo pooling")
	}
	reset := NewServer().(*server).initOrResetRPCInfoFunc()
	peerA := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7001}
	peerB := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 7002}

	previous := reset(nil, peerA)
	previous.Invocation().(rpcinfo.InvocationSetter).SetStreamingMode(serviceinfo.StreamingUnary)
	stats := rpcinfo.AsMutableRPCStats(previous.Stats())
	stats.IncrRecvSize(6)
	stats.IncrSendSize(9)

	// Supply a known pool hit before the next stream has been read.
	current := reset(previous, peerB)
	if current != previous {
		t.Fatal("expected to reuse the previous request's RPCInfo")
	}
	if got := current.From().Address(); got == nil || got.String() != peerB.String() {
		t.Errorf("new request address = %v, want %v", got, peerB)
	}
	if got := current.Stats().LastRecvSize(); got != 0 {
		t.Errorf("new request LastRecvSize = %d, want 0", got)
	}
	if got := current.Stats().LastSendSize(); got != 0 {
		t.Errorf("new request LastSendSize = %d, want 0", got)
	}
	if got := current.Invocation().StreamingMode(); got != serviceinfo.StreamingNone {
		t.Errorf("new request StreamingMode = %d, want StreamingNone", got)
	}
}
