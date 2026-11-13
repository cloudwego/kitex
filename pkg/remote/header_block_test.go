package remote

import (
	"bytes"
	"context"
	"encoding/binary"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/ttheader"
)

type nativeBlockEntry struct {
	Kind, ID   uint16
	Op         uint32
	Key, Value string
}

func nativeBlockSpan(data, span []byte) string {
	offset, size := binary.LittleEndian.Uint32(span), binary.LittleEndian.Uint32(span[4:])
	return string(data[offset : offset+size])
}

func nativeBlockEntries(data []byte) []nativeBlockEntry {
	count := int(binary.LittleEndian.Uint32(data[12:]))
	entries := make([]nativeBlockEntry, count)
	for i := range entries {
		p := data[len(data)-(i+1)*headerBlockEntrySize:]
		entries[i] = nativeBlockEntry{binary.LittleEndian.Uint16(p), binary.LittleEndian.Uint16(p[2:]), binary.LittleEndian.Uint32(p[4:]), nativeBlockSpan(data, p[8:]), nativeBlockSpan(data, p[16:])}
	}
	return entries
}

func nativeBlockSlot(data []byte, slot int) int {
	return int(binary.LittleEndian.Uint16(data[headerBlockSlots+2*slot:]))
}

func nativeBlockMetadata(h HeaderAccess, field int) string {
	data, _ := h.NativeBlock()
	return nativeBlockSpan(data, data[headerBlockMetadata+8*field:])
}

func TestNativeBlockMirrorsWritesAsAppendOnlyLog(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	if data, pinned := h.NativeBlock(); data != nil || pinned {
		t.Fatal("plain headers exposed a block")
	}
	h.EnablePrimary(false)
	h.SetInt(3, "caller")
	h.SetInt(3, "caller")
	h.SetString("key", "value")
	h.SetInt(7, "cluster")
	h.SetInt(7, "other")
	h.DeleteString("key")
	h.DeleteString("missing")
	h.SetString("RPC_TRANSIT_secondary-acl-token", "secondary")
	h.SetInt(21, "")
	data, _ := h.NativeBlock()
	if len(data) != headerBlockInitialSize {
		t.Fatalf("block storage: size=%d", len(data))
	}
	if binary.LittleEndian.Uint32(data) != headerBlockVersion || int(binary.LittleEndian.Uint32(data[4:])) != len(data) {
		t.Fatal("block head")
	}
	want := []nativeBlockEntry{{3, 3, 0, "", "caller"}, {2, 0, 0, "key", "value"}, {3, 7, 0, "", "cluster"}, {3, 7, 0, "", "other"}, {2, 0, 1, "key", ""}, {2, 0, 0, "RPC_TRANSIT_secondary-acl-token", "secondary"}, {3, 21, 0, "", ""}}
	got := nativeBlockEntries(data)
	if len(got) != len(want) {
		t.Fatalf("entries: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d: %+v", i, got[i])
		}
	}
	end := headerBlockHeadSize
	for _, e := range want {
		end += len(e.Key) + len(e.Value)
	}
	if int(binary.LittleEndian.Uint32(data[8:])) != end {
		t.Fatal("data end")
	}
	for field, value := range map[int]string{0: "caller", 3: "other", 16: "secondary", 17: "", 1: "", 15: ""} {
		if nativeBlockMetadata(h, field) != value {
			t.Fatalf("metadata %d: %q", field, nativeBlockMetadata(h, field))
		}
	}
	h.DeleteInt(3)
	if data, _ = h.NativeBlock(); nativeBlockMetadata(h, 0) != "" || h.View().Len() != 5 || len(nativeBlockEntries(data)) != 8 {
		t.Fatal("delete did not clear metadata or append a tombstone")
	}
	for slot, entry := range map[int]int{3: 8, 7: 4, 21: 7, headerBlockIntSlots + 3: 6, 2: 0, 31: 0, headerBlockIntSlots: 0, headerBlockSlotCount - 1: 0} {
		if nativeBlockSlot(data, slot) != entry {
			t.Fatalf("slot %d: %d", slot, nativeBlockSlot(data, slot))
		}
	}
	if !h.SetNativeMetadata(0, "fallback") || !h.SetNativeMetadata(18, "10.0.0.1") || h.SetNativeMetadata(19, "x") || h.SetNativeMetadata(-1, "x") {
		t.Fatal("native metadata bounds")
	}
	if nativeBlockMetadata(h, 0) != "fallback" || nativeBlockMetadata(h, 18) != "10.0.0.1" || len(nativeBlockEntries(data)) != 8 {
		t.Fatal("native metadata was not stored outside entries")
	}
	if got := h.View().Len(); got != 5 {
		t.Fatalf("native metadata changed Go entries: %d", got)
	}
}

