package transmeta

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

func primaryMetadataMessage(t *testing.T, primary, response bool) remote.Message {
	t.Helper()
	cfg := rpcinfo.NewRPCConfig()
	mutable := rpcinfo.AsMutableRPCConfig(cfg)
	mutable.SetTransportProtocol(transport.TTHeaderFramed)
	mutable.SetPayloadCodec(serviceinfo.Thrift)
	mutable.SetRPCTimeout(3 * time.Second)
	mutable.SetConnectTimeout(time.Second)
	from := rpcinfo.NewEndpointInfo("initial-from", "initial-method", nil, nil)
	to := rpcinfo.NewEndpointInfo("destination", "Echo", nil, nil)
	invocation := rpcinfo.NewInvocation("IDL", "Echo")
	ri := rpcinfo.NewRPCInfo(from, to, invocation, cfg, rpcinfo.NewRPCStats())
	typ := remote.Call
	if response {
		typ = remote.Reply
	}
	msg := remote.NewMessage(nil, ri, typ, remote.Client)
	t.Cleanup(func() { remote.RecycleMessage(msg) })
	if primary && !remote.AccessHeaders(msg.TransInfo()).EnablePrimary(response) {
		t.Fatal("primary headers did not activate")
	}
	return msg
}

func primaryMetadataHeaders(info remote.TransInfo) (map[uint16]string, map[string]string) {
	ints, strings := make(map[uint16]string), make(map[string]string)
	h := remote.AccessHeaders(info)
	h.RangeInt(func(k uint16, v string) bool { ints[k] = v; return true })
	h.RangeString(func(k, v string) bool { strings[k] = v; return true })
	return ints, strings
}

func TestPrimaryClientTTHeaderMetadataParity(t *testing.T) {
	for _, locked := range []bool{false, true} {
		legacy := primaryMetadataMessage(t, false, false)
		primary := primaryMetadataMessage(t, true, false)
		for _, msg := range []remote.Message{legacy, primary} {
			h := remote.AccessHeaders(msg.TransInfo())
			h.SetInt(65535, "unknown")
			h.SetString("preserved", "value")
			if locked {
				cfg := rpcinfo.AsMutableRPCConfig(msg.RPCInfo().Config())
				cfg.LockConfig(rpcinfo.BitRPCTimeout)
				cfg.LockConfig(rpcinfo.BitConnectTimeout)
			}
			if _, err := ClientTTHeaderHandler.WriteMeta(context.Background(), msg); err != nil {
				t.Fatal(err)
			}
		}
		wantInt, wantStr := primaryMetadataHeaders(legacy.TransInfo())
		gotInt, gotStr := primaryMetadataHeaders(primary.TransInfo())
		if !reflect.DeepEqual(gotInt, wantInt) || !reflect.DeepEqual(gotStr, wantStr) {
			t.Fatalf("locked=%v primary=(%v,%v) legacy=(%v,%v)", locked, gotInt, gotStr, wantInt, wantStr)
		}
		if gotInt[transmeta.TransportType] != framedTransportType || gotStr[transmeta.HeaderIDLServiceName] != "IDL" {
			t.Fatalf("lost transport/IDL metadata: %v %v", gotInt, gotStr)
		}
		_, hasTimeout := gotInt[transmeta.RPCTimeout]
		if hasTimeout != locked || !remote.AccessHeaders(primary.TransInfo()).Primary() {
			t.Fatalf("timeout or primary state changed: %v", gotInt)
		}
	}
}

func TestPrimaryServerTTHeaderReadParity(t *testing.T) {
	for _, timeout := range []string{"97", "", "invalid"} {
		var results [2]struct {
			service, method string
			timeout         time.Duration
		}
		for i, primary := range []bool{false, true} {
			msg := primaryMetadataMessage(t, primary, false)
			h := remote.AccessHeaders(msg.TransInfo())
			h.MergeInt(map[uint16]string{transmeta.FromService: "caller", transmeta.FromMethod: "Call", transmeta.RPCTimeout: timeout})
			if _, err := ServerTTHeaderHandler.ReadMeta(context.Background(), msg); err != nil {
				t.Fatal(err)
			}
			results[i].service = msg.RPCInfo().From().ServiceName()
			results[i].method = msg.RPCInfo().From().Method()
			results[i].timeout = msg.RPCInfo().Config().RPCTimeout()
			if primary && !h.Primary() {
				t.Fatal("read materialized primary headers")
			}
		}
		if results[0] != results[1] || results[1].service != "caller" || results[1].method != "Call" {
			t.Fatalf("timeout=%q results=%+v", timeout, results)
		}
	}
}

