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

package transmeta

import (
	"context"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"

	"github.com/cloudwego/kitex/internal/test"
	"github.com/cloudwego/kitex/pkg/logid"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

func TestClientReadMetainfo(t *testing.T) {
	ctx := context.Background()
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), nil, rpcinfo.NewRPCStats())
	msg := remote.NewMessage(nil, ri, remote.Call, remote.Client)
	hd := map[string]string{
		"hello": "world",
	}
	msg.TransInfo().PutTransStrInfo(hd)

	var err error
	ctx, err = MetainfoClientHandler.ReadMeta(ctx, msg)
	test.Assert(t, err == nil)

	kvs := metainfo.RecvAllBackwardValues(ctx)
	test.Assert(t, len(kvs) == 0)

	ctx = metainfo.WithBackwardValues(context.Background())
	ctx, err = MetainfoClientHandler.ReadMeta(ctx, msg)
	test.Assert(t, err == nil)

	kvs = metainfo.RecvAllBackwardValues(ctx)
	test.Assert(t, len(kvs) == 1)
	test.Assert(t, kvs["hello"] == "world")
}

func TestClientWriteMetainfo(t *testing.T) {
	SetMetainfoPropagationMode(MetainfoPropagationLegacy)

	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	ctx := context.Background()
	ctx = metainfo.WithValue(ctx, "tk", "tv")
	ctx = metainfo.WithPersistentValue(ctx, "pk", "pv")
	msg := remote.NewMessage(nil, ri, remote.Call, remote.Client)

	mcfg := rpcinfo.AsMutableRPCConfig(ri.Config())
	mcfg.SetTransportProtocol(transport.PurePayload)
	mcfg.SetPayloadCodec(serviceinfo.Thrift)
	ctx, err := MetainfoClientHandler.WriteMeta(ctx, msg)
	test.Assert(t, err == nil)

	kvs := msg.TransInfo().TransStrInfo()
	test.Assert(t, len(kvs) == 2, kvs)
	test.Assert(t, kvs[metainfo.PrefixTransient+"tk"] == "tv")
	test.Assert(t, kvs[metainfo.PrefixPersistent+"pk"] == "pv")

	mcfg.SetTransportProtocol(transport.TTHeader)
	_, err = MetainfoClientHandler.WriteMeta(ctx, msg)
	test.Assert(t, err == nil)

	kvs = msg.TransInfo().TransStrInfo()
	test.Assert(t, len(kvs) == 2, kvs)
	test.Assert(t, kvs[metainfo.PrefixTransient+"tk"] == "tv")
	test.Assert(t, kvs[metainfo.PrefixPersistent+"pk"] == "pv")
}

func TestServerReadMetainfo(t *testing.T) {
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	ctx0 := context.Background()
	msg := remote.NewMessage(nil, ri, remote.Call, remote.Client)

	hd := map[string]string{
		"hello":                          "world",
		metainfo.PrefixTransient + "tk":  "tv",
		metainfo.PrefixPersistent + "pk": "pv",
	}
	msg.TransInfo().PutTransStrInfo(hd)

	mcfg := rpcinfo.AsMutableRPCConfig(ri.Config())
	mcfg.SetTransportProtocol(transport.PurePayload)
	mcfg.SetPayloadCodec(serviceinfo.Thrift)
	ctx, err := MetainfoServerHandler.ReadMeta(ctx0, msg)
	tvs := metainfo.GetAllValues(ctx)
	pvs := metainfo.GetAllPersistentValues(ctx)
	test.Assert(t, err == nil)
	test.Assert(t, len(tvs) == 1 && tvs["tk"] == "tv", tvs)
	test.Assert(t, len(pvs) == 1 && pvs["pk"] == "pv")

	mcfg.SetTransportProtocol(transport.TTHeader)
	ctx, err = MetainfoServerHandler.ReadMeta(ctx0, msg)
	ctx = metainfo.TransferForward(ctx)
	tvs = metainfo.GetAllValues(ctx)
	pvs = metainfo.GetAllPersistentValues(ctx)
	test.Assert(t, err == nil)
	test.Assert(t, len(tvs) == 1 && tvs["tk"] == "tv", tvs)
	test.Assert(t, len(pvs) == 1 && pvs["pk"] == "pv")

	ctx = metainfo.TransferForward(ctx)
	tvs = metainfo.GetAllValues(ctx)
	pvs = metainfo.GetAllPersistentValues(ctx)
	test.Assert(t, len(tvs) == 0, len(tvs))
	test.Assert(t, len(pvs) == 1 && pvs["pk"] == "pv")
}

