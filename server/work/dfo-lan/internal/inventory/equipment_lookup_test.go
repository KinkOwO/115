package inventory

import "testing"

func TestEquipmentLookupPrefersContainerAndChecksIdentity(t *testing.T) {
	bag := Bag{
		Equipment: []BagEquipment{{Slot: 7, Template: 100}, {Slot: 8, Template: 200}},
		Worn:      []BagEquipment{{Slot: 7, Template: 100}, {Slot: 7, Template: 300}},
	}
	for _, tc := range []struct {
		space     byte
		slot      uint16
		template  uint32
		wantSpace byte
		wantIndex int
	}{
		{0, 7, 100, 0, 0},
		{3, 7, 100, 3, 0},
		{0, 7, 300, 3, 1},
		{3, 8, 200, 0, 1},
		{0, 7, 999, 3, -1},
	} {
		space, items, index := bag.findEquipment(tc.space, tc.slot, tc.template)
		if space != tc.wantSpace || index != tc.wantIndex {
			t.Errorf("lookup %+v: space=%d index=%d", tc, space, index)
		}
		if index >= 0 && (items[index].Slot != tc.slot || items[index].Template != tc.template) {
			t.Fatal("lookup returned a different equipment instance")
		}
	}
}