func TestPrimaryResponseTTHeaderMetadataParity(t *testing.T) {
	legacy := primaryMetadataMessage(t, false, true)
	primary := primaryMetadataMessage(t, true, true)
	for _, msg := range []remote.Message{legacy, primary} {
		msg.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetBizStatusErr(kerrors.NewBizStatusErrorWithExtra(17, "denied", map[string]string{"reason": "policy"}))
		rpcinfo.AsTaggable(msg.RPCInfo().To()).SetTag(rpcinfo.ConnResetTag, "")
		if _, err := ServerTTHeaderHandler.WriteMeta(context.Background(), msg); err != nil {
			t.Fatal(err)
		}
		if _, err := ClientTTHeaderHandler.ReadMeta(context.Background(), msg); err != nil {
			t.Fatal(err)
		}
		if val, ok := remote.AccessHeaders(msg.TransInfo()).StringValue(transmeta.HeaderConnectionReadyToReset); !ok || val != "" {
			t.Fatal("empty reset header presence was lost")
		}
	}
	ints, strings := primaryMetadataHeaders(primary.TransInfo())
	wantInts, wantStrings := primaryMetadataHeaders(legacy.TransInfo())
	if !reflect.DeepEqual(ints, wantInts) || !reflect.DeepEqual(strings, wantStrings) || !remote.AccessHeaders(primary.TransInfo()).Primary() {
		t.Fatalf("response mismatch: %v %v / %v %v", ints, strings, wantInts, wantStrings)
	}
	for _, extra := range []string{`{"key":"value"}`, "{invalid", ""} {
		input := map[string]string{bizStatus: "17", bizMessage: "failure", bizExtra: extra}
		h := remote.AccessHeaders(primary.TransInfo())
		h.MergeString(input)
		want, wantErr := ParseBizStatusErr(input)
		got, gotErr := parseBizStatusErr(h.String)
		if (wantErr == nil) != (gotErr == nil) || !reflect.DeepEqual(got, want) {
			t.Fatalf("public map helper differs for extra=%q: %v/%v %v/%v", extra, got, gotErr, want, wantErr)
		}
	}
}

func TestPrimaryMetainfoForwardMergeParity(t *testing.T) {
	fields := map[string]string{
		metainfo.PrefixTransient + "same":             "wire-transient",
		metainfo.PrefixPersistent + "same":            "wire-persistent",
		metainfo.PrefixTransientUpstream + "upstream": "wire-upstream",
		metainfo.PrefixTransientUpstream:              "no-key",
		metainfo.PrefixTransient + "empty":            "",
		"ordinary":                                    "ignored",
		"":                                            "empty-key",
	}
	for _, existing := range []bool{false, true} {
		ctx := context.Background()
		if existing {
			ctx = metainfo.WithValue(ctx, "same", "old")
			ctx = metainfo.WithPersistentValue(ctx, "same", "old")
			ctx = metainfo.WithPersistentValue(ctx, "retained", "yes")
		}
		var contexts [2]context.Context
		for i, primary := range []bool{false, true} {
			msg := primaryMetadataMessage(t, primary, false)
			h := remote.AccessHeaders(msg.TransInfo())
			h.MergeString(fields)
			var err error
			contexts[i], err = MetainfoServerHandler.ReadMeta(ctx, msg)
			if err != nil || !metainfo.SendBackwardValue(contexts[i], "backward", "enabled") {
				t.Fatalf("server read: %v", err)
			}
			if primary && !h.Primary() {
				t.Fatal("metainfo read materialized headers")
			}
		}
		for hop := 0; hop < 3; hop++ {
			var saved [2]map[string]string
			for i := range contexts {
				saved[i] = make(map[string]string)
				metainfo.SaveMetaInfoToMap(contexts[i], saved[i])
				contexts[i] = metainfo.TransferForward(contexts[i])
			}
			if !reflect.DeepEqual(saved[0], saved[1]) {
				t.Fatalf("existing=%v hop=%d: %v", existing, hop, saved)
			}
		}
	}
}

func TestPrimaryMetainfoBackwardAllFieldsParity(t *testing.T) {
	fields := map[string]string{"ordinary": "visible", "": "empty-key", "empty-value": "", metainfo.PrefixTransient + "meta": "value"}
	for _, primary := range []bool{false, true} {
		msg := primaryMetadataMessage(t, primary, true)
		h := remote.AccessHeaders(msg.TransInfo())
		h.MergeString(fields)
		ctx := metainfo.WithBackwardValues(context.Background())
		metainfo.SetBackwardValue(ctx, "retained", "old")
		metainfo.SetBackwardValue(ctx, "ordinary", "overwrite")
		ctx, err := MetainfoClientHandler.ReadMeta(ctx, msg)
		if err != nil {
			t.Fatal(err)
		}
		got := metainfo.RecvAllBackwardValues(ctx)
		if got["retained"] != "old" || len(got) != len(fields)+1 {
			t.Fatalf("backward merge lost fields: %v", got)
		}
		for key, want := range fields {
			if value, ok := got[key]; !ok || value != want {
				t.Fatalf("backward key %q: %q/%v want %q", key, value, ok, want)
			}
		}
		if primary && !h.Primary() {
			t.Fatal("backward read materialized headers")
		}
	}
}