func TestNativeBlockResponseKindsAndReuse(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	h.EnablePrimary(true)
	h.SetInt(3, "caller")
	h.SetString("sc", "1606")
	data, _ := h.NativeBlock()
	got := nativeBlockEntries(data)
	if len(got) != 2 || got[0] != (nativeBlockEntry{10, 3, 0, "", "caller"}) || got[1] != (nativeBlockEntry{4, 0, 0, "sc", "1606"}) || nativeBlockMetadata(h, 0) != "" {
		t.Fatalf("response block: %+v", got)
	}
	first := &data[0]
	ti.zero()
	h.EnablePrimary(false)
	if data, _ = h.NativeBlock(); &data[0] != first || len(nativeBlockEntries(data)) != 0 || binary.LittleEndian.Uint32(data[8:]) != headerBlockHeadSize {
		t.Fatal("block was not reused empty")
	}
	for field := 0; field < headerBlockMetadataSize; field++ {
		if nativeBlockMetadata(h, field) != "" {
			t.Fatalf("stale metadata %d", field)
		}
	}
	h.SetInt(6, "callee")
	if nativeBlockMetadata(h, 1) != "callee" || nativeBlockEntries(data)[0] != (nativeBlockEntry{3, 6, 0, "", "callee"}) {
		t.Fatal("reused block content")
	}
	if ti.TransIntInfo()[6] != "callee" {
		t.Fatal("legacy fallback lost value")
	}
	if data, _ = h.NativeBlock(); data != nil {
		t.Fatal("legacy fallback kept exposing a block")
	}
}

func TestNativeBlockGrowthKeepsOffsetsAndEarlierStorage(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	h.EnablePrimary(false)
	h.SetInt(3, "caller")
	before, _ := h.NativeBlock()
	snapshot := append([]byte(nil), before...)
	large := strings.Repeat("v", 3*headerBlockInitialSize)
	h.SetString("large", large)
	for i := 0; i < 64; i++ {
		h.SetInt(uint16(100+i), "x")
	}
	after, _ := h.NativeBlock()
	if &after[0] == &before[0] || len(after) <= len(before) || !bytes.Equal(before[16:], snapshot[16:]) {
		t.Fatal("growth reused or rewrote the earlier block")
	}
	got := nativeBlockEntries(after)
	if len(got) != 66 || got[0] != (nativeBlockEntry{3, 3, 0, "", "caller"}) || got[1].Value != large || got[65] != (nativeBlockEntry{3, 163, 0, "", "x"}) || nativeBlockMetadata(h, 0) != "caller" {
		t.Fatal("growth lost entries")
	}
	if binary.LittleEndian.Uint32(after[headerBlockMetadata:]) != binary.LittleEndian.Uint32(snapshot[headerBlockMetadata:]) {
		t.Fatal("growth moved data offsets")
	}
	ti.zero()
	h.EnablePrimary(false)
	if again, _ := h.NativeBlock(); &again[0] != &after[0] {
		t.Fatal("grown block was not retained for reuse")
	}
}

