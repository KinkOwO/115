package inventory

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func avatarCloneFixture(t *testing.T) (*WearService, Role) {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadWearRules("../../configs/equipment-wear.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	bagRules, e := LoadBagRules("../../configs/inventory.next29.json")
	if e != nil {
		t.Fatal(e)
	}

	rules.Special = true
	rules.Slots["[hair avatar]"] = 1

	for _, item := range []struct {
		id    uint32
		clone bool
	}{{517560000, true}, {517560001, true}, {517562678, false}, {517562666, false}} {
		fields := map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[hair avatar]"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
			"[minimum level]":  {{Type: 0, Value: 1}},
		}
		if item.clone {
			fields["[item category]"] = []pvf.Token{{Type: 6, Text: "clear avatar"}}
		}
		eq.index[item.id] = EquipmentDefinition{ID: item.id, Path: "fixture.equ", SHA256: strings.Repeat("1", 64), Fields: fields}
	}

	b := Bag{
		Version: "ordinary-bag-v1",
		Special: map[byte][]BagEquipment{
			1: {
				{Slot: 0, Template: 517560000}, // clone 1
				{Slot: 1, Template: 517562678}, // look 1
				{Slot: 2, Template: 517562666}, // look 2
				{Slot: 3, Template: 517560001}, // clone 2
			},
		},
	}
	raw, e := SaveBag(json.RawMessage(`{"level":10,"advancement":0}`), b)
	if e != nil {
		t.Fatal(e)
	}
	return &WearService{Catalog: eq, Professions: c, BagRules: bagRules, Rules: rules}, Role{Profession: 0, ConfigVersion: c.Source.SaveIdentity(), State: raw}
}

