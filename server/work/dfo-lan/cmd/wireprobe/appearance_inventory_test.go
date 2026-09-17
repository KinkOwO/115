package main

import (
	"bytes"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func TestAppearanceRestoresAvatarAndWornInstances(t *testing.T) {
	raw := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":3,"template":40601,"durability":9}],"special_equipment":{"1":[]}}}`)
	before := bytes.Clone(raw)
	plan, e := appearanceInventory(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 4 || plan[0].ID != 13 || plan[0].Payload[0] != 1 || plan[2].Payload[0] != 3 || plan[3].ID != 14 {
		t.Fatal(plan)
	}
	want, e := inventory.WornPayload(raw)
	if e != nil || !bytes.Equal(want, plan[2].Payload) {
		t.Fatal(e, "worn instance mismatch")
	}
	if !bytes.Equal(before, raw) {
		t.Fatal("state changed")
	}
}