func TestNativeBlockPinnedOnceAndReleasedWithOwner(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	h.EnablePrimary(false)
	first, pinned := h.NativeBlock()
	if !pinned || ti.store.block.pins == nil || ti.store.block.grown {
		t.Skip("header blocks are pinned by their owner only on go1.21 and later")
	}
	pins := ti.store.block.pins
	for i := 0; i < 3; i++ {
		ti.zero()
		h.EnablePrimary(i%2 == 1)
		h.SetInt(3, "caller")
		if again, pinned := h.NativeBlock(); &again[0] != &first[0] || !pinned || ti.store.block.pins != pins || ti.store.block.grown {
			t.Fatal("reuse replaced the pinned block")
		}
	}
	h.SetString("large", strings.Repeat("v", 4*headerBlockInitialSize))
	grown, _ := h.NativeBlock()
	if &grown[0] == &first[0] || !ti.store.block.grown || ti.store.block.pins != pins {
		t.Fatal("growth did not keep both blocks under the same pins")
	}
	_ = ti.TransIntInfo()
	if ti.store.block.grown == false || &ti.store.block.data[0] != &grown[0] {
		t.Fatal("legacy fallback released blocks that native may still borrow")
	}
	ti.zero()
	h.EnablePrimary(false)
	if again, pinned := h.NativeBlock(); &again[0] != &grown[0] || !pinned || ti.store.block.grown {
		t.Fatal("recycle did not keep the grown block pinned for reuse")
	}
	h.SetString("huge", strings.Repeat("v", 2*headerBlockRetainedSize))
	if huge, _ := h.NativeBlock(); len(huge) <= headerBlockRetainedSize || h.String("huge") == "" {
		t.Fatal("oversized block")
	}
	ti.zero()
	if ti.store.block.data != nil || ti.store.block.grown {
		t.Fatal("recycle retained an oversized block")
	}
	h.EnablePrimary(false)
	h.SetInt(3, "caller")
	if small, pinned := h.NativeBlock(); len(small) != headerBlockInitialSize || !pinned || nativeBlockMetadata(h, 0) != "caller" || len(nativeBlockEntries(small)) != 1 {
		t.Fatal("store did not restart from a fresh pinned block")
	}
	for i := 0; i < 64; i++ {
		dropped := newTransInfo().(*transInfo)
		a := AccessHeaders(dropped)
		a.EnablePrimary(false)
		a.SetString("large", strings.Repeat("v", 4*headerBlockInitialSize))
		if _, pinned := a.NativeBlock(); !pinned {
			t.Fatal("dropped store was not pinned")
		}
	}
	for i := 0; i < 4; i++ {
		runtime.GC()
	}
}

func TestNativeBlockDecodeFillsEntriesAndMetadata(t *testing.T) {
	var frame []byte
	w := bufiox.NewBytesWriter(&frame)
	if _, err := ttheader.Encode(context.Background(), ttheader.EncodeParam{IntInfo: map[uint16]string{3: "caller", 9: "Method"}, StrInfo: map[string]string{"tail": "value", ttheader.GDPRToken: "token"}}, w); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(frame, uint32(len(frame)-4))
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	h.EnablePrimary(false)
	if _, err := h.DecodeTTHeader(context.Background(), bufiox.NewBytesReader(frame)); err != nil {
		t.Fatal(err)
	}
	data, _ := h.NativeBlock()
	found := map[nativeBlockEntry]bool{}
	for _, e := range nativeBlockEntries(data) {
		found[e] = true
	}
	for _, e := range []nativeBlockEntry{{3, 3, 0, "", "caller"}, {3, 9, 0, "", "Method"}, {2, 0, 0, "tail", "value"}, {2, 0, 0, ttheader.GDPRToken, "token"}} {
		if !found[e] {
			t.Fatalf("missing %+v in %+v", e, found)
		}
	}
	if len(found) != 4 || nativeBlockMetadata(h, 0) != "caller" || nativeBlockMetadata(h, 8) != "Method" || nativeBlockMetadata(h, 15) != "token" {
		t.Fatal("decoded metadata")
	}
}

