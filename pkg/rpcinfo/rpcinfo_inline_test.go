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

package rpcinfo_test

import (
	"testing"

	"github.com/cloudwego/kitex/internal"
	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/utils"
)

func TestNewRPCInfoWithInlineFields(t *testing.T) {
	rpcinfo.EnablePool(true)
	ri := rpcinfo.NewRPCInfoWithInlineFields()
	test.Assert(t, ri != nil)
	test.Assert(t, ri.From() != nil)
	test.Assert(t, ri.To() != nil)
	test.Assert(t, ri.Invocation() != nil)
	test.Assert(t, ri.Config() != nil)
	test.Assert(t, ri.Stats() != nil)

	// Test recycle
	ri.(internal.Reusable).Recycle()
}

func TestInlineRPCInfo_PutRPCInfo(t *testing.T) {
	ri := rpcinfo.NewRPCInfoWithInlineFields()

	from := rpcinfo.AsMutableEndpointInfo(ri.From())
	to := rpcinfo.AsMutableEndpointInfo(ri.To())
	invocation := ri.Invocation().(rpcinfo.InvocationSetter)
	config := rpcinfo.AsMutableRPCConfig(ri.Config())
	stats := rpcinfo.AsMutableRPCStats(ri.Stats())

	from.SetServiceName("from")
	from.SetMethod("from-method")
	from.SetAddress(utils.NewNetAddr("tcp", "127.0.0.1:8888"))
	from.SetTag("key", "value")

	to.SetServiceName("to")
	to.SetMethod("to-method")
	invocation.SetServiceName("svc")
	invocation.SetMethodName("method")
	config.SetRPCTimeout(123)
	stats.SetLevel(1)
	stats.SetSendSize(9)

	rpcinfo.PutRPCInfo(ri)

	test.Assert(t, ri.From() != nil)
	test.Assert(t, ri.To() != nil)
	test.Assert(t, ri.Invocation() != nil)
	test.Assert(t, ri.Config() != nil)
	test.Assert(t, ri.Stats() != nil)

	test.Assert(t, ri.From().ServiceName() == "")
	test.Assert(t, ri.From().Method() == "")
	test.Assert(t, ri.From().Address() == nil)
	_, exist := ri.From().Tag("key")
	test.Assert(t, !exist)

	test.Assert(t, ri.To().ServiceName() == "")
	test.Assert(t, ri.To().Method() == "")
	test.Assert(t, ri.Invocation().ServiceName() == "")
	test.Assert(t, ri.Invocation().MethodName() == "")
	test.Assert(t, ri.Config().RPCTimeout() == 0)
	test.Assert(t, ri.Stats().Level() == 0)
	test.Assert(t, ri.Stats().SendSize() == 0)
}

func BenchmarkNewRPCInfoWithInlineFields(b *testing.B) {
	rpcinfo.EnablePool(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ri := rpcinfo.NewRPCInfoWithInlineFields()
		ri.(internal.Reusable).Recycle()
	}
}

func BenchmarkNewRPCInfo(b *testing.B) {
	rpcinfo.EnablePool(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		from := rpcinfo.NewEndpointInfo("", "", nil, nil)
		to := rpcinfo.NewMutableEndpointInfo("", "", nil, nil).ImmutableView()
		ink := rpcinfo.NewInvocation("", "", "")
		cfg := rpcinfo.NewRPCConfig()
		stats := rpcinfo.NewRPCStats()
		ri := rpcinfo.NewRPCInfo(from, to, ink, cfg, stats)
		ri.(internal.Reusable).Recycle()
	}
}

func BenchmarkNewRPCInfoWithInlineFields_NoPool(b *testing.B) {
	rpcinfo.EnablePool(false)
	defer rpcinfo.EnablePool(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ri := rpcinfo.NewRPCInfoWithInlineFields()
		ri.(internal.Reusable).Recycle() // no-op when pool disabled
	}
}

func BenchmarkNewRPCInfo_NoPool(b *testing.B) {
	rpcinfo.EnablePool(false)
	defer rpcinfo.EnablePool(true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		from := rpcinfo.NewEndpointInfo("", "", nil, nil)
		to := rpcinfo.NewMutableEndpointInfo("", "", nil, nil).ImmutableView()
		ink := rpcinfo.NewInvocation("", "", "")
		cfg := rpcinfo.NewRPCConfig()
		stats := rpcinfo.NewRPCStats()
		ri := rpcinfo.NewRPCInfo(from, to, ink, cfg, stats)
		ri.(internal.Reusable).Recycle() // no-op when pool disabled
	}
}
