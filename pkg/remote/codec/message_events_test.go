package codec

import (
	"context"
	"encoding/binary"
	"errors"
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
				call := new(wireEvents)
				ctx = context.WithValue(ctx, remote.TransportContextKey{}, call)
				codec := &defaultCodec{}
				if err := codec.PrepareMessage(ctx, send); err != nil {
					t.Fatal(err)
				}
				want := remote.WireInfo{WithTTHeader: true, Framed: tp&transport.Framed != 0, SeqID: 33, ProtocolID: uint16(ProtocolIDDefault), Flags: uint16(HeaderFlagDuplexReverse), MessageType: uint32(send.MessageType())}
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
				prefix := 14 + int(binary.BigEndian.Uint16(wire[12:14]))*4
				if tp&transport.Framed != 0 {
					prefix += 4
				}
				if wantLen := len(wire) - prefix; recv.PayloadLen() != wantLen {
					t.Fatalf("payload length recv=%d, want=%d", recv.PayloadLen(), wantLen)
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

func TestUnaryWireMetadataFramed(t *testing.T) {
	send := initClientSendMsg(transport.Framed, 16<<10)
	defer remote.RecycleMessage(send)
	send.SetPayloadCodec(mpc)
	send.RPCInfo().Invocation().(rpcinfo.InvocationSetter).SetSeqID(33)
	call := new(wireEvents)
	ctx := context.WithValue(context.Background(), remote.TransportContextKey{}, call)
	out := remote.NewWriterBuffer(32 << 10)
	defer out.Release(nil)
	codec := &defaultCodec{}
	if err := codec.Encode(ctx, send, out); err != nil {
		t.Fatal(err)
	}
	raw, err := out.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 8 || raw[4] != 0x80 || raw[5] != 1 || call.RequestWire.WithTTHeader || !call.RequestWire.Framed {
		t.Fatal("did not encode Framed", call.RequestWire)
	}
	ctx, recv := initServerRecvMsgWithMockMsg(ctx)
	defer remote.RecycleMessage(recv)
	recv.SetPayloadCodec(mpc)
	in := remote.NewReaderBuffer(raw)
	defer in.Release(nil)
	if err = codec.DecodeMeta(ctx, recv, in); err != nil {
		t.Fatal(err)
	}
	if call.RequestWire.WithTTHeader || !call.RequestWire.Framed {
		t.Fatal("actual Framed wire misidentified")
	}
	if err = codec.DecodePayload(ctx, recv, &unaryBodyReader{ByteBuffer: in}); err != nil {
		t.Fatal(err)
	}
	if call.RequestWire.SeqID != 33 || call.RequestWire.MessageType != uint32(remote.Call) {
		t.Fatal("missing Framed envelope facts", call.RequestWire)
	}
}

type wireEvents struct{ RequestWire, ResponseWire remote.WireInfo }

func (h *wireEvents) OnMessageEvent(_ context.Context, msg remote.Message, event remote.MessageEvent, wire remote.WireInfo) error {
	w := &h.ResponseWire
	if (event == remote.MessageEncode && msg.RPCRole() == remote.Client) || (event != remote.MessageEncode && msg.RPCRole() == remote.Server) {
		w = &h.RequestWire
	}
	switch event {
	case remote.MessageEncode, remote.MessageDecodeHeader:
		*w = wire
	case remote.MessageDecodeMeta:
		w.WithTTHeader, w.Framed, w.MessageType = wire.WithTTHeader, wire.Framed, wire.MessageType
		return UpdateMsgType(wire.MessageType, msg)
	case remote.MessageDecodePayloadDone:
		w.MessageType = uint32(msg.MessageType())
		if !w.WithTTHeader {
			w.SeqID = msg.RPCInfo().Invocation().SeqID()
		}
	}
	return nil
}

type failingPayload struct {
	remote.PayloadCodec
	err error
}

func (p failingPayload) Unmarshal(context.Context, remote.Message, remote.ByteBuffer) error {
	return p.err
}

type completionEvents struct {
	err  error
	done bool
}

func (h *completionEvents) OnMessageEvent(_ context.Context, _ remote.Message, event remote.MessageEvent, _ remote.WireInfo) error {
	if event == remote.MessageDecodePayloadDone {
		h.done = true
		return h.err
	}
	return nil
}

func TestPayloadCompletionPreservesDecoderError(t *testing.T) {
	decoderErr := errors.New("decoder failed")
	completionErr := errors.New("completion failed")
	for _, original := range []error{nil, decoderErr} {
		msg := initClientRecvMsg()
		msg.SetPayloadCodec(failingPayload{PayloadCodec: mpc, err: original})
		h := &completionEvents{err: completionErr}
		ctx := context.WithValue(context.Background(), remote.TransportContextKey{}, h)
		in := remote.NewReaderBuffer(nil)
		err := (&defaultCodec{}).DecodePayload(ctx, msg, in)
		in.Release(nil)
		remote.RecycleMessage(msg)
		want := original
		if want == nil {
			want = completionErr
		}
		if !h.done || !errors.Is(err, want) {
			t.Fatalf("completion=%v error=%v, want %v", h.done, err, want)
		}
	}
}

type skippedPayload struct{ remote.PayloadCodec }

func (skippedPayload) Unmarshal(_ context.Context, _ remote.Message, in remote.ByteBuffer) error {
	return in.Skip(3)
}

type countedReader struct{ remote.ByteBuffer }

// Some readers preserve a legacy ReadLen contract while separately exposing
// the full wire consumption, including Skip.
func (*countedReader) ReadLen() int       { return 0 }
func (r *countedReader) ConsumedLen() int { return r.ByteBuffer.ReadLen() }

func TestPayloadAccountingDoesNotDependOnObserver(t *testing.T) {
	for _, observed := range []bool{false, true} {
		msg := initClientRecvMsg()
		msg.SetPayloadCodec(skippedPayload{PayloadCodec: mpc})
		msg.SetPayloadLen(0)
		ctx := context.Background()
		if observed {
			ctx = context.WithValue(ctx, remote.TransportContextKey{}, &completionEvents{})
		}
		in := &countedReader{ByteBuffer: remote.NewReaderBuffer([]byte{1, 2, 3})}
		err := (&defaultCodec{}).DecodePayload(ctx, msg, in)
		if err != nil || msg.PayloadLen() != 3 || msg.RPCInfo().Stats().RecvSize() != 3 {
			t.Fatalf("observer=%v payload=%d received=%d err=%v", observed, msg.PayloadLen(), msg.RPCInfo().Stats().RecvSize(), err)
		}
		in.Release(nil)
		remote.RecycleMessage(msg)
	}
}