func TestServerWriteMetainfo(t *testing.T) {
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
	msg := remote.NewMessage(nil, ri, remote.Call, remote.Client)

	ctx := context.Background()
	ctx = metainfo.WithBackwardValuesToSend(ctx)
	ctx = metainfo.WithValue(ctx, "tk", "tv")
	ctx = metainfo.WithPersistentValue(ctx, "pk", "pv")
	ok := metainfo.SendBackwardValue(ctx, "bk", "bv")
	test.Assert(t, ok)

	mcfg := rpcinfo.AsMutableRPCConfig(ri.Config())
	mcfg.SetTransportProtocol(transport.PurePayload)
	mcfg.SetPayloadCodec(serviceinfo.Thrift)
	ctx, err := MetainfoServerHandler.WriteMeta(ctx, msg)
	test.Assert(t, err == nil)
	kvs := msg.TransInfo().TransStrInfo()
	test.Assert(t, len(kvs) == 1 && kvs["bk"] == "bv", kvs)

	mcfg.SetTransportProtocol(transport.TTHeader)
	_, err = MetainfoServerHandler.WriteMeta(ctx, msg)
	test.Assert(t, err == nil)
	kvs = msg.TransInfo().TransStrInfo()
	test.Assert(t, len(kvs) == 1 && kvs["bk"] == "bv", kvs)
}

func Test_addStreamID(t *testing.T) {
	t.Run("without-stream-log-id", func(t *testing.T) {
		md := metadata.MD{
			transmeta.HTTPStreamLogID: nil,
		}
		ctx := context.Background()
		ctx = addStreamIDToContext(ctx, md)
		logID := logid.GetStreamLogID(ctx)
		test.Assert(t, logID == "", logID) // won't generate a new one
	})

	t.Run("with-stream-log-id", func(t *testing.T) {
		md := metadata.MD{
			transmeta.HTTPStreamLogID: []string{"test"},
		}
		ctx := context.Background()
		ctx = addStreamIDToContext(ctx, md)
		logID := logid.GetStreamLogID(ctx)
		test.Assert(t, logID == "test", logID)
	})
}

