package remote

import (
	"context"
	"encoding/binary"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/cloudwego/gopkg/bufiox"
	"github.com/cloudwego/gopkg/protocol/ttheader"
)

func TestPrimaryHeaderSynchronousBorrowAndReuse(t *testing.T) {
	for _, response := range []bool{false, true} {
		ti := newTransInfo().(*transInfo)
		h := AccessHeaders(ti)
		h.EnablePrimary(response)
		var copies [][]HeaderEntry
		for i := 0; i < 8; i++ {
			h.SetString("unknown", strings.Clone(strconv.Itoa(i)))
			h.SetInt(65530, strings.Clone(strconv.Itoa(i)))
			view := h.View()
			copies = append(copies, append([]HeaderEntry(nil), view.entries...))
		}
		runtime.GC()
		for i, entries := range copies {
			if entries[0].Value != strconv.Itoa(i) || entries[1].Value != strconv.Itoa(i) {
				t.Fatalf("copied view %d changed", i)
			}
			wantString, wantInt := uint16(2), uint16(3)
			if response {
				wantString, wantInt = 4, 10
			}
			if entries[0].Kind != wantString || entries[1].Kind != wantInt {
				t.Fatal("domain changed")
			}
		}
		h.DeleteString("unknown")
		if _, exists := h.StringValue("unknown"); exists {
			t.Fatal("delete did not reach map")
		}
		if copies[7][0].Value != "7" {
			t.Fatal("delete mutated copied view")
		}
		ti.zero()
		for _, e := range ti.store.view.entries[:cap(ti.store.view.entries)] {
			if e != (HeaderEntry{}) {
				t.Fatal("recycle retained string roots")
			}
		}
		h.EnablePrimary(response)
		h.SetInt(9, "Echo")
		if !h.View().Valid() || h.View().Len() != 1 {
			t.Fatal("reuse failed")
		}
	}
}

func TestPrimaryHeaderPointerView(t *testing.T) {
	var missing *HeaderView
	if missing.Valid() || missing.Len() != 0 || AccessHeaders(nil).View() != nil {
		t.Fatal("nil view is not empty")
	}
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	if h.View() != nil || !h.EnablePrimary(false) {
		t.Fatal("fresh view state")
	}
	view := h.View()
	if view != &ti.store.view || !view.Valid() || view.Len() != 0 {
		t.Fatal("view does not borrow the owner")
	}
	h.SetInt(3, "caller")
	if h.View() != view || view.Len() != 1 || view.metadataIndex[0] != 1 {
		t.Fatal("owner view identity changed")
	}
	if ti.TransIntInfo()[3] != "caller" || h.View() != nil || h.View().Valid() || h.View().Len() != 0 {
		t.Fatal("legacy fallback retained a trusted view")
	}
	ti.zero()
	if h.View() != nil {
		t.Fatal("recycle retained a trusted view")
	}
}

func TestPrimaryHeaderStickyAliases(t *testing.T) {
	for _, kind := range []string{"getter", "put-int", "put-string"} {
		t.Run(kind, func(t *testing.T) {
			ti := newTransInfo().(*transInfo)
			h := AccessHeaders(ti)
			h.EnablePrimary(false)
			var ints map[uint16]string
			var strs map[string]string
			switch kind {
			case "getter":
				strs = ti.TransStrInfo()
			case "put-int":
				ints = map[uint16]string{9: "Echo"}
				ti.PutTransIntInfo(ints)
			case "put-string":
				strs = map[string]string{"unknown": "before"}
				ti.PutTransStrInfo(strs)
			}
			h.SetString("safe", "value")
			h.EnablePrimary(false)
			if h.View().Valid() {
				t.Fatal("escape was reset inside message lifetime")
			}
			if ints != nil {
				ints[9] = "late"
				if h.Int(9) != "late" {
					t.Fatal("Put alias lost")
				}
			}
			if strs != nil {
				strs["late"] = "alias"
				if h.String("late") != "alias" {
					t.Fatal("getter/Put alias lost")
				}
			}
			ti.zero()
			h.EnablePrimary(false)
			h.SetInt(9, "new")
			if !h.View().Valid() {
				t.Fatal("new map identity did not restore tracking")
			}
			if ints != nil {
				ints[9] = "stale"
			}
			if strs != nil {
				strs["late"] = "stale"
			}
			if h.Int(9) != "new" || h.String("late") != "" {
				t.Fatal("old alias polluted renewed owner")
			}
		})
	}
	ti := newTransInfo().(*transInfo)
	ti.zero()
	alias := ti.TransStrInfo()
	h := AccessHeaders(ti)
	h.EnablePrimary(false)
	alias["live"] = "value"
	if h.View().Valid() || h.String("live") != "value" {
		t.Fatal("live pre-enable alias was detached")
	}
}

