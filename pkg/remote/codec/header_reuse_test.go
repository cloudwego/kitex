package codec

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/cloudwego/gopkg/protocol/ttheader"
	"github.com/cloudwego/kitex/pkg/remote"
)

func reuseHeaderWire(t *testing.T) []byte {
	t.Helper()
	out := remote.NewWriterBuffer(128)
	defer out.Release(nil)
	length, err := ttheader.Encode(context.Background(), ttheader.EncodeParam{
		IntInfo: map[uint16]string{60000: "iv"},
		StrInfo: map[string]string{"sk": "sv"},
	}, out)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := out.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(length, uint32(len(wire)-4+mockPayloadLen))
	return append([]byte(nil), wire...)
}

func TestTTHeaderReuseAndMerge(t *testing.T) {
	for _, populated := range []bool{false, true} {
		_, msg := initServerRecvMsg(context.Background())
		ints, strs := msg.TransInfo().TransIntInfo(), msg.TransInfo().TransStrInfo()
		if populated {
			ints[50000], ints[60000] = "keep", "old"
			strs["keep"], strs["sk"] = "keep", "old"
		}
		in := remote.NewReaderBuffer(reuseHeaderWire(t))
		err := ttHeaderCodec.decode(context.Background(), msg, in)
		in.Release(nil)
		if err != nil {
			t.Fatal(err)
		}
		if ints[60000] != "iv" || strs["sk"] != "sv" {
			t.Fatal("decode replaced the existing map or lost decoded values")
		}
		if populated && (ints[50000] != "keep" || strs["keep"] != "keep") {
			t.Fatal("decode lost preexisting headers")
		}
		remote.RecycleMessage(msg)
	}
}

type rejectingHeaderEvent struct{ err error }

func (h rejectingHeaderEvent) OnMessageEvent(context.Context, remote.Message, remote.MessageEvent, remote.WireInfo) error {
	return h.err
}

func TestTTHeaderReuseRollback(t *testing.T) {
	for _, rejectEvent := range []bool{false, true} {
		ctx := context.Background()
		wire := reuseHeaderWire(t)
		if rejectEvent {
			ctx = context.WithValue(ctx, remote.TransportContextKey{}, rejectingHeaderEvent{errors.New("reject header")})
		} else {
			// The two valid fields occupy 22 bytes plus two padding bytes.
			// An invalid final info ID fails after both maps were populated.
			wire[len(wire)-1] = 255
		}
		_, msg := initServerRecvMsg(ctx)
		ints, strs := msg.TransInfo().TransIntInfo(), msg.TransInfo().TransStrInfo()
		in := remote.NewReaderBuffer(wire)
		err := ttHeaderCodec.decode(ctx, msg, in)
		in.Release(nil)
		if err == nil || len(ints) != 0 || len(strs) != 0 {
			t.Fatalf("failed decode retained headers: err=%v ints=%v strs=%v", err, ints, strs)
		}
		remote.RecycleMessage(msg)
	}
}

type nilHeaderMaps struct{ remote.TransInfo }

func (nilHeaderMaps) TransIntInfo() map[uint16]string { return nil }
func (nilHeaderMaps) TransStrInfo() map[string]string { return nil }

type nilHeaderMessage struct {
	remote.Message
	info remote.TransInfo
}

func (m nilHeaderMessage) TransInfo() remote.TransInfo { return m.info }

func TestTTHeaderReuseNilMaps(t *testing.T) {
	_, msg := initServerRecvMsg(context.Background())
	defer remote.RecycleMessage(msg)
	info := msg.TransInfo()
	wrapped := nilHeaderMessage{Message: msg, info: nilHeaderMaps{info}}
	in := remote.NewReaderBuffer(reuseHeaderWire(t))
	defer in.Release(nil)
	if err := ttHeaderCodec.decode(context.Background(), wrapped, in); err != nil {
		t.Fatal(err)
	}
	if info.TransIntInfo()[60000] != "iv" || info.TransStrInfo()["sk"] != "sv" {
		t.Fatal("nil map fallback lost decoded fields")
	}
}
