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

func TestHeaderCodecRoundTrip(t *testing.T) {
	for _, tb := range transportBuffers {
		for _, response := range []bool{false, true} {
			name := tb.Name + "/request"
			if response {
				name = tb.Name + "/response"
			}
			t.Run(name, func(t *testing.T) {
				ctx := context.Background()
				send := initClientSendMsg(transport.TTHeader)
				if response {
					remote.RecycleMessage(send)
					send = initServerSendMsg(transport.TTHeader)
				}
				defer remote.RecycleMessage(send)
				send.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetSeqID(42)

				headers := send.TransInfo()
				headers.TransIntInfo()[transmeta.FromService] = "source-service"
				headers.TransIntInfo()[60000] = "unknown-int"
				headers.TransStrInfo()[transmeta.HeaderIDLServiceName] = "idl-service"
				headers.TransStrInfo()[transmeta.HeaderTransRemoteAddr] = "127.0.0.1:9299"
				headers.TransStrInfo()["unknown-string"] = "kept"
				buf := tb.NewBuffer()
				length, err := ttHeaderCodec.encode(ctx, send, buf)
				if err != nil {
					t.Fatal(err)
				}
				binary.BigEndian.PutUint32(length, uint32(buf.WrittenLen()-Size32+mockPayloadLen))
				if err = buf.Flush(); err != nil {
					t.Fatal(err)
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
				decoded := recv.TransInfo()
				if decoded.TransIntInfo()[60000] != "unknown-int" || decoded.TransStrInfo()["unknown-string"] != "kept" {
					t.Fatal("decode or fillBasicInfo lost unknown headers")
				}

				if response {
					if addr := recv.RPCInfo().To().Address(); addr == nil || addr.String() != "127.0.0.1:9299" {
						t.Fatalf("response address=%v", addr)
					}
				} else if recv.RPCInfo().From().ServiceName() != "source-service" || recv.RPCInfo().Invocation().ServiceName() != "idl-service" {
					t.Fatal("request derived RPCInfo was not filled")
				}
				public := recv.TransInfo().TransStrInfo()
				if public["unknown-string"] != "kept" {
					t.Fatal("public map lost decoded values")
				}
				public["unknown-string"] = "direct-write"
				if decoded.TransStrInfo()["unknown-string"] != "direct-write" {
					t.Fatal("public map was detached")
				}
			})
		}
	}
}