func TestPrimaryHeaderACLUnknownAndPlain(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	h.SetString("unknown", "value")
	if ti.store != nil || h.View().Valid() {
		t.Fatal("plain path allocated tracker")
	}
	ti.zero()
	h.EnablePrimary(false)
	h.SetInt(32, "integer")
	h.SetInt(65535, "unknown")
	h.SetString("", "")
	if !h.View().Valid() || h.View().Len() != 3 || h.View().metadataIndex[15] != 0 {
		t.Fatal("int ACL or unknown changed")
	}
	h.SetString("RPC_TRANSIT_gdpr-token", "string")
	if h.View().Valid() {
		t.Fatal("ambiguous aliases did not deopt")
	}
	h.DeleteInt(32)
	if h.View().Valid() {
		t.Fatal("invalid tracking resurrected")
	}
	ints, strs := h.LegacyMaps()
	if len(ints) != 1 || strs["RPC_TRANSIT_gdpr-token"] != "string" {
		t.Fatal("authoritative maps changed")
	}
}

func TestPrimaryHeaderEncodePreservesTracking(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	h.EnablePrimary(false)
	h.SetString("unknown", "x\x00y")
	h.SetInt(65530, "value")
	var wire []byte
	out := bufiox.NewBytesWriter(&wire)
	length, err := h.EncodeTTHeader(context.Background(), ttheader.EncodeParam{ProtocolID: ttheader.ProtocolIDThriftBinary}, out)
	if err != nil {
		t.Fatal(err)
	}
	if err := out.Flush(); err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(length, uint32(len(wire)-4))
	if !h.View().Valid() {
		t.Fatal("encoder exposed map")
	}
	p, err := ttheader.Decode(context.Background(), bufiox.NewBytesReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	ints, strs := collectPrimaryHeaders(h)
	if !reflect.DeepEqual(p.IntInfo, ints) || !reflect.DeepEqual(p.StrInfo, strs) {
		t.Fatal("wire headers changed")
	}
	t.Logf("store=%d transInfo=%d entry=%d retained=%d", unsafe.Sizeof(headerStore{}), unsafe.Sizeof(transInfo{}), unsafe.Sizeof(HeaderEntry{}), cap(ti.store.view.entries))
}

func TestPrimaryHeaderInvalidAndNonEmptyMerge(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	h.EnablePrimary(false)
	h.SetInt(3, "caller")
	h.SetString("existing", "value")
	ints := map[uint16]string{65530: "unknown"}
	strs := map[string]string{"unknown": "original"}
	ti.PutTransIntInfo(ints)
	ti.PutTransStrInfo(strs)
	ints[65530] = "changed"
	strs["unknown"] = "changed"
	if h.Primary() || h.Int(65530) != "unknown" || h.String("unknown") != "original" {
		t.Fatal("nonempty merge adopted source alias")
	}
	h.SetString("oversized", strings.Repeat("x", (16<<20)+1))
	if h.Primary() || h.View().Valid() {
		t.Fatal("oversized header did not invalidate")
	}
	h.DeleteString("oversized")
	if h.View().Valid() {
		t.Fatal("invalid view revived")
	}
	ti.zero()
	h.EnablePrimary(false)
	if !h.Primary() {
		t.Fatal("invalid state leaked across reset")
	}
}

func collectPrimaryHeaders(h HeaderAccess) (map[uint16]string, map[string]string) {
	ints, strs := make(map[uint16]string), make(map[string]string)
	h.RangeInt(func(key uint16, value string) bool { ints[key] = value; return true })
	h.RangeString(func(key, value string) bool { strs[key] = value; return true })
	return ints, strs
}

func primaryTestWire(t *testing.T, ints map[uint16]string, strs map[string]string) []byte {
	t.Helper()
	var wire []byte
	out := bufiox.NewBytesWriter(&wire)
	field, err := ttheader.Encode(context.Background(), ttheader.EncodeParam{ProtocolID: ttheader.ProtocolIDThriftBinary, IntInfo: ints, StrInfo: strs}, out)
	if err != nil {
		t.Fatal(err)
	}
	if err = out.Flush(); err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(field, uint32(len(wire)-4))
	return wire
}

func TestPrimaryHeaderCanonicalCountsAndDeletion(t *testing.T) {
	for _, response := range []bool{false, true} {
		ti := newTransInfo().(*transInfo)
		h := AccessHeaders(ti)
		if !h.EnablePrimary(response) {
			t.Fatal("enable")
		}
		h.SetInt(9, "Echo")
		h.SetInt(65535, "unknown")
		h.SetString("", "empty-key")
		h.SetString(ttheader.GDPRToken, "old")
		h.SetString(ttheader.GDPRToken, "new")
		h.DeleteString(ttheader.GDPRToken)
		h.DeleteString(ttheader.GDPRToken)
		h.SetString(ttheader.GDPRToken, "current")
		h.DeleteInt(65535)
		if ti.intInfo != nil || ti.strInfo != nil {
			t.Fatal("primary created value maps")
		}
		if ti.store.intCount != 1 || ti.store.strCount != 2 || ti.store.gdprIndex == 0 {
			t.Fatal("counts/index")
		}
		ints, strs := collectPrimaryHeaders(h)
		if len(ints) != 1 || ints[9] != "Echo" || len(strs) != 2 || strs[ttheader.GDPRToken] != "current" {
			t.Fatal("range values")
		}
		calls := 0
		h.RangeString(func(string, string) bool { calls++; return false })
		if calls != 1 {
			t.Fatal("early stop")
		}
		var wire []byte
		out := bufiox.NewBytesWriter(&wire)
		field, err := h.EncodeTTHeader(context.Background(), ttheader.EncodeParam{}, out)
		if err != nil {
			t.Fatal(err)
		}
		if err = out.Flush(); err != nil {
			t.Fatal(err)
		}
		binary.BigEndian.PutUint32(field, uint32(len(wire)-4))
		p, err := ttheader.Decode(context.Background(), bufiox.NewBytesReader(wire))
		if err != nil || !reflect.DeepEqual(p.IntInfo, ints) || !reflect.DeepEqual(p.StrInfo, strs) {
			t.Fatalf("roundtrip: %v", err)
		}
		if !h.Primary() || ti.intInfo != nil || ti.strInfo != nil {
			t.Fatal("encoder materialized maps")
		}
	}
}

func TestPrimaryHeaderDecodeAndReaderLifetime(t *testing.T) {
	for _, response := range []bool{false, true} {
		ints := map[uint16]string{3: "poc.caller", 9: "Echo", 65535: "unknown"}
		strs := map[string]string{"": "empty-key", "opaque": "a\x00b", ttheader.GDPRToken: "token"}
		wire := primaryTestWire(t, ints, strs)
		ti := newTransInfo().(*transInfo)
		h := AccessHeaders(ti)
		h.EnablePrimary(response)
		reader := bufiox.NewBytesReader(wire)
		p, err := h.DecodeTTHeader(context.Background(), reader)
		if err != nil {
			t.Fatal(err)
		}
		if err := reader.Release(nil); err != nil {
			t.Fatal(err)
		}
		for i := range wire {
			wire[i] = 0xff
		}
		runtime.GC()
		actualInt, actualString := collectPrimaryHeaders(h)
		if !reflect.DeepEqual(actualInt, ints) || !reflect.DeepEqual(actualString, strs) {
			t.Fatal("reader release poisoned store")
		}
		if p.IntInfo != nil || p.StrInfo != nil || ti.intInfo != nil || ti.strInfo != nil {
			t.Fatal("decode populated value maps")
		}
		if !h.View().Valid() {
			t.Fatal("decode deoptimized")
		}
		wantString, wantInt := uint16(2), uint16(3)
		if response {
			wantString, wantInt = 4, 10
		}
		for _, e := range h.View().entries {
			if e.Kind != wantString && e.Kind != wantInt {
				t.Fatal("decode domain")
			}
		}
	}
}

func TestPrimaryHeaderDecodeFailureAtomicity(t *testing.T) {
	good := primaryTestWire(t, map[uint16]string{9: "ok"}, nil)
	bad := append([]byte(nil), good...)
	if bad[len(bad)-1] != 0 {
		t.Fatal("fixture needs one padding byte")
	}
	bad[len(bad)-1] = 0xff
	for _, enabled := range []bool{false, true} {
		for _, nonempty := range []bool{false, true} {
			ti := newTransInfo().(*transInfo)
			h := AccessHeaders(ti)
			if enabled {
				h.EnablePrimary(false)
			}
			if nonempty {
				h.SetInt(9, "original")
				h.SetString("prior", "keep")
			}
			if _, err := h.DecodeTTHeader(context.Background(), bufiox.NewBytesReader(bad)); err == nil {
				t.Fatal("expected malformed header")
			}
			ints, strs := collectPrimaryHeaders(h)
			if nonempty {
				if len(ints) != 1 || ints[9] != "original" || len(strs) != 1 || strs["prior"] != "keep" {
					t.Fatal("failed decode changed existing headers")
				}
			} else if len(ints) != 0 || len(strs) != 0 {
				t.Fatal("failed fresh decode retained partial fields")
			}
			if _, err := h.DecodeTTHeader(context.Background(), bufiox.NewBytesReader(good)); err != nil {
				t.Fatal(err)
			}
			if h.Int(9) != "ok" || (nonempty && h.String("prior") != "keep") {
				t.Fatal("decode retry/merge")
			}
			ti.zero()
			if !h.EnablePrimary(false) {
				t.Fatal("recycle did not restore primary")
			}
		}
	}
}

func TestPrimaryHeaderLegacySameCardinalityAndDirection(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	h.EnablePrimary(false)
	h.SetString("old", "before")
	alias := ti.TransStrInfo()
	delete(alias, "old")
	alias["new"] = "after"
	if h.EnablePrimary(false) || h.Primary() || h.String("old") != "" || h.String("new") != "after" {
		t.Fatal("same cardinality alias escaped detection")
	}
	ti.zero()
	if !h.EnablePrimary(false) {
		t.Fatal("fresh primary")
	}
	h.SetInt(9, "Echo")
	if h.EnablePrimary(true) || h.Primary() || h.Int(9) != "Echo" {
		t.Fatal("role switch silently reused kinds")
	}
}

func TestPrimaryHeaderCapacityFallbackKeepsCurrentWrite(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	h := AccessHeaders(ti)
	h.EnablePrimary(false)
	h.SetInt(9, "Echo")
	value := strings.Repeat("x", (16<<20)+1)
	var sink ttheader.HeaderSink = ti
	sink.SetString("oversized", value)
	if h.Primary() || h.Int(9) != "Echo" || h.String("oversized") != value {
		t.Fatal("fallback dropped current field")
	}
	h.DeleteString("oversized")
	if h.EnablePrimary(false) {
		t.Fatal("fallback was not sticky")
	}
}

type primaryCustomInfo struct {
	ints             map[uint16]string
	strs             map[string]string
	intPuts, strPuts int
}

func (c *primaryCustomInfo) TransIntInfo() map[uint16]string { return c.ints }
func (c *primaryCustomInfo) TransStrInfo() map[string]string { return c.strs }
func (c *primaryCustomInfo) PutTransIntInfo(values map[uint16]string) {
	if values == nil {
		return
	}
	c.intPuts++
	if len(c.ints) == 0 {
		c.ints = values
		return
	}
	for key, value := range values {
		c.ints[key] = value
	}
}
func (c *primaryCustomInfo) PutTransStrInfo(values map[string]string) {
	if values == nil {
		return
	}
	c.strPuts++
	if len(c.strs) == 0 {
		c.strs = values
		return
	}
	for key, value := range values {
		c.strs[key] = value
	}
}
func (c *primaryCustomInfo) Recycle() { c.ints, c.strs = nil, nil }

func TestPrimaryHeaderCustomAndNil(t *testing.T) {
	empty := AccessHeaders(nil)
	if empty.EnablePrimary(false) || empty.Primary() || empty.View().Valid() {
		t.Fatal("nil enabled")
	}
	empty.SetInt(9, "unused")
	empty.SetString("unused", "value")
	empty.DeleteInt(9)
	empty.DeleteString("unused")
	if _, ok := empty.IntValue(9); ok {
		t.Fatal("nil lookup")
	}
	if _, ok := empty.StringValue("unused"); ok {
		t.Fatal("nil lookup")
	}
	empty.RangeInt(func(uint16, string) bool { t.Fatal("nil range"); return true })
	empty.RangeString(func(string, string) bool { t.Fatal("nil range"); return true })
	custom := &primaryCustomInfo{}
	h := AccessHeaders(custom)
	if h.EnablePrimary(false) {
		t.Fatal("custom enabled")
	}
	wire := primaryTestWire(t, map[uint16]string{65535: "unknown"}, map[string]string{"": "empty"})
	if _, err := h.DecodeTTHeader(context.Background(), bufiox.NewBytesReader(wire)); err != nil {
		t.Fatal(err)
	}
	if h.Int(65535) != "unknown" || h.String("") != "empty" {
		t.Fatal("nil map return was not installed")
	}
	h.SetInt(9, "Echo")
	h.SetString("new", "value")
	h.DeleteInt(65535)
	h.DeleteString("")
	ints, strs := collectPrimaryHeaders(h)
	if len(ints) != 1 || ints[9] != "Echo" || len(strs) != 1 || strs["new"] != "value" {
		t.Fatal("custom safe access")
	}
}

func TestPrimaryHeaderCustomSetPreservesDirectMapContract(t *testing.T) {
	for _, allocated := range []bool{false, true} {
		custom := &primaryCustomInfo{}
		if allocated {
			custom.ints, custom.strs = map[uint16]string{}, map[string]string{}
		}
		intAlias, strAlias := custom.ints, custom.strs
		h := AccessHeaders(custom)
		h.SetInt(9, "Echo")
		h.SetString("key", "value")
		if allocated {
			if custom.intPuts != 0 || custom.strPuts != 0 || intAlias[9] != "Echo" || strAlias["key"] != "value" {
				t.Fatal("custom direct map became Put")
			}
		} else if custom.intPuts != 1 || custom.strPuts != 1 {
			t.Fatal("nil map did not use Put")
		}
		intPuts, strPuts := custom.intPuts, custom.strPuts
		h.SetInt(9, "Changed")
		h.SetString("key", "changed")
		h.DeleteInt(9)
		h.DeleteString("key")
		if custom.intPuts != intPuts || custom.strPuts != strPuts || len(custom.ints) != 0 || len(custom.strs) != 0 {
			t.Fatal("custom direct update/delete")
		}
	}
}

func TestPrimaryHeaderPointerSinkEquivalent(t *testing.T) {
	for _, response := range []bool{false, true} {
		valueInfo, pointerInfo := newTransInfo().(*transInfo), newTransInfo().(*transInfo)
		valueAccess, pointerAccess := AccessHeaders(valueInfo), AccessHeaders(pointerInfo)
		valueAccess.EnablePrimary(response)
		pointerAccess.EnablePrimary(response)
		var valueSink ttheader.HeaderSink = valueAccess
		var pointerSink ttheader.HeaderSink = pointerInfo
		for _, sink := range []ttheader.HeaderSink{valueSink, pointerSink} {
			sink.SetInt(3, "caller")
			sink.SetInt(9, "Echo")
			sink.SetInt(65535, "unknown")
			sink.SetString("", "")
			sink.SetString("unknown", "a\x00b")
			sink.SetString("unknown", "updated")
		}
		if !reflect.DeepEqual(valueAccess.View(), pointerAccess.View()) {
			t.Fatal("pointer sink changed entries or metadata")
		}
		for _, sink := range []ttheader.HeaderSink{valueSink, pointerSink} {
			sink.SetInt(32, "int-token")
			sink.SetString(ttheader.GDPRToken, "string-token")
			sink.SetString("after-fallback", "kept")
		}
		wantInts, wantStrings := collectPrimaryHeaders(valueAccess)
		gotInts, gotStrings := collectPrimaryHeaders(pointerAccess)
		if valueAccess.Primary() || pointerAccess.Primary() || !reflect.DeepEqual(wantInts, gotInts) || !reflect.DeepEqual(wantStrings, gotStrings) {
			t.Fatal("pointer sink changed fallback fields")
		}
	}
}
