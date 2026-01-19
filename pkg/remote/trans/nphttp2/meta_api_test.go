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

package nphttp2

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
)

func TestGetTrailerMetadataFromCtx(t *testing.T) {
	// success
	md := metadata.Pairs("k", "v")
	ctx := GRPCTrailer(context.Background(), &md)
	m := *GetTrailerMetadataFromCtx(ctx)
	test.Assert(t, len(m["k"]) == 1)
	test.Assert(t, m["k"][0] == "v")

	// failure
	m2 := GetTrailerMetadataFromCtx(context.Background())
	test.Assert(t, m2 == nil)
}

func TestGetHeaderMetadataFromCtx(t *testing.T) {
	// success
	md := metadata.Pairs("k", "v")
	ctx := GRPCHeader(context.Background(), &md)
	m := *GetHeaderMetadataFromCtx(ctx)
	test.Assert(t, len(m["k"]) == 1)
	test.Assert(t, m["k"][0] == "v")

	// failure
	m2 := GetHeaderMetadataFromCtx(context.Background())
	test.Assert(t, m2 == nil)
}

func Test_unaryMetaTracer(t *testing.T) {
	ctl := &rpcinfo.TraceController{}
	opt := newMockClientOption(ctl)
	// streaming none
	ctx := newMockCtxWithRPCInfo(serviceinfo.StreamingNone)
	verifyUnary(t, ctx, opt, ctl)
	// unary
	ctx = newMockCtxWithRPCInfo(serviceinfo.StreamingUnary)
	verifyUnary(t, ctx, opt, ctl)
	// streaming
	modes := []serviceinfo.StreamingMode{serviceinfo.StreamingClient, serviceinfo.StreamingServer, serviceinfo.StreamingBidirectional}
	for _, mode := range modes {
		ctx = newMockCtxWithRPCInfo(mode)
		verifyStreaming(t, ctx, opt, ctl)
	}
}

func verifyUnary(t *testing.T, ctx context.Context, opt *remote.ClientOption, ctl *rpcinfo.TraceController) {
	ri := rpcinfo.GetRPCInfo(ctx)
	_, err := opt.ConnPool.Get(ctx, "tcp", mockAddr0, remote.ConnOption{Dialer: opt.Dialer, ConnectTimeout: time.Second})
	test.Assert(t, err == nil, err)
	t.Run("header + trailer", func(t *testing.T) {
		hd := &metadata.MD{}
		nctx := GRPCHeader(ctx, hd)
		tl := &metadata.MD{}
		nctx = GRPCTrailer(nctx, tl)
		ctl.HandleStreamStartEvent(nctx, ri, rpcinfo.StreamStartEvent{})
		gotHd := map[string][]string{
			"key": {"val1", "val2"},
		}
		ctl.HandleStreamRecvHeaderEvent(nctx, ri, rpcinfo.StreamRecvHeaderEvent{GRPCHeader: gotHd})
		gotTl := map[string][]string{
			"KEY": {"VAL1", "VAL2"},
		}
		ctl.HandleStreamFinishEvent(nctx, ri, rpcinfo.StreamFinishEvent{GRPCTrailer: gotTl})
		test.DeepEqual(t, *hd, metadata.MD(gotHd))
		test.DeepEqual(t, *tl, metadata.MD(gotTl))
	})
	t.Run("header", func(t *testing.T) {
		hd := &metadata.MD{}
		nctx := GRPCHeader(ctx, hd)
		tl := &metadata.MD{}
		nctx = GRPCTrailer(nctx, tl)
		ctl.HandleStreamStartEvent(nctx, ri, rpcinfo.StreamStartEvent{})
		gotHd := map[string][]string{
			"key": {"val1", "val2"},
		}
		ctl.HandleStreamRecvHeaderEvent(nctx, ri, rpcinfo.StreamRecvHeaderEvent{GRPCHeader: gotHd})
		// maybe recv RstStream Frame
		ctl.HandleStreamFinishEvent(nctx, ri, rpcinfo.StreamFinishEvent{})
		test.DeepEqual(t, *hd, metadata.MD(gotHd))
		test.Assert(t, (*tl).Len() == 0, *tl)
	})
	t.Run("trailer-only", func(t *testing.T) {
		hd := &metadata.MD{}
		nctx := GRPCHeader(ctx, hd)
		tl := &metadata.MD{}
		nctx = GRPCTrailer(nctx, tl)
		ctl.HandleStreamStartEvent(nctx, ri, rpcinfo.StreamStartEvent{})
		gotTl := map[string][]string{
			"KEY": {"VAL1", "VAL2"},
		}
		ctl.HandleStreamFinishEvent(nctx, ri, rpcinfo.StreamFinishEvent{GRPCTrailer: gotTl})
		test.Assert(t, (*hd).Len() == 0, *hd)
		test.DeepEqual(t, *tl, metadata.MD(gotTl))
	})
}

