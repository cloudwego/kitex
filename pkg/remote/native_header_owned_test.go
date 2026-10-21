package remote

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/cloudwego/gopkg/protocol/ttheader"
)

func TestNativePrimaryHeaderDecodeCoalescesStringStorage(t *testing.T) {
	param := ttheader.EncodeParam{IntInfo: make(map[uint16]string), StrInfo: make(map[string]string)}
	for i := 0; i < 16; i++ {
		param.IntInfo[uint16(i)] = fmt.Sprintf("integer-value-%d", i)
	}
	for i := 0; i < 8; i++ {
		param.StrInfo[fmt.Sprintf("string-key-%d", i)] = fmt.Sprintf("string-value-%d", i)
	}
	param.StrInfo[ttheader.GDPRToken] = "acl-token"
	wire, err := ttheader.EncodeToBytes(context.Background(), param)
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(wire, uint32(len(wire)-4))
	ti := newTransInfo().(*transInfo)
	defer ti.Recycle()
	headers := AccessHeaders(ti)
	if !headers.EnablePrimary(false) {
		t.Fatal("primary metadata unavailable")
	}
	reader := &nativeByteBuffer{data: wire}
	allocs := testing.AllocsPerRun(500, func() {
		ti.store.reset(false)
		reader.read = 0
		if _, err := headers.DecodeTTHeader(context.Background(), reader); err != nil {
			t.Fatal(err)
		}
	})
	for key, want := range param.IntInfo {
		if got := headers.Int(key); got != want {
			t.Fatalf("int %d: %q != %q", key, got, want)
		}
	}
	for key, want := range param.StrInfo {
		if got := headers.String(key); got != want {
			t.Fatalf("string %q: %q != %q", key, got, want)
		}
	}
	if allocs > 2 {
		t.Fatalf("native header strings allocated separately: %.2f allocations", allocs)
	}
	t.Logf("native header decode allocations: %.2f", allocs)
}