func TestMetainfoPropagation(t *testing.T) {
	defer SetMetainfoPropagationModeProvider(nil)
	defer SetMetainfoPropagationMode(MetainfoPropagationLegacy)

	ctxA := context.Background()
	ctxA = metainfo.WithValue(ctxA, "TK", "tv")
	ctxA = metainfo.WithPersistentValue(ctxA, "PK", "pv")

	saveToMap := func(ctx context.Context) map[string]string {
		h := make(map[string]string)
		metainfo.SaveMetaInfoToMap(ctx, h)
		return h
	}
	newInboundCtx := func(h map[string]string) context.Context {
		return TransferForwardMetaInfo(metainfo.SetMetaInfoFromMap(context.Background(), h))
	}
	connect := func(ctx context.Context) metadata.MD {
		ctx, err := MetainfoClientHandler.OnConnectStream(ctx)
		test.Assert(t, err == nil)
		md, ok := metadata.FromOutgoingContext(ctx)
		test.Assert(t, ok)
		return md
	}
	writeMeta := func(ctx context.Context) map[string]string {
		ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
		msg := remote.NewMessage(nil, ri, remote.Call, remote.Client)
		_, err := MetainfoClientHandler.WriteMeta(ctx, msg)
		test.Assert(t, err == nil)
		return msg.TransInfo().TransStrInfo()
	}

	headerAB := saveToMap(ctxA)
	test.Assert(t, headerAB[metainfo.PrefixTransient+"TK"] == "tv", headerAB)

	// Legacy mode preserves historical upstream transient forwarding.
	SetMetainfoPropagationMode(MetainfoPropagationLegacy)
	ctxB := newInboundCtx(headerAB)
	got, ok := metainfo.GetValue(ctxB, "TK")
	test.Assert(t, ok && got == "tv", got, ok)
	headerBC := saveToMap(ctxB)
	test.Assert(t, headerBC[metainfo.PrefixTransient+"TK"] == "tv",
		"legacy mode should preserve historical upstream transient forwarding, headerBC=", headerBC)

	// Single-hop mode filters upstream transient values unless B sets them again.
	SetMetainfoPropagationMode(MetainfoPropagationSingleHop)
	ctxB = newInboundCtx(headerAB)
	ctxB = metainfo.WithPersistentValue(ctxB, "PK", "pv")
	headerBC = saveToMap(ctxB)
	_, leaked := headerBC[metainfo.PrefixTransient+"TK"]
	test.Assert(t, !leaked, headerBC)
	test.Assert(t, headerBC[metainfo.PrefixPersistent+"PK"] == "pv", headerBC)

	ctxB = metainfo.WithValue(ctxB, "TK", "tv2")
	headerBC = saveToMap(ctxB)
	test.Assert(t, headerBC[metainfo.PrefixTransient+"TK"] == "tv2", headerBC)

	ctxClient := context.Background()
	ctxClient = metainfo.WithValue(ctxClient, "tk", "tv")
	ctxClient = metainfo.WithPersistentValue(ctxClient, "pk", "pv")
	ctxClient = metainfo.TransferForward(ctxClient)
	kvs := writeMeta(ctxClient)
	test.Assert(t, len(kvs) == 1, kvs)
	test.Assert(t, kvs[metainfo.PrefixTransient+"tk"] == "", kvs)
	test.Assert(t, kvs[metainfo.PrefixPersistent+"pk"] == "pv", kvs)

	ctxClient = metainfo.WithValue(ctxClient, "tk", "tv2")
	kvs = writeMeta(ctxClient)
	test.Assert(t, len(kvs) == 2, kvs)
	test.Assert(t, kvs[metainfo.PrefixTransient+"tk"] == "tv2", kvs)
	test.Assert(t, kvs[metainfo.PrefixPersistent+"pk"] == "pv", kvs)

	// HTTP/2 metadata follows the same single-hop rule and preserves
	// unrelated outgoing metadata.
	cfg := rpcinfo.NewRPCConfig()
	rpcinfo.AsMutableRPCConfig(cfg).SetTransportProtocol(transport.GRPC)
	ri := rpcinfo.NewRPCInfo(nil, nil, rpcinfo.NewInvocation("", ""), cfg, rpcinfo.NewRPCStats())
	ctx := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"x-normal", "kept",
	))
	ctx = metainfo.WithValue(ctx, "TK", "tv")
	ctx = metainfo.WithPersistentValue(ctx, "PK", "pv")
	ctx = metainfo.TransferForward(ctx)

	md := connect(ctx)
	test.Assert(t, len(md.Get(metainfo.HTTPPrefixTransient+"TK")) == 0, md)
	test.Assert(t, md.Get(metainfo.HTTPPrefixPersistent + "PK")[0] == "pv", md)
	test.Assert(t, md.Get("x-normal")[0] == "kept", md)

	ctx = metainfo.WithValue(ctx, "TK", "tv2")
	md = connect(ctx)
	test.Assert(t, md.Get(metainfo.HTTPPrefixTransient + "TK")[0] == "tv2", md)
	test.Assert(t, md.Get(metainfo.HTTPPrefixPersistent + "PK")[0] == "pv", md)

	// Legacy mode serializes upstream transient values to HTTP/2 metadata as-is.
	SetMetainfoPropagationMode(MetainfoPropagationLegacy)
	ctxLegacy := rpcinfo.NewCtxWithRPCInfo(context.Background(), ri)
	ctxLegacy = metainfo.WithValue(ctxLegacy, "TK", "tv")
	ctxLegacy = metainfo.WithPersistentValue(ctxLegacy, "PK", "pv")
	md = connect(ctxLegacy)
	test.Assert(t, md.Get(metainfo.HTTPPrefixTransient + "TK")[0] == "tv", md)
	test.Assert(t, md.Get(metainfo.HTTPPrefixPersistent + "PK")[0] == "pv", md)

	// Provider takes precedence over the process default.
	SetMetainfoPropagationMode(MetainfoPropagationLegacy)
	SetMetainfoPropagationModeProvider(func(context.Context) MetainfoPropagationMode {
		return MetainfoPropagationSingleHop
	})
	test.Assert(t, getMetainfoPropagationMode(context.Background()) == MetainfoPropagationSingleHop,
		"provider should win over process default, effective=", getMetainfoPropagationMode(context.Background()))

	// A nil provider falls back to the process default mode.
	SetMetainfoPropagationMode(MetainfoPropagationSingleHop)
	var nilProvider func(context.Context) MetainfoPropagationMode
	metainfoPropagationModeProvider.Store(nilProvider)
	test.Assert(t, getMetainfoPropagationMode(context.Background()) == MetainfoPropagationSingleHop,
		"nil provider should fall back to process default, effective=", getMetainfoPropagationMode(context.Background()))
}