func TestNativeBlockSlotsIndexKnownFields(t *testing.T) {
	keys := []string{"RPC_PERSIST_SERVICE_LEVEL", "RPC_PERSIST_ori_psm", "RPC_PERSIST_SAMPLE_REQUEST", "RPC_TRANSIT_secondary-acl-token", "Rpc-Persist-Lane-P-Did", "RPC_TRANSIT_EGRESS_SENT_TS", "sc", "crrst", "RPC_PERSIST_crrst", "K_ProcessAtTime"}
	if headerBlockSlots+2*headerBlockSlotCount != headerBlockHeadSize || headerBlockIntSlots+len(keys) > headerBlockSlotCount {
		t.Fatal("slot table layout")
	}
	for _, response := range []bool{false, true} {
		ti := newTransInfo().(*transInfo)
		h := AccessHeaders(ti)
		h.EnablePrimary(response)
		for id := uint16(0); id < 40; id++ {
			if id != 32 {
				h.SetInt(id, "v")
			}
		}
		for _, key := range keys {
			h.SetString(key, "value")
		}
		h.SetString("tail", "value")
		h.SetString(ttheader.GDPRToken, "token")
		data, _ := h.NativeBlock()
		entries := nativeBlockEntries(data)
		stringKind, intKind := ti.store.kinds()
		for slot := 0; slot < headerBlockSlotCount; slot++ {
			at := nativeBlockSlot(data, slot)
			switch {
			case slot < headerBlockIntSlots:
				if at == 0 || entries[at-1] != (nativeBlockEntry{intKind, uint16(slot), 0, "", "v"}) {
					t.Fatalf("int slot %d: %d", slot, at)
				}
			case slot < headerBlockIntSlots+len(keys):
				if at == 0 || entries[at-1] != (nativeBlockEntry{stringKind, 0, 0, keys[slot-headerBlockIntSlots], "value"}) || headerBlockStringSlot(keys[slot-headerBlockIntSlots]) != slot {
					t.Fatalf("string slot %d: %d", slot, at)
				}
			default:
				if at != 0 {
					t.Fatalf("reserved slot %d: %d", slot, at)
				}
			}
		}
		if headerBlockStringSlot("tail") != -1 || headerBlockStringSlot(ttheader.GDPRToken) != -1 || headerBlockStringSlot("") != -1 {
			t.Fatal("long-tail keys were given slots")
		}
		h.SetString("sc", "changed")
		h.DeleteInt(7)
		h.SetString("crrst", "")
		data, _ = h.NativeBlock()
		entries = nativeBlockEntries(data)
		if at := nativeBlockSlot(data, headerBlockIntSlots+6); entries[at-1] != (nativeBlockEntry{stringKind, 0, 0, "sc", "changed"}) || at != len(entries)-2 {
			t.Fatal("replaced slot does not point at the latest entry")
		}
		if at := nativeBlockSlot(data, 7); entries[at-1] != (nativeBlockEntry{intKind, 7, 1, "", ""}) || at != len(entries)-1 {
			t.Fatal("deleted slot does not point at the tombstone")
		}
		if at := nativeBlockSlot(data, headerBlockIntSlots+7); entries[at-1] != (nativeBlockEntry{stringKind, 0, 0, "crrst", ""}) || at != len(entries) {
			t.Fatal("empty value is not distinguishable from a missing slot")
		}
		ti.zero()
		h.EnablePrimary(response)
		data, _ = h.NativeBlock()
		for slot := 0; slot < headerBlockSlotCount; slot++ {
			if nativeBlockSlot(data, slot) != 0 {
				t.Fatalf("stale slot %d", slot)
			}
		}
	}
}
