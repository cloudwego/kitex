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

package rpcinfo

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
)

func TestFreezeRPCInfo(t *testing.T) {
	ri := NewRPCInfo(
		EmptyEndpointInfo(),
		EmptyEndpointInfo(),
		NewInvocation("service", "method"),
		NewRPCConfig(),
		NewRPCStats(),
	)
	ctx := NewCtxWithRPCInfo(context.Background(), ri)

	test.Assert(t, AsTaggable(ri.From()) != nil)
	test.Assert(t, AsTaggable(ri.To()) != nil)
	test.Assert(t, AsMutableEndpointInfo(ri.From()) != nil)
	test.Assert(t, AsMutableEndpointInfo(ri.To()) != nil)
	test.Assert(t, AsMutableRPCConfig(ri.Config()) != nil)
	test.Assert(t, AsMutableRPCStats(ri.Stats()) != nil)

	ctx2 := FreezeRPCInfo(ctx)
	test.Assert(t, ctx2 != ctx)
	ri2 := GetRPCInfo(ctx2)
	checkFreezeRPCInfo(t, ri2, ri)

	// call FreezeRPCInfo continuously should not cause a panic
	ctx3 := FreezeRPCInfo(ctx2)
	test.Assert(t, ctx3 != ctx2)
	ri3 := GetRPCInfo(ctx3)
	checkFreezeRPCInfo(t, ri3, ri2)
}

func TestFreezeRPCInfoKeepsValuesAfterOriginalRecycled(t *testing.T) {
	cfg := NewRPCConfig()
	mcfg := AsMutableRPCConfig(cfg)
	test.Assert(t, mcfg != nil)
	test.Assert(t, mcfg.SetRPCTimeout(time.Second) == nil)
	test.Assert(t, mcfg.SetConnectTimeout(2*time.Second) == nil)
	test.Assert(t, mcfg.SetReadWriteTimeout(3*time.Second) == nil)

	ri := NewRPCInfo(
		NewEndpointInfo("fromService", "fromMethod", nil, map[string]string{"from": "tag"}),
		NewEndpointInfo("toService", "toMethod", nil, map[string]string{"to": "tag"}),
		NewInvocation("invocationService", "invocationMethod", "invocationPackage"),
		cfg,
		NewRPCStats(),
	)
	ctx := NewCtxWithRPCInfo(context.Background(), ri)
	frozenCtx := FreezeRPCInfo(ctx)
	frozenRI := GetRPCInfo(frozenCtx)

	PutRPCInfo(ri)

	test.Assert(t, frozenRI.From().ServiceName() == "fromService")
	test.Assert(t, frozenRI.From().Method() == "fromMethod")
	fromTag, ok := frozenRI.From().Tag("from")
	test.Assert(t, ok && fromTag == "tag")
	test.Assert(t, frozenRI.To().ServiceName() == "toService")
	test.Assert(t, frozenRI.To().Method() == "toMethod")
	toTag, ok := frozenRI.To().Tag("to")
	test.Assert(t, ok && toTag == "tag")
	test.Assert(t, frozenRI.Invocation().PackageName() == "invocationPackage")
	test.Assert(t, frozenRI.Invocation().ServiceName() == "invocationService")
	test.Assert(t, frozenRI.Invocation().MethodName() == "invocationMethod")
	test.Assert(t, frozenRI.Config().RPCTimeout() == time.Second)
	test.Assert(t, frozenRI.Config().ConnectTimeout() == 2*time.Second)
	test.Assert(t, frozenRI.Config().ReadWriteTimeout() == 3*time.Second)
	test.Assert(t, frozenRI.Stats() == nil)
}

func checkFreezeRPCInfo(t *testing.T, ri, prevRI RPCInfo) {
	test.Assert(t, ri != nil)
	test.Assert(t, ri != prevRI)

	test.Assert(t, AsTaggable(ri.From()) == nil)
	test.Assert(t, AsTaggable(ri.To()) == nil)
	test.Assert(t, AsMutableEndpointInfo(ri.From()) == nil)
	test.Assert(t, AsMutableEndpointInfo(ri.To()) == nil)
	test.Assert(t, AsMutableRPCConfig(ri.Config()) == nil)
	test.Assert(t, ri.Stats() == nil)
}
