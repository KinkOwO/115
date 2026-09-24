package inventory

import (
	"dfolan/internal/game/protocol"
	"testing"
)

func TestMoveVaultCrossPartialMergeAndStrictSlot(t *testing.T) {
	a := Vault{Slots: 8, Items: []VaultItem{{Slot: 3, Template: 100, Amount: 1000}}}
	b := Vault{Slots: 8, Items: []VaultItem{{Slot: 1, Template: 100, Amount: 600}}}
	r := protocol.ItemMoveRequest{SourceList: 2, SourceSlot: 3, SourceItem: 100, DestinationList: 45, DestinationSlot: 1, DestinationItem: 100, Count: 900}
	a, b, moved, err := MoveVaultCross(a, b, 1000, r)
	if err != nil || moved != 400 || a.ItemAt(3).Amount != 600 || b.ItemAt(1).Amount != 1000 {
		t.Fatalf("merge: %v %d %+v %+v", err, moved, a, b)
	}
	r.DestinationSlot = 4
	r.DestinationItem = 0
	r.Count = 0
	a, b, moved, err = MoveVaultCross(a, b, 500, r)
	if err != nil || moved != 500 || a.ItemAt(3).Amount != 100 || b.ItemAt(4).Amount != 500 {
		t.Fatalf("strict slot: %v %d %+v %+v", err, moved, a, b)
	}
	if b.ItemAt(0) != nil {
		t.Fatal("moved to an unrequested slot")
	}
}

func TestSortVaultSpaceKeepsEquipmentAndCompressesSlots(t *testing.T) {
	v := Vault{Slots: 8, Items: []VaultItem{{Slot: 6, Template: 200, Amount: 1}, {Slot: 4, Template: 100, Amount: 2}, {Slot: 2, Template: 100, Amount: 1}}}
	sorted := SortVaultSpace(v)
	if sorted.Items[0].Slot != 0 || sorted.Items[0].Amount != 1 || sorted.Items[1].Slot != 1 || sorted.Items[1].Amount != 2 || sorted.Items[2].Slot != 2 || v.Items[0].Slot != 6 {
		t.Fatalf("sort=%+v original=%+v", sorted.Items, v.Items)
	}
}
