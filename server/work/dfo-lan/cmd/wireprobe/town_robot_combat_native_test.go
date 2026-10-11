package main

import (
	"bytes"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Read-only copy of the current synthetic archive. No grant, database or live
// connection is opened. Emits only a private native-reader test vector.
func TestTownRobotCombatFullEntryNativeFixture(t *testing.T) {
	input := os.Getenv("DFO_ROBOT_COMBAT_FIXTURE")
	if input == "" {
		t.Skip("explicit read-only combat equipment fixture")
	}
	if filepath.Base(input) != "r24-combat-input.json" {
		t.Fatal("offline input required")
	}
	original, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	records, err := readTownRobotArchive(input)
	if err != nil {
		t.Fatal(err)
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "characters,skills,items,quests,loot,equipment,equipment-selection,fame", ArchivePath: `D:\115us\server\work\client-build\Script.inner.pvf`, ArchiveChecksum: "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc", CharacterPolicyPath: "../../configs/pvf-character-policy.json", DropPolicyPath: "../../configs/pvf-drop-policy.json", DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	c.Selection.Full = c.Equipment
	s := &character.Service{Catalog: *c.Characters, Learning: c.Learning, Equipment: c.Selection, DetailedWornCandidate: true}
	s.WearRules, err = inventory.LoadWearRules("../../configs/equipment-wear.full-candidate.json", c.SourceChecksum)
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.InstallFameRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	role := records[1].Role
	role.WireID = 65534
	snapshot, err := s.TownRobotCompanionSnapshot(role)
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := s.EntryAddition(role)
	if err != nil {
		t.Fatal(err)
	}
	full, err := s.TownRobotEntryAddition(role)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := protocol.TagEquipment(snapshot.Worn)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(full[361:361+len(encoded)], encoded) || bytes.Equal(full, ordinary) {
		t.Fatal("full combat rows missing")
	}
	types := map[uint32]int{}
	slots := map[uint16]bool{}
	for _, item := range snapshot.Worn {
		slots[item.Slot] = true
		kind, err := c.Selection.EquipmentKind(item.Template)
		if err != nil {
			t.Fatal(err)
		}
		slot, ok := s.WearRules.Slots[kind]
		if !ok {
			t.Fatal("unmapped kind", kind)
		}
		types[item.Template] = int(slot)
	}
	for _, slot := range []uint16{12, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 25, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47} {
		if !slots[slot] {
			t.Fatal("fixture missing equipment", slot)
		}
	}
	wrong := role
	wrong.AccountID = 1
	if _, err = s.TownRobotEntryAddition(wrong); err == nil {
		t.Fatal("real account accepted synthetic projection")
	}
	unchanged, err := s.EntryAddition(role)
	if err != nil || !bytes.Equal(ordinary, unchanged) {
		t.Fatal("ordinary projection changed", err)
	}
	disk, err := os.ReadFile(input)
	if err != nil || !bytes.Equal(original, disk) {
		t.Fatal("fixture mutated")
	}
	vector, _ := json.MarshalIndent(map[string]any{"payload_hex": fmt.Sprintf("%x", full), "ordinary_hex": fmt.Sprintf("%x", ordinary), "types": types, "slots": slots, "source_sha256": c.SourceChecksum}, "", "  ")
	if err = os.WriteFile(filepath.Join(filepath.Dir(input), "r24-entry-vector.json"), vector, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("full combat worn=%d; 12 normal + 12 oath slots; state preserved", len(slots))
}