func verifyStreaming(t *testing.T, ctx context.Context, opt *remote.ClientOption, ctl *rpcinfo.TraceController) {
	ri := rpcinfo.GetRPCInfo(ctx)
	_, err := opt.ConnPool.Get(ctx, "tcp", mockAddr0, remote.ConnOption{Dialer: opt.Dialer, ConnectTimeout: time.Second})
	test.Assert(t, err == nil, err)
	t.Run("header + trailer", func(t *testing.T) {
		hd := &metadata.MD{}
		nctx := GRPCHeader(ctx, hd)
		tl := &metadata.MD{}
		nctx = GRPCTrailer(nctx, tl)
		ctl.HandleStreamStartEvent(nctx, ri, rpcinfo.StreamStartEvent{})
		gotHd := map[string][]string{
			"key": {"val1", "val2"},
		}
		ctl.HandleStreamRecvHeaderEvent(nctx, ri, rpcinfo.StreamRecvHeaderEvent{GRPCHeader: gotHd})
		gotTl := map[string][]string{
			"KEY": {"VAL1", "VAL2"},
		}
		ctl.HandleStreamFinishEvent(nctx, ri, rpcinfo.StreamFinishEvent{GRPCTrailer: gotTl})
		test.Assert(t, (*hd).Len() == 0, *tl)
		test.Assert(t, (*tl).Len() == 0, *tl)
	})
	t.Run("header", func(t *testing.T) {
		hd := &metadata.MD{}
		nctx := GRPCHeader(ctx, hd)
		tl := &metadata.MD{}
		nctx = GRPCTrailer(nctx, tl)
		ctl.HandleStreamStartEvent(nctx, ri, rpcinfo.StreamStartEvent{})
		gotHd := map[string][]string{
			"key": {"val1", "val2"},
		}
		ctl.HandleStreamRecvHeaderEvent(nctx, ri, rpcinfo.StreamRecvHeaderEvent{GRPCHeader: gotHd})
		// maybe recv RstStream Frame
		ctl.HandleStreamFinishEvent(nctx, ri, rpcinfo.StreamFinishEvent{})
		test.Assert(t, (*hd).Len() == 0, *tl)
		test.Assert(t, (*tl).Len() == 0, *tl)
	})
	t.Run("trailer-only", func(t *testing.T) {
		hd := &metadata.MD{}
		nctx := GRPCHeader(ctx, hd)
		tl := &metadata.MD{}
		nctx = GRPCTrailer(nctx, tl)
		ctl.HandleStreamStartEvent(nctx, ri, rpcinfo.StreamStartEvent{})
		gotTl := map[string][]string{
			"KEY": {"VAL1", "VAL2"},
		}
		ctl.HandleStreamFinishEvent(nctx, ri, rpcinfo.StreamFinishEvent{GRPCTrailer: gotTl})
		test.Assert(t, (*hd).Len() == 0, *tl)
		test.Assert(t, (*tl).Len() == 0, *tl)
	})
}
