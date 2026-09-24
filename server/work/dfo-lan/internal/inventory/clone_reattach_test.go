package inventory

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNonAvatarWornSpaceUpdatePreservesOrdinaryEquipmentRows(t *testing.T) {
	base := EquipmentRow(BagEquipment{Slot: 12, Template: 101000013})
	record := append([]byte(nil), base[:]...)
	record[60] = 0x7a
	worn := []BagEquipment{
		{Slot: 3, Template: 517500000},
		{Slot: 12, Template: 101000013, Record: record},
		{Slot: 26, Template: 500991361},
		{Slot: 47, Template: 100610096},
	}
	state, err := SaveBag(json.RawMessage(`{}`), Bag{Version: "ordinary-bag-v1", Worn: worn})
	if err != nil {
		t.Fatal(err)
	}
	got, err := NonAvatarWornSpaceUpdate(state)
	if err != nil {
		t.Fatal(err)
	}
	want, err := EquipmentPayload(3, []BagEquipment{worn[1], worn[3]}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("ordinary equipment or its original 181-byte row was not preserved")
	}
}
