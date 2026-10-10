package codec

import (
	"context"
	"encoding/binary"
	"reflect"
	"strconv"
	"testing"

	"github.com/cloudwego/gopkg/protocol/ttheader"

	"github.com/cloudwego/kitex/pkg/remote"
)

func pooledHeaderFrame(t *testing.T) []byte {
	t.Helper()
	frame, err := ttheader.EncodeToBytes(context.Background(), ttheader.EncodeParam{
		ProtocolID: ttheader.ProtocolIDThriftBinary,
		IntInfo:    map[uint16]string{3: "caller", 9: "method"},
		StrInfo:    map[string]string{"header": "value", ttheader.GDPRToken: "token"},
	})
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(frame[:4], uint32(len(frame)-4))
	return frame
}

func pooledHeaderDecode(ctx context.Context, msg remote.Message, frame []byte) error {
	in := remote.NewReaderBuffer(frame)
	err := ttHeaderCodec.decode(ctx, msg, in)
	_ = in.Release(err)
	return err
}

func TestTTHeaderDecodeRetainsPooledMaps(t *testing.T) {
	frame := pooledHeaderFrame(t)
	ctx, msg := initServerRecvMsg(context.Background())
	defer remote.RecycleMessage(msg)
	ints, strs := msg.TransInfo().TransIntInfo(), msg.TransInfo().TransStrInfo()
	for round := 0; round < 3; round++ {
		if err := pooledHeaderDecode(ctx, msg, frame); err != nil {
			t.Fatal(err)
		}
		if ints[3] != "caller" || ints[9] != "method" || strs["header"] != "value" || strs[ttheader.GDPRToken] != "token" {
			t.Fatalf("round %d: decode replaced the pooled maps", round)
		}
		for i := 0; i < 64; i++ {
			ints[uint16(1000+i)] = "mesh-value"
			strs["mesh-"+strconv.Itoa(i)] = "mesh-value"
		}
		if msg.TransInfo().TransIntInfo()[1000] != "mesh-value" || msg.TransInfo().TransStrInfo()["mesh-0"] != "mesh-value" {
			t.Fatal("decoded maps no longer share pooled storage")
		}
		for key := range ints {
			delete(ints, key)
		}
		for key := range strs {
			delete(strs, key)
		}
	}
}

func TestTTHeaderDecodePooledMapErrorRecovery(t *testing.T) {
	valid := pooledHeaderFrame(t)
	unknown := append(append([]byte(nil), valid...), 255, 0, 0, 0)
	binary.BigEndian.PutUint32(unknown[:4], uint32(len(unknown)-4))
	binary.BigEndian.PutUint16(unknown[12:14], uint16((len(unknown)-ttheader.TTHeaderMetaSize)/4))
	for _, nonempty := range []bool{false, true} {
		ctx, msg := initServerRecvMsg(context.Background())
		ints, strs := msg.TransInfo().TransIntInfo(), msg.TransInfo().TransStrInfo()
		if nonempty {
			ints[3], ints[60000] = "original", "keep-int"
			strs["header"], strs["keep"] = "original", "keep-string"
		}
		wantInts, wantStrs := make(map[uint16]string), make(map[string]string)
		for k, v := range ints {
			wantInts[k] = v
		}
		for k, v := range strs {
			wantStrs[k] = v
		}
		for _, malformed := range [][]byte{valid[:13], valid[:len(valid)-1], unknown} {
			if err := pooledHeaderDecode(ctx, msg, malformed); err == nil {
				t.Fatal("malformed header succeeded")
			}
			if !reflect.DeepEqual(msg.TransInfo().TransIntInfo(), wantInts) || !reflect.DeepEqual(msg.TransInfo().TransStrInfo(), wantStrs) {
				t.Fatal("failed decode published partial headers")
			}
		}
		if err := pooledHeaderDecode(ctx, msg, valid); err != nil {
			t.Fatal(err)
		}
		if ints[3] != "caller" || strs["header"] != "value" || strs[ttheader.GDPRToken] != "token" {
			t.Fatal("successful decode after failure lost map contents")
		}
		if nonempty && (ints[60000] != "keep-int" || strs["keep"] != "keep-string") {
			t.Fatal("successful decode discarded existing unique keys")
		}
		remote.RecycleMessage(msg)
	}
}