// These expectations replace the obsolete double-Worn contract: the current
// reader and request writer require the ordinary source to remain in list1.
func cloneMove(t *testing.T, s *WearService, role *Role, r protocol.ItemMoveRequest) Bag {
	t.Helper()
	r.Count = 1
	r.Selection = 0xffffffff
	raw, err := s.MoveOrdinary(*role, r)
	if err != nil {
		t.Fatal(err)
	}
	role.State = raw
	b, err := ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func findAvatar(b Bag, slot uint16) BagEquipment {
	for _, row := range b.Special[1] {
		if row.Slot == slot {
			return row
		}
	}
	return BagEquipment{}
}
func TestNativeCloneSourceRemainsPhysicalBagInstance(t *testing.T) {
	s, role := avatarCloneFixture(t)
	b := cloneMove(t, s, &role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 0, SourceItem: 517560000, DestinationList: 3, DestinationSlot: 1})
	if len(b.Worn) != 1 || b.Worn[0].CloneSource != nil {
		t.Fatal("single Clone invented a source")
	}
	before := findAvatar(b, 1)
	b = cloneMove(t, s, &role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 1, SourceItem: 517562678, DestinationList: 3, DestinationSlot: 1, DestinationItem: 517560000})
	if len(b.Worn) != 1 || b.Worn[0].Template != 517560000 || b.CloneAvatarLook(b.Worn[0]) != 517562678 {
		t.Fatal("binding replaced Clone instead of its relationship")
	}
	if !reflect.DeepEqual(before, findAvatar(b, 1)) {
		t.Fatal("binding consumed or rewrote the source instance")
	}
	b = cloneMove(t, s, &role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 2, SourceItem: 517562666, DestinationList: 3, DestinationSlot: 1, DestinationItem: 517560000})
	if b.CloneAvatarLook(b.Worn[0]) != 517562666 || findAvatar(b, 1).Template != 517562678 || findAvatar(b, 2).Template != 517562666 {
		t.Fatal("rebinding moved either look")
	}
	// Swapping two real sources must follow the selected physical row.
	b = cloneMove(t, s, &role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 2, SourceItem: 517562666, DestinationList: 1, DestinationSlot: 1, DestinationItem: 517562678})
	if b.Worn[0].CloneSource.Slot != 1 || b.CloneAvatarLook(b.Worn[0]) != 517562666 {
		t.Fatal("source did not follow bag swap")
	}
	// Static current writer revocation: source identity zero although source exists.
	r := protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 1, DestinationList: 3, DestinationSlot: 1, DestinationItem: 517562666, Flags: [3]byte{0, 0, 1}}
	if CloneMoveAckMode(role.State, r) != 1 {
		t.Fatal("revocation must use native mode1")
	}
	before = findAvatar(b, 1)
	b = cloneMove(t, s, &role, r)
	if len(b.Worn) != 1 || b.Worn[0].Template != 517560000 || b.Worn[0].CloneSource != nil || !reflect.DeepEqual(before, findAvatar(b, 1)) {
		t.Fatal("revocation moved an instance")
	}
	if _, err := s.MoveOrdinary(role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 1, DestinationList: 3, DestinationSlot: 1, DestinationItem: 517562666, Count: 1, Selection: 0xffffffff, Flags: [3]byte{0, 0, 1}}); err == nil {
		t.Fatal("stale revocation succeeded after relationship cleared")
	}
}
func TestNativeCloneOverWornLookAndPhysicalRemoval(t *testing.T) {
	s, role := avatarCloneFixture(t)
	b := cloneMove(t, s, &role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 1, SourceItem: 517562678, DestinationList: 3, DestinationSlot: 1})
	look := b.Worn[0]
	b = cloneMove(t, s, &role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 0, SourceItem: 517560000, DestinationList: 3, DestinationSlot: 1, DestinationItem: 517562678})
	look.Slot = 0
	look.Group = 0
	if len(b.Worn) != 1 || b.CloneAvatarLook(b.Worn[0]) != 517562678 || !reflect.DeepEqual(look, findAvatar(b, 0)) {
		t.Fatal("Clone over look must swap physical source back to bag")
	}
	b = cloneMove(t, s, &role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 4, DestinationList: 3, DestinationSlot: 1, DestinationItem: 517560000})
	if len(b.Worn) != 0 || findAvatar(b, 4).CloneSource != nil || !reflect.DeepEqual(look, findAvatar(b, 0)) {
		t.Fatal("physical removal must not move or consume source")
	}
	b = cloneMove(t, s, &role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 4, SourceItem: 517560000, DestinationList: 3, DestinationSlot: 1})
	if b.CloneAvatarLook(b.Worn[0]) != 0 {
		t.Fatal("re-equipped Clone resurrected removed relationship")
	}
}
func TestNativeCloneLegacyMigrationLosslessAtomicAndIdempotent(t *testing.T) {
	s, role := avatarCloneFixture(t)
	record := make([]byte, protocol.CurrentItemRecordSize)
	for i := range record {
		record[i] = byte(i)
	}
	binary.LittleEndian.PutUint32(record[2:], 517562678)
	look := BagEquipment{Slot: 1, Template: 517562678, Group: 1, Record: record, AvatarOptions: []byte{1, 2, 3}, AvatarSockets: []byte{4, 5, 6}, Period: 123, Refine: 3, Durability: 29}
	old := Bag{Version: "ordinary-bag-v1", Worn: []BagEquipment{{Slot: 1, Template: 517560000}, look}, Special: map[byte][]BagEquipment{1: {{Slot: 0, Template: 517562666}}}}
	snapshot, _ := json.Marshal(old)
	migrated, changed, err := s.NormalizeCloneAvatars(old)
	if err != nil || !changed {
		t.Fatalf("migration: %t %v", changed, err)
	}
	unchanged, _ := json.Marshal(old)
	if !bytes.Equal(snapshot, unchanged) {
		t.Fatal("migration mutated its input")
	}
	want := look
	want.Group = 0
	if len(migrated.Worn) != 1 || migrated.Worn[0].CloneSource.Slot != 1 || !reflect.DeepEqual(findAvatar(migrated, 1), want) {
		t.Fatal("migration lost instance bytes or overwrote occupied slot")
	}
	again, changed, err := s.NormalizeCloneAvatars(migrated)
	if err != nil || changed || !reflect.DeepEqual(migrated, again) {
		t.Fatal("migration not idempotent")
	}
	role.State = json.RawMessage(`{"level":10,"advancement":0,"future":{"keep":true},"inventory":{"version":"ordinary-bag-v1","future_inventory":{"keep":true},"weapon_skin":123}}`)
	migrated.WeaponSkin = 0
	raw, err := SaveBag(role.State, migrated)
	if err != nil || !bytes.Contains(raw, []byte(`"future_inventory":{"keep":true}`)) || !bytes.Contains(raw, []byte(`"future":{"keep":true}`)) || bytes.Contains(raw, []byte(`"weapon_skin"`)) {
		t.Fatal("migration lost unknown fields or resurrected a cleared known field")
	}
	// Capacity pressure is a refusal, never a destructive partial migration.
	full := old
	full.Special = copySpecialEquipment(old.Special)
	for slot := uint16(1); slot < protocol.AvatarInventorySlots(0); slot++ {
		full.Special[1] = append(full.Special[1], BagEquipment{Slot: slot, Template: 517562666})
	}
	before, _ := json.Marshal(full)
	result, changed, err := s.NormalizeCloneAvatars(full)
	after, _ := json.Marshal(result)
	if err == nil || changed || !bytes.Equal(before, after) {
		t.Fatal("full bag migration was not refused atomically")
	}
	bad := old
	bad.Worn = append([]BagEquipment(nil), old.Worn...)
	bad.Worn[1].Template = 517560001
	if _, _, err := s.NormalizeCloneAvatars(bad); err == nil {
		t.Fatal("migration accepted Clone as ordinary source")
	}
}
func TestNativeCloneBindingRejectsWrongPartAndNeverGuessesCategory(t *testing.T) {
	s, role := avatarCloneFixture(t)
	cloneMove(t, s, &role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 0, SourceItem: 517560000, DestinationList: 3, DestinationSlot: 1})
	changed := s.Catalog.index[517562678]
	changed.Fields = map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[coat avatar]"}}, "[usable job]": {{Type: 6, Text: "[all]"}}}
	s.Catalog.index[517562678] = changed
	snapshot := append([]byte(nil), role.State...)
	if _, err := s.MoveOrdinary(role, protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 1, SourceItem: 517562678, DestinationList: 3, DestinationSlot: 1, DestinationItem: 517560000, Count: 1, Selection: 0xffffffff}); err == nil {
		t.Fatal("source part change did not alter binding eligibility")
	}
	if !bytes.Equal(role.State, snapshot) {
		t.Fatal("rejected binding altered input")
	}
}
