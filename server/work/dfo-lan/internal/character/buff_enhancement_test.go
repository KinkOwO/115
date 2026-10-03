package character

import (
	"bytes"

	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"

	"encoding/json"

	"os"
	"path/filepath"

	"testing"
)

func buffFixture(t *testing.T) (*Service, Character) {
	t.Helper()
	const sum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	catalog := inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: []inventory.EquipmentDefinition{
		{ID: 100, Path: "title.equ", SHA256: sum, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[title name]"}}}},
		{ID: 101, Path: "other-title.equ", SHA256: sum, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[title name]"}}}},
	}}
	raw, _ := json.Marshal(catalog)
	path := filepath.Join(t.TempDir(), "equipment.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	equipment, err := inventory.LoadEquipmentCatalog(path, sum)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Equipment: equipment, WearRules: inventory.WearRules{Slots: map[string]uint16{"[title name]": 13}}}
	state := json.RawMessage(`{"level":10,"learned_skills":[{"306":1},null],"future":{"keep":true},"quest_state":{"keep":1}}`)
	state, err = inventory.SaveBag(state, inventory.Bag{Version: "ordinary-bag-v1", Gold: 999, Equipment: []inventory.BagEquipment{{Slot: 57, Template: 100}}})
	if err != nil {
		t.Fatal(err)
	}
	return s, Character{AccountID: 1, ID: 1, Profession: 9, ConfigVersion: sum, State: state}
}

func TestBuffEnhancementPersistenceAndIdentity(t *testing.T) {
	s, role := buffFixture(t)
	original := append([]byte(nil), role.State...)
	raw, err := s.applyBuffEnhancement(role, protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 57})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(role.State, original) {
		t.Fatal("apply mutated source state")
	}
	role.State = raw
	got, err := s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatalf("restore %x %v", got, err)
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	if string(fields["future"]) != `{"keep":true}` || string(fields["quest_state"]) != `{"keep":1}` {
		t.Fatal("unrelated state lost")
	}
	var state State
	json.Unmarshal(raw, &state)
	merged, err := mergeSkillState(raw, state)
	if err != nil {
		t.Fatal(err)
	}
	role.State = merged
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatal("skill merge lost registration")
	}
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		t.Fatal(err)
	}
	b.Equipment[0].Slot = 80
	role.State, err = inventory.SaveBag(role.State, b)
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 80, 0}) {
		t.Fatalf("move did not follow owned identity: %x %v", got, err)
	}
	b.Equipment[0].Template = 101
	role.State, _ = inventory.SaveBag(role.State, b)
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 0}) {
		t.Fatal("reused slot incorrectly restored")
	}
	b.Equipment = []inventory.BagEquipment{{Slot: 80, Template: 100}, {Slot: 81, Template: 100}}
	role.State, _ = inventory.SaveBag(role.State, b)
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 0}) {
		t.Fatal("ambiguous duplicate restored")
	}
}

func TestBuffEnhancementSelectionAndClear(t *testing.T) {
	s, role := buffFixture(t)
	got, err := s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0, 0, 0}) {
		t.Fatal("legacy save not cleared")
	}
	for _, req := range []protocol.BuffEnhancementRequest{{Skill: 306, Kind: 13, List: 0, Slot: 57}, {Skill: 0, Kind: 48, List: 46, Slot: 65535}, {Skill: 306, Kind: 48, List: 46, Slot: 65535}} {
		role.State, err = s.applyBuffEnhancement(role, req)
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatal("skill selection discarded items")
	}
	role.State, err = s.applyBuffEnhancement(role, protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 46, Slot: 65535})
	if err != nil {
		t.Fatal(err)
	}
	got, err = s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 0}) {
		t.Fatal("clear registration failed")
	}
	for _, req := range []protocol.BuffEnhancementRequest{{Skill: 999, Kind: 13, List: 0, Slot: 57}, {Skill: 306, Kind: 12, List: 0, Slot: 57}, {Skill: 306, Kind: 13, List: 0, Slot: 58}} {
		if _, err = s.applyBuffEnhancement(role, req); err == nil {
			t.Fatalf("accepted invalid %+v", req)
		}
	}
}

// Optional real PostgreSQL exercise, confined to a new temporary schema.