func TestPrimaryMetainfoWritesParity(t *testing.T) {
	t.Cleanup(func() { SetMetainfoPropagationMode(MetainfoPropagationLegacy) })
	for _, mode := range []MetainfoPropagationMode{MetainfoPropagationLegacy, MetainfoPropagationSingleHop} {
		SetMetainfoPropagationMode(mode)
		ctx := metainfo.SetMetaInfoFromMap(context.Background(), map[string]string{metainfo.PrefixTransientUpstream + "upstream": "up"})
		ctx = metainfo.WithValue(ctx, "transient", "transient-value")
		ctx = metainfo.WithPersistentValue(ctx, "persistent", "persistent-value")
		ctx = metainfo.WithBackwardValuesToSend(ctx)
		metainfo.SendBackwardValuesFromMap(ctx, map[string]string{"ordinary": "reply", "": "empty-key", "empty-value": ""})
		for _, handler := range []remote.MetaHandler{MetainfoClientHandler, MetainfoServerHandler} {
			var values [2]map[string]string
			for i, primary := range []bool{false, true} {
				msg := primaryMetadataMessage(t, primary, false)
				h := remote.AccessHeaders(msg.TransInfo())
				h.SetString("preserved", "header")
				if _, err := handler.WriteMeta(ctx, msg); err != nil {
					t.Fatal(err)
				}
				_, values[i] = primaryMetadataHeaders(msg.TransInfo())
				if primary && !h.Primary() {
					t.Fatal("metainfo write materialized headers")
				}
			}
			if !reflect.DeepEqual(values[0], values[1]) {
				t.Fatalf("mode=%v handler=%T: %v", mode, handler, values)
			}
		}
	}
}

func TestPrimaryLegacyEmptyMapAlias(t *testing.T) {
	msg := primaryMetadataMessage(t, false, true)
	ints, strings := make(map[uint16]string), make(map[string]string)
	msg.TransInfo().PutTransIntInfo(ints)
	msg.TransInfo().PutTransStrInfo(strings)
	h := remote.AccessHeaders(msg.TransInfo())
	if h.EnablePrimary(true) {
		t.Fatal("escaped empty maps were reclaimed during a message")
	}
	msg.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetBizStatusErr(kerrors.NewBizStatusError(17, "denied"))
	if _, err := ServerTTHeaderHandler.WriteMeta(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if ints[transmeta.MsgType] == "" || strings[bizStatus] != "17" || strings[bizMessage] != "denied" {
		t.Fatalf("direct writes detached old empty maps: %v %v", ints, strings)
	}
	msg.TransInfo().PutTransStrInfo(map[string]string{"merged": "value"})
	if strings["merged"] != "value" {
		t.Fatal("Put detached a nonempty map")
	}

	client := primaryMetadataMessage(t, false, false)
	emptyInt, emptyString := make(map[uint16]string), make(map[string]string)
	client.TransInfo().PutTransIntInfo(emptyInt)
	client.TransInfo().PutTransStrInfo(emptyString)
	if _, err := ClientTTHeaderHandler.WriteMeta(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if len(emptyInt) != 0 || emptyString[transmeta.HeaderIDLServiceName] != "IDL" {
		t.Fatalf("legacy client Put/direct-write distinction changed: %v %v", emptyInt, emptyString)
	}
	replacement := map[string]string{"replacement": "first"}
	other := primaryMetadataMessage(t, false, false)
	other.TransInfo().PutTransStrInfo(map[string]string{})
	other.TransInfo().PutTransStrInfo(replacement)
	replacement["replacement"] = "second"
	if remote.AccessHeaders(other.TransInfo()).String("replacement") != "second" {
		t.Fatal("Put failed to preserve replacement map alias")
	}
}

type primaryMetadataPutProbe struct {
	remote.TransInfo
	intPuts, strPuts int
}

func (p *primaryMetadataPutProbe) PutTransIntInfo(values map[uint16]string) {
	p.intPuts++
	p.TransInfo.PutTransIntInfo(values)
}

func (p *primaryMetadataPutProbe) PutTransStrInfo(values map[string]string) {
	p.strPuts++
	p.TransInfo.PutTransStrInfo(values)
}

type primaryMetadataCustomMessage struct {
	remote.Message
	info remote.TransInfo
}

func (m primaryMetadataCustomMessage) TransInfo() remote.TransInfo { return m.info }

func TestPrimaryLegacyCustomTransInfoDirectWrites(t *testing.T) {
	msg := primaryMetadataMessage(t, false, true)
	ints, strings := msg.TransInfo().TransIntInfo(), msg.TransInfo().TransStrInfo()
	probe := &primaryMetadataPutProbe{TransInfo: msg.TransInfo()}
	custom := primaryMetadataCustomMessage{Message: msg, info: probe}
	msg.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetBizStatusErr(kerrors.NewBizStatusError(17, "denied"))
	if _, err := ServerTTHeaderHandler.WriteMeta(context.Background(), custom); err != nil {
		t.Fatal(err)
	}
	if probe.intPuts != 0 || probe.strPuts != 0 {
		t.Fatalf("legacy direct writes invoked custom Put: int=%d str=%d", probe.intPuts, probe.strPuts)
	}
	if ints[transmeta.MsgType] == "" || strings[bizStatus] != "17" || strings[bizMessage] != "denied" {
		t.Fatalf("legacy custom maps lost writes: %v %v", ints, strings)
	}
}
