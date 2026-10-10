package remote

import (
	"reflect"
	"testing"

	"github.com/cloudwego/gopkg/protocol/ttheader"
)

func TestPrimarySinkACLOrder(t *testing.T) {
	for _, response := range []bool{false, true} {
		for _, stringFirst := range []bool{false, true} {
			primary, legacy := newTransInfo().(*transInfo), newTransInfo().(*transInfo)
			a, b := AccessHeaders(primary), AccessHeaders(legacy)
			a.EnablePrimary(response)
			primary.SetInt(65535, "opaque\x00value")
			b.SetInt(65535, "opaque\x00value")
			primary.SetString("", "")
			b.SetString("", "")
			if stringFirst {
				primary.SetString(ttheader.GDPRToken, "string")
				b.SetString(ttheader.GDPRToken, "string")
			} else {
				primary.SetInt(32, "integer")
				b.SetInt(32, "integer")
			}
			if !a.Primary() {
				t.Fatal("single ACL field materialized maps")
			}
			primary.SetInt(32, "integer")
			b.SetInt(32, "integer")
			primary.SetString(ttheader.GDPRToken, "string")
			b.SetString(ttheader.GDPRToken, "string")
			if a.Primary() {
				t.Fatal("mixed ACL fields did not materialize maps")
			}
			primary.SetInt(9, "after")
			b.SetInt(9, "after")
			primary.SetString("mct", "after")
			b.SetString("mct", "after")
			ints, strs := a.LegacyMaps()
			wantInts, wantStrs := b.LegacyMaps()
			if !reflect.DeepEqual(ints, wantInts) || !reflect.DeepEqual(strs, wantStrs) {
				t.Fatal("typed sink changed ACL fallback or opaque fields")
			}
		}
	}
}
