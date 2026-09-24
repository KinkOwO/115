package inventory

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestHandoffWornDisplayUpdate(t *testing.T) {
	raw, e := SaveBag(json.RawMessage(`{"level":6}`), Bag{Version: "ordinary-bag-v1", Gold: 345, Worn: []BagEquipment{{Slot: 19, Template: 20002}}})
	if e != nil {
		t.Fatal(e)
	}
	original := append([]byte(nil), raw...)
	p, e := WornSpaceUpdate(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 3+protocol.CurrentItemRecordSize || p[0] != 3 || binary.LittleEndian.Uint16(p[1:]) != 1 {
		t.Fatal("wrong NOTI14 worn layout")
	}
	row := EquipmentRow(BagEquipment{Slot: 19, Template: 20002})
	if !bytes.Equal(p[3:], row[:]) {
		t.Fatal("equipment row changed")
	}
	if !bytes.Equal(original, raw) {
		t.Fatal("display refresh changed state")
	}
	empty, e := WornSpaceUpdate(json.RawMessage(`{}`))
	if e != nil || len(empty) != 0 {
		t.Fatal("empty equipment should not trigger rebuild")
	}
}

func TestHasWornWeaponUsesWornSlot12(t *testing.T) {
	for _, tc := range []struct {
		name string
		bag  Bag
		want bool
	}{
		{"worn weapon", Bag{Worn: []BagEquipment{{Slot: 12, Template: 101000013}}}, true},
		{"bag slot 12 only", Bag{Equipment: []BagEquipment{{Slot: 12, Template: 101000013}}}, false},
		{"other worn slot", Bag{Worn: []BagEquipment{{Slot: 19, Template: 20002}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.bag.Version = "ordinary-bag-v1"
			state, err := SaveBag(json.RawMessage(`{}`), tc.bag)
			if err != nil {
				t.Fatal(err)
			}
			if got := HasWornWeapon(state); got != tc.want {
				t.Fatalf("HasWornWeapon=%v, want %v", got, tc.want)
			}
		})
	}
}
