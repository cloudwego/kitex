package codec

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/remote/transmeta"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/rpcinfo/remoteinfo"
	"github.com/cloudwego/kitex/pkg/serviceinfo"
	"github.com/cloudwego/kitex/transport"
)

type primaryCodecLifecycle struct {
	remote.UnaryLifecycle
	enabled bool
}

func (h primaryCodecLifecycle) PrimaryUnaryHeaders() bool { return h.enabled }

func TestPrimaryHeaderCodecRoundTrip(t *testing.T) {
	for _, tb := range transportBuffers {
		for _, response := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				name := tb.Name + "/request"
				if response {
					name = tb.Name + "/response"
				}
				if enabled {
					name += "/primary"
				} else {
					name += "/legacy"
				}
				t.Run(name, func(t *testing.T) {
					call := remote.NewUnaryCall(primaryCodecLifecycle{enabled: enabled})
					defer call.Recycle()
					ctx := remote.WithUnaryCall(context.Background(), call)
					send := initClientSendMsg(transport.TTHeader)
					if response {
						remote.RecycleMessage(send)
						send = initServerSendMsg(transport.TTHeader)
					}
					defer remote.RecycleMessage(send)
					send.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetSeqID(42)
					if got := remote.PreparePrimaryHeaders(ctx, send, response); got != enabled {
						t.Fatalf("send activation=%v, want %v", got, enabled)
					}
					headers := remote.AccessHeaders(send.TransInfo())
					headers.SetInt(transmeta.FromService, "source-service")
					headers.SetInt(60000, "unknown-int")
					headers.SetString(transmeta.HeaderIDLServiceName, "idl-service")
					headers.SetString(transmeta.HeaderTransRemoteAddr, "127.0.0.1:9299")
					headers.SetString("unknown-string", "kept")
					buf := tb.NewBuffer()
					length, err := ttHeaderCodec.encode(ctx, send, buf)
					if err != nil {
						t.Fatal(err)
					}
					binary.BigEndian.PutUint32(length, uint32(buf.WrittenLen()-Size32+mockPayloadLen))
					if err = buf.Flush(); err != nil {
						t.Fatal(err)
					}
					if headers.Primary() != enabled {
						t.Fatal("encode changed primary state")
					}
					_, recv := initServerRecvMsg(ctx)
					if response {
						remote.RecycleMessage(recv)
						to := remoteinfo.NewRemoteInfo(&rpcinfo.EndpointBasicInfo{ServiceName: mockServiceName}, mockMethod)
						to.SetInstance(&mockInst{})
						ri := rpcinfo.NewRPCInfo(rpcinfo.EmptyEndpointInfo(), to.ImmutableView(), rpcinfo.NewInvocation("", mockMethod), rpcinfo.NewRPCConfig(), rpcinfo.NewRPCStats())
						recv = remote.NewMessage(nil, ri, remote.Reply, remote.Client)
					}
					defer remote.RecycleMessage(recv)
					recv.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetSeqID(42)
					rpcinfo.AsMutableRPCConfig(recv.RPCInfo().Config()).SetPayloadCodec(serviceinfo.Thrift)
					if err = ttHeaderCodec.decode(ctx, recv, buf); err != nil {
						t.Fatal(err)
					}
					decoded := remote.AccessHeaders(recv.TransInfo())
					if decoded.Primary() != enabled || decoded.Int(60000) != "unknown-int" || decoded.String("unknown-string") != "kept" {
						t.Fatal("decode or fillBasicInfo lost primary/unknown headers")
					}
					if enabled && (!decoded.View().Valid() || decoded.View().Len() != 5) {
						t.Fatal("decoded view missing entries")
					}
					if response {
						if addr := recv.RPCInfo().To().Address(); addr == nil || addr.String() != "127.0.0.1:9299" {
							t.Fatalf("response address=%v", addr)
						}
					} else if recv.RPCInfo().From().ServiceName() != "source-service" || recv.RPCInfo().Invocation().ServiceName() != "idl-service" {
						t.Fatal("request derived RPCInfo was not filled")
					}
					public := recv.TransInfo().TransStrInfo()
					if decoded.Primary() || public["unknown-string"] != "kept" {
						t.Fatal("public map did not opt out with existing values")
					}
					public["unknown-string"] = "direct-write"
					if decoded.String("unknown-string") != "direct-write" || remote.PreparePrimaryHeaders(ctx, recv, response) {
						t.Fatal("escaped map was detached or primary was re-enabled")
					}
				})
			}
		}
	}
}
