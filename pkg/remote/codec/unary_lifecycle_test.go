package codec

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/cloudwego/kitex/pkg/remote"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/transport"
)

func TestUnaryWireMetadata(t *testing.T) {
	for _, tp := range []transport.Protocol{transport.TTHeader, transport.TTHeaderFramed} {
		for _, role := range []remote.RPCRole{remote.Client, remote.Server} {
			t.Run(fmt.Sprintf("%s/%d", tp, role), func(t *testing.T) {
				ctx := context.Background()
				send := initClientSendMsg(tp, 16<<10)
				if role == remote.Server {
					remote.RecycleMessage(send)
					send = initServerSendMsg(tp, 16<<10)
				}
				defer remote.RecycleMessage(send)
				send.SetPayloadCodec(mpc)
				send.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetSeqID(33)
				send.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetMethodName(mockMethod)
				send.Tags()[HeaderFlagsKey] = HeaderFlagDuplexReverse
				call := remote.NewUnaryCall(nil)
				defer call.Recycle()
				ctx = remote.WithUnaryCall(ctx, call)
				codec := &defaultCodec{}
				if err := codec.PrepareUnaryWire(ctx, send, call); err != nil {
					t.Fatal(err)
				}
				want := remote.UnaryWire{Framed: tp&transport.Framed != 0, SeqID: 33, ProtocolID: uint16(ProtocolIDDefault), Flags: uint16(HeaderFlagDuplexReverse), MessageType: uint32(send.MessageType())}
				got := call.RequestWire
				if role == remote.Server {
					got = call.ResponseWire
				}
				if got != want {
					t.Fatalf("prepared wire = %+v, want %+v", got, want)
				}
				out := remote.NewWriterBuffer(32 << 10)
				defer out.Release(nil)
				if err := codec.Encode(ctx, send, out); err != nil {
					t.Fatal(err)
				}
				wire, err := out.Bytes()
				if err != nil {
					t.Fatal(err)
				}
				binary.BigEndian.PutUint32(wire[8:12], 999)
				wire[14] = byte(ProtocolIDKitexProtobuf)
				ctx, recv := initServerRecvMsgWithMockMsg(ctx)
				if role == remote.Server {
					remote.RecycleMessage(recv)
					recv = initClientRecvMsg()
					recv.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetSeqID(33)
				}
				defer remote.RecycleMessage(recv)
				recv.SetPayloadCodec(mpc)
				in := remote.NewReaderBuffer(wire)
				defer in.Release(nil)
				if err := codec.DecodeMeta(ctx, recv, in); err != nil {
					t.Fatal(err)
				}
				if send.PayloadLen() != recv.PayloadLen() {
					t.Fatalf("payload length send=%d recv=%d", send.PayloadLen(), recv.PayloadLen())
				}
				if err := codec.DecodePayload(ctx, recv, &unaryBodyReader{ByteBuffer: in}); err != nil {
					t.Fatal(err)
				}
				want.SeqID, want.ProtocolID = 999, uint16(ProtocolIDKitexProtobuf)
				got = call.RequestWire
				if role == remote.Server {
					got = call.ResponseWire
				}
				if got != want || recv.RPCInfo().Invocation().SeqID() != 33 {
					t.Fatalf("decoded wire = %+v, want %+v; payload seqid = %d", got, want, recv.RPCInfo().Invocation().SeqID())
				}
			})
		}
	}
}

type unaryBodyReader struct {
	remote.ByteBuffer
}

func (b *unaryBodyReader) Peek(n int) ([]byte, error) {
	if n > 8 {
		return nil, fmt.Errorf("unary body peek: %d bytes", n)
	}
	return b.ByteBuffer.Peek(n)
}
