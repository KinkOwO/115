package character

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

// A merged glow fix must preserve owned weapon/title looks and the original save.
func TestMergedGlowKeepsOwnedSkinsAndRecord(t *testing.T) {
	weapon := protocol.OrdinaryItem(12, 999, 1)
	weapon[10] = 0x80 | 16
	title := protocol.OrdinaryItem(13, 101, 1)
	title[10] = 7
	bag := inventory.Bag{Version: "ordinary-bag-v1", WeaponSkin: 888, TitleSkin: 102,
		TitleBook: map[byte][]inventory.TitleBookEntry{4: {{Equipment: inventory.BagEquipment{Slot: 10, Template: 102}}}},
		Worn:      []inventory.BagEquipment{{Slot: 12, Template: 999, Record: weapon[:]}, {Slot: 13, Template: 101, Record: title[:]}}}
	raw, e := inventory.SaveBag(json.RawMessage(`{"level":110,"unrelated":{"keep":7}}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	before := append([]byte(nil), raw...)
	look, e := wornAppearance(raw)
	if e != nil || len(look) != 2 {
		t.Fatal(look, e)
	}
	if look[0].Item != 888 || look[0].UpgradeLevel != 16 || look[1].Item != 102 || look[1].UpgradeLevel != 7 {
		t.Fatalf("list projection lost skins/glow: %+v", look)
	}
	rows, e := (&Service{}).wornAppearance(raw)
	if e != nil || len(rows) != 2 {
		t.Fatal(rows, e)
	}
	if rows[0].Model != 888 || rows[0].Flags != 32 || rows[1].Model != 102 || rows[1].Flags != 14 {
		t.Fatalf("refresh projection lost skins/glow: %+v", rows)
	}
	if !bytes.Equal(raw, before) {
		t.Fatal("projection rewrote save")
	}
	restored, e := inventory.ReadBag(raw)
	if e != nil || !bytes.Equal(restored.Worn[0].Record, weapon[:]) {
		t.Fatal("instance changed", e)
	}
}
