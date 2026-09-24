package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A character must be able to wear any structurally valid piece it meets the
// level/job/grow requirement for, regardless of the drop-pool rules ([free]
// attach and a rarity ceiling). Those rules decide what a monster may DROP,
// not what a character may WEAR. Before the fix the equip path called Basic and
// refused a [trade delete] bound shoe (e.g. the Explorer-collection 100261068)
// and every rarity-2 uncommon the archer otherwise qualified for - the user's
// "my own gear cannot be equipped" report. This locks the wear gate to the
// structural Reward check plus slot fit plus WearableBy.
func TestWearAcceptsBoundAndUncommonGear(t *testing.T) {
	const sum = "1111111111111111111111111111111111111111111111111111111111111111"
	shoe := func(id uint32, attach string, rarity int32) EquipmentDefinition {
		return EquipmentDefinition{ID: id, Path: "equipment/x.equ", SHA256: sum, Fields: map[string][]pvf.Token{
			"[attach type]":    {{Type: 6, Text: attach}},
			"[rarity]":         {{Type: 0, Value: rarity}},
			"[equipment type]": {{Type: 6, Text: "[shoes]"}},
			"[minimum level]":  {{Type: 0, Value: 1}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
			"[durability]":     {{Type: 0, Value: 40}},
			"[grade]":          {{Type: 0, Value: 1}},
		}}
	}
	cat := EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: []EquipmentDefinition{
		shoe(5001, "[free]", 0),         // control: drop-pool eligible
		shoe(5002, "[trade delete]", 0), // bound: Reward yes, Basic no
		shoe(5003, "[free]", 2),         // uncommon: Reward yes, Basic no
	}}
	dir := t.TempDir()
	path := filepath.Join(dir, "equipment.json")
	b, e := json.Marshal(cat)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0o600); e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog(path, sum)
	if e != nil {
		t.Fatal(e)
	}
	// Bound gear stays out of the drop pool. An uncommon (rarity 2) piece is
	// droppable: the source rarity roll names rarity 0/1/2 and everything the
	// catalog carries above grade 20 is rarity>=1, so excluding 2 left every
	// dungeon above level 21 with no gear to drop at all.
	for _, d := range eq.DropPool() {
		if d.ID == 5002 {
			t.Fatalf("drop pool leaked bound gear %d", d.ID)
		}
	}
	uncommonDroppable := false
	for _, d := range eq.DropPool() {
		if d.ID == 5003 {
			uncommonDroppable = true
		}
	}
	if !uncommonDroppable {
		t.Fatal("rarity 2 gear is no longer droppable; high-level dungeons lose their whole gear pool")
	}
	svc := &WearService{
		Catalog:     eq,
		Professions: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: sum}, Professions: map[byte]catalog.Profession{16: {Job: "[archer]"}}},
		BagRules:    BagRules{EquipmentSlots: [2]uint16{9, 44}},
		Rules:       WearRules{Source: sum, Slots: map[string]uint16{"[shoes]": 19}},
	}
	for _, id := range []uint32{5001, 5002, 5003} {
		bag := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: id}}}
		state, e := SaveBag(json.RawMessage(`{"level":1,"advancement":0}`), bag)
		if e != nil {
			t.Fatal(e)
		}
		role := storage.Character{Profession: 16, ConfigVersion: sum, State: state}
		r := protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: id, DestinationList: 3, DestinationSlot: 19, Count: 1, Selection: 0xffffffff}
		raw, e := svc.MoveOrdinary(role, r)
		if e != nil {
			t.Fatalf("archer could not equip template %d: %v", id, e)
		}
		worn, e := ReadBag(raw)
		if e != nil || len(worn.Worn) != 1 || worn.Worn[0].Template != id || worn.Worn[0].Slot != 19 {
			t.Fatalf("template %d not worn: %+v %v", id, worn, e)
		}
	}
}
