package remote

import "testing"

func TestNativeHeaderChangesCheckpointAndFallback(t *testing.T) {
	msg := NewMessage(nil, nil, Reply, Client)
	defer RecycleMessage(msg)
	access := AccessHeaders(msg.TransInfo())
	access.EnablePrimary(false)
	access.SetString("existing", "initial")
	access.SetInt(3, "caller")
	if !access.ResetNativeChanges(true) {
		t.Fatal("tracking could not start")
	}
	access.SetString("existing", "initial")
	if access.NativeChanges().Len() != 0 {
		t.Fatal("equal value was recorded as a change")
	}
	access.SetString("existing", "intermediate")
	access.SetString("existing", "final")
	access.DeleteInt(3)
	access.SetString("empty", "")
	view := access.NativeChanges()
	if view.Len() != 3 || view.entries[0].Value != "final" || view.entries[1].Op != 1 || view.entries[2].Value != "" || view.entries[2].Op != 0 {
		t.Fatal("change list lost overwrite, delete or empty value semantics")
	}
	access.ResetNativeChanges(true)
	if access.NativeChanges().Len() != 0 {
		t.Fatal("native checkpoint retained already applied operations")
	}
	alias := msg.TransInfo().TransStrInfo()
	alias["existing"] = "legacy-value"
	if access.NativeChanges() != nil || access.ResetNativeChanges(true) {
		t.Fatal("escaped map did not disable delta tracking")
	}
	if alias["existing"] != "legacy-value" {
		t.Fatal("legacy map mutation lost")
	}
}

func TestNativeHeaderChangesSteadyReuse(t *testing.T) {
	msg := NewMessage(nil, nil, Reply, Client)
	defer RecycleMessage(msg)
	access := AccessHeaders(msg.TransInfo())
	access.EnablePrimary(false)
	access.SetString("existing", "initial")
	allocations := testing.AllocsPerRun(100, func() {
		access.ResetNativeChanges(true)
		access.SetString("existing", "first")
		access.SetString("existing", "second")
		if access.NativeChanges().Len() != 1 {
			t.Fatal("overwrite was not deduplicated")
		}
		access.ResetNativeChanges(false)
	})
	if allocations != 0 {
		t.Fatalf("steady native changes allocated %v objects", allocations)
	}
	store := access.info.(*transInfo).store
	for _, entry := range store.changeView.entries[:cap(store.changeView.entries)] {
		if entry != (HeaderEntry{}) {
			t.Fatal("inactive change view retained a string")
		}
	}
}
