package remote

import (
	"fmt"
	"reflect"
	"testing"
)

func TestPrimaryDenseSlots(t *testing.T) {
	ti := newTransInfo().(*transInfo)
	a := AccessHeaders(ti)
	if !a.EnablePrimary(false) {
		t.Fatal("primary not enabled")
	}
	for round := 0; round < 3; round++ {
		for id := uint16(0); id < 33; id++ {
			a.SetInt(id, "value")
			if v, ok := a.IntValue(id); !ok || v != "value" {
				t.Fatalf("missing int %d", id)
			}
		}
		for _, key := range []string{"mct", "mtt", "rip", "isn", "RPC_TRANSIT_EGRESS_SENT_TS", "RPC_TRANSIT_secondary-acl-token"} {
			a.SetString(key, "value")
			a.DeleteString(key)
			if _, ok := a.StringValue(key); ok {
				t.Fatal("deleted string retained")
			}
			a.SetString(key, "")
			if v, ok := a.StringValue(key); !ok || v != "" {
				t.Fatal("empty string lost")
			}
		}
		if (round == 0 && (ti.store.intSlots != nil || ti.store.strSlots != nil)) || len(ti.store.intSlots) != 0 || len(ti.store.strSlots) != 0 {
			t.Fatal("common headers allocated slot maps")
		}
		for _, id := range []uint16{33, 64, 65535} {
			a.SetInt(id, "unknown")
			if a.Int(id) != "unknown" {
				t.Fatal("unknown ID lost")
			}
			a.DeleteInt(id)
			if _, ok := a.IntValue(id); ok {
				t.Fatal("deleted unknown ID retained")
			}
		}
		for _, key := range []string{"", "unknown", "key\x00suffix"} {
			a.SetString(key, "unknown")
			if a.String(key) != "unknown" {
				t.Fatal("unknown string lost")
			}
		}
		ti.store.reset(false)
		for id := uint16(0); id < 33; id++ {
			if _, ok := a.IntValue(id); ok {
				t.Fatal("stale int slot after reset")
			}
		}
		for _, key := range []string{"mct", "rip", "unknown", ""} {
			if _, ok := a.StringValue(key); ok {
				t.Fatal("stale string slot after reset")
			}
		}
	}
}

func TestPrimaryDenseDifferential(t *testing.T) {
	primary, legacy := newTransInfo().(*transInfo), newTransInfo().(*transInfo)
	a, b := AccessHeaders(primary), AccessHeaders(legacy)
	a.EnablePrimary(false)
	for round := 0; round < 4; round++ {
		for _, id := range []uint16{0, 2, 3, 9, 31, 32, 33, 65535} {
			value := fmt.Sprint(round, id)
			a.SetInt(id, value)
			b.SetInt(id, value)
			if a.Int(id) != b.Int(id) {
				t.Fatal("int mismatch")
			}
		}
		for _, key := range []string{"mct", "mtt", "rip", "isn", "", "custom", "RPC_TRANSIT_secondary-acl-token", "RPC_TRANSIT_EGRESS_SENT_TS"} {
			value := fmt.Sprint(round, key)
			a.SetString(key, value)
			b.SetString(key, value)
			if a.String(key) != b.String(key) {
				t.Fatal("string mismatch")
			}
		}
		for _, id := range []uint16{0, 9, 32, 65535} {
			a.DeleteInt(id)
			b.DeleteInt(id)
		}
		for _, key := range []string{"mct", "rip", "custom", ""} {
			a.DeleteString(key)
			b.DeleteString(key)
		}
	}
	ints, strs := a.LegacyMaps()
	wantInts, wantStrs := b.LegacyMaps()
	if !reflect.DeepEqual(ints, wantInts) || !reflect.DeepEqual(strs, wantStrs) {
		t.Fatal("materialized maps differ")
	}
}
