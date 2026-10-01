package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
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

	// Clone hair avatar 1
	eq.index[517560000] = EquipmentDefinition{
		ID:     517560000,
		Path:   "equipment/avatar/archer/hair_clone1.equ",
		SHA256: strings.Repeat("1", 64),
		Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[hair avatar]"}},
			"[item category]":  {{Type: 6, Text: "clear avatar"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
			"[minimum level]":  {{Type: 0, Value: 1}},
		},
	}
	// Clone hair avatar 2
	eq.index[517560001] = EquipmentDefinition{
		ID:     517560001,
		Path:   "equipment/avatar/archer/hair_clone2.equ",
		SHA256: strings.Repeat("2", 64),
		Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[hair avatar]"}},
			"[item category]":  {{Type: 6, Text: "clear avatar"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
			"[minimum level]":  {{Type: 0, Value: 1}},
		},
	}
	// Appearance hair avatar 1
	eq.index[517562678] = EquipmentDefinition{
		ID:     517562678,
		Path:   "equipment/avatar/archer/hair_look1.equ",
		SHA256: strings.Repeat("3", 64),
		Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[hair avatar]"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
			"[minimum level]":  {{Type: 0, Value: 1}},
		},
	}
	// Appearance hair avatar 2
	eq.index[517562666] = EquipmentDefinition{
		ID:     517562666,
		Path:   "equipment/avatar/archer/hair_look2.equ",
		SHA256: strings.Repeat("4", 64),
		Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[hair avatar]"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
			"[minimum level]":  {{Type: 0, Value: 1}},
		},
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

func TestAvatarCloneAndAppearanceCoexistence(t *testing.T) {
	s, role := avatarCloneFixture(t)

	// Step 1: Equip Clone Avatar 1 (slot 0 in bag -> slot 1 worn)
	// Flags: [0, 0, 0] (Clone avatar tab)
	reqClone := protocol.ItemMoveRequest{
		SourceList:      1,
		SourceSlot:      0,
		SourceItem:      517560000,
		DestinationList: 3,
		DestinationSlot: 1,
		DestinationItem: 0,
		Count:           1,
		Selection:       0xffffffff,
		Flags:           [3]byte{0, 0, 0},
	}
	raw, err := s.MoveOrdinary(role, reqClone)
	if err != nil {
		t.Fatalf("equip clone avatar failed: %v", err)
	}
	role.State = raw

	b, err := ReadBag(raw)
	if err != nil {
		t.Fatalf("ReadBag failed: %v", err)
	}
	if len(b.Worn) != 1 {
		t.Fatalf("expected 1 worn item, got %d", len(b.Worn))
	}
	if b.Worn[0].Slot != 1 || b.Worn[0].Template != 517560000 || b.Worn[0].Group != 0 {
		t.Fatalf("unexpected worn clone avatar: %+v", b.Worn[0])
	}

	// Step 2: Equip Appearance Avatar 1 (slot 1 in bag -> slot 1 worn)
	// Flags: [0, 0, 1] (Appearance avatar tab)
	reqLook := protocol.ItemMoveRequest{
		SourceList:      1,
		SourceSlot:      1,
		SourceItem:      517562678,
		DestinationList: 3,
		DestinationSlot: 1,
		DestinationItem: 0,
		Count:           1,
		Selection:       0xffffffff,
		Flags:           [3]byte{0, 0, 1},
	}
	raw, err = s.MoveOrdinary(role, reqLook)
	if err != nil {
		t.Fatalf("equip appearance avatar failed: %v", err)
	}
	role.State = raw

	b, err = ReadBag(raw)
	if err != nil {
		t.Fatalf("ReadBag failed: %v", err)
	}
	// Crucial check: BOTH avatars must coexist in b.Worn!
	// Clone avatar MUST NOT be kicked off!
	if len(b.Worn) != 2 {
		t.Fatalf("expected 2 worn items (clone + appearance), got %d: %+v", len(b.Worn), b.Worn)
	}

	var hasClone, hasLook bool
	for _, w := range b.Worn {
		if w.Slot == 1 && w.Template == 517560000 && w.Group == 0 {
			hasClone = true
		}
		if w.Slot == 1 && w.Template == 517562678 && w.Group == 1 {
			hasLook = true
		}
	}
	if !hasClone || !hasLook {
		t.Fatalf("expected both clone and look avatars worn: %+v", b.Worn)
	}

	// Verify WornBaseItems produces exactly 1 item for NOTI 13/14 (preferring clone)
	base := b.WornBaseItems()
	if len(base) != 1 || base[0].Slot != 1 || base[0].Template != 517560000 {
		t.Fatalf("WornBaseItems should select clone avatar as base, got %+v", base)
	}

	// Step 3: Replace Appearance Avatar (equip look 2 from bag slot 2 -> slot 1 worn)
	// Client sends DestinationItem = 517562678 (old look avatar) and Flags[2] = 1
	reqReplaceLook := protocol.ItemMoveRequest{
		SourceList:      1,
		SourceSlot:      2,
		SourceItem:      517562666,
		DestinationList: 3,
		DestinationSlot: 1,
		DestinationItem: 517562678,
		Count:           1,
		Selection:       0xffffffff,
		Flags:           [3]byte{0, 0, 1},
	}
	raw, err = s.MoveOrdinary(role, reqReplaceLook)
	if err != nil {
		t.Fatalf("replace appearance avatar failed: %v", err)
	}
	role.State = raw

	b, err = ReadBag(raw)
	if err != nil {
		t.Fatalf("ReadBag failed: %v", err)
	}
	if len(b.Worn) != 2 {
		t.Fatalf("expected 2 worn items after look replacement, got %d: %+v", len(b.Worn), b.Worn)
	}
	hasClone = false
	var hasNewLook bool
	for _, w := range b.Worn {
		if w.Slot == 1 && w.Template == 517560000 && w.Group == 0 {
			hasClone = true
		}
		if w.Slot == 1 && w.Template == 517562666 && w.Group == 1 {
			hasNewLook = true
		}
	}
	if !hasClone || !hasNewLook {
		t.Fatalf("look replacement failed or kicked clone avatar: %+v", b.Worn)
	}
	// Old look avatar (517562678) should be back in bag slot 2
	var foundOldInBag bool
	for _, it := range b.Special[1] {
		if it.Slot == 2 && it.Template == 517562678 {
			foundOldInBag = true
		}
	}
	if !foundOldInBag {
		t.Fatalf("old look avatar not returned to bag slot 2: %+v", b.Special[1])
	}

	// Step 4: Replace Clone Avatar (equip clone 2 from bag slot 3 -> slot 1 worn)
	// Client sends DestinationItem = 517560000 (old clone avatar) and Flags[2] = 0
	reqReplaceClone := protocol.ItemMoveRequest{
		SourceList:      1,
		SourceSlot:      3,
		SourceItem:      517560001,
		DestinationList: 3,
		DestinationSlot: 1,
		DestinationItem: 517560000,
		Count:           1,
		Selection:       0xffffffff,
		Flags:           [3]byte{0, 0, 0},
	}
	raw, err = s.MoveOrdinary(role, reqReplaceClone)
	if err != nil {
		t.Fatalf("replace clone avatar failed: %v", err)
	}
	role.State = raw

	b, err = ReadBag(raw)
	if err != nil {
		t.Fatalf("ReadBag failed: %v", err)
	}
	if len(b.Worn) != 2 {
		t.Fatalf("expected 2 worn items after clone replacement, got %d: %+v", len(b.Worn), b.Worn)
	}
	var hasNewClone bool
	hasNewLook = false
	for _, w := range b.Worn {
		if w.Slot == 1 && w.Template == 517560001 && w.Group == 0 {
			hasNewClone = true
		}
		if w.Slot == 1 && w.Template == 517562666 && w.Group == 1 {
			hasNewLook = true
		}
	}
	if !hasNewClone || !hasNewLook {
		t.Fatalf("clone replacement failed or kicked look avatar: %+v", b.Worn)
	}

	// Step 5: Unequip Appearance Avatar (worn slot 1 -> bag slot 5)
	// Flags[2] = 1 (Appearance avatar tab)
	reqUnequipLook := protocol.ItemMoveRequest{
		SourceList:      3,
		SourceSlot:      1,
		SourceItem:      517562666,
		DestinationList: 1,
		DestinationSlot: 5,
		DestinationItem: 0,
		Count:           1,
		Selection:       0xffffffff,
		Flags:           [3]byte{0, 0, 1},
	}
	raw, err = s.MoveOrdinary(role, reqUnequipLook)
	if err != nil {
		t.Fatalf("unequip appearance avatar failed: %v", err)
	}
	role.State = raw

	b, err = ReadBag(raw)
	if err != nil {
		t.Fatalf("ReadBag failed: %v", err)
	}
	// Clone avatar MUST STILL BE EQUIPPED!
	if len(b.Worn) != 1 || b.Worn[0].Slot != 1 || b.Worn[0].Template != 517560001 || b.Worn[0].Group != 0 {
		t.Fatalf("clone avatar was removed when unequipping look avatar: %+v", b.Worn)
	}

	// Step 6: Unequip Clone Avatar (worn slot 1 -> bag slot 6)
	// Flags[2] = 0 (Clone avatar tab)
	reqUnequipClone := protocol.ItemMoveRequest{
		SourceList:      3,
		SourceSlot:      1,
		SourceItem:      517560001,
		DestinationList: 1,
		DestinationSlot: 6,
		DestinationItem: 0,
		Count:           1,
		Selection:       0xffffffff,
		Flags:           [3]byte{0, 0, 0},
	}
	raw, err = s.MoveOrdinary(role, reqUnequipClone)
	if err != nil {
		t.Fatalf("unequip clone avatar failed: %v", err)
	}

	b, err = ReadBag(raw)
	if err != nil {
		t.Fatalf("ReadBag failed: %v", err)
	}
	if len(b.Worn) != 0 {
		t.Fatalf("expected 0 worn items, got %+v", b.Worn)
	}
}

func TestLegacyWornAvatarMigration(t *testing.T) {
	s, role := avatarCloneFixture(t)

	// Simulate existing character in DB where an appearance avatar was worn
	// before the Group field was introduced (saved with no group, so Group == 0).
	legacyBag := Bag{
		Version: "ordinary-bag-v1",
		Worn: []BagEquipment{
			{Slot: 1, Template: 517562678}, // appearance avatar, group = 0
		},
		Special: map[byte][]BagEquipment{
			1: {
				{Slot: 0, Template: 517560000}, // clone avatar in bag
			},
		},
	}
	rawLegacy, err := SaveBag(json.RawMessage(`{"level":10,"advancement":0}`), legacyBag)
	if err != nil {
		t.Fatal(err)
	}
	role.State = rawLegacy

	// Now equip the clone avatar (517560000) onto slot 1.
	// It MUST NOT overwrite or kick off the appearance avatar!
	req := protocol.ItemMoveRequest{
		SourceList:      1,
		SourceSlot:      0,
		SourceItem:      517560000,
		DestinationList: 3,
		DestinationSlot: 1,
		DestinationItem: 0,
		Count:           1,
		Selection:       0xffffffff,
		Flags:           [3]byte{0, 0, 0},
	}
	raw, err := s.MoveOrdinary(role, req)
	if err != nil {
		t.Fatalf("equipping clone over legacy appearance failed: %v", err)
	}

	b, err := ReadBag(raw)
	if err != nil {
		t.Fatalf("ReadBag failed: %v", err)
	}
	if len(b.Worn) != 2 {
		t.Fatalf("expected both clone and migrated appearance avatar to coexist, got %d: %+v", len(b.Worn), b.Worn)
	}
	var hasClone, hasLook bool
	for _, w := range b.Worn {
		if w.Slot == 1 && w.Template == 517560000 && w.Group == 0 {
			hasClone = true
		}
		if w.Slot == 1 && w.Template == 517562678 && w.Group == 1 {
			hasLook = true
		}
	}
	if !hasClone || !hasLook {
		t.Fatalf("legacy migration failed: %+v", b.Worn)
	}
}

// Live 2026-09-22: equipping a clone avatar over a slot whose appearance
// avatar is worn makes the client fill DestinationItem with the DISPLAYED
// (other-group) item - plain_hex 010900ca4d9c1e0100000003010073589c1e with
// flags 000000. The server must treat that as a group-local insert, not a
// stale identity (refusal code 4 surfaces as "inventory is full").
func TestCloneEquipOverAppearanceWithDisplayedDestinationItem(t *testing.T) {
	s, role := avatarCloneFixture(t)

	bag := Bag{
		Version: "ordinary-bag-v1",
		Worn: []BagEquipment{
			{Slot: 1, Template: 517562678, Group: 1}, // appearance avatar worn
		},
		Special: map[byte][]BagEquipment{
			1: {
				{Slot: 0, Template: 517560000}, // clone avatar in bag
			},
		},
	}
	raw0, err := SaveBag(json.RawMessage(`{"level":10,"advancement":0}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role.State = raw0

	req := protocol.ItemMoveRequest{
		SourceList:      1,
		SourceSlot:      0,
		SourceItem:      517560000,
		DestinationList: 3,
		DestinationSlot: 1,
		DestinationItem: 517562678, // displayed other-group item, NOT a replace target
		Count:           1,
		Selection:       0xffffffff,
		Flags:           [3]byte{0, 0, 0},
	}
	raw, err := s.MoveOrdinary(role, req)
	if err != nil {
		t.Fatalf("clone equip over appearance was refused: %v", err)
	}
	b, err := ReadBag(raw)
	if err != nil {
		t.Fatalf("ReadBag failed: %v", err)
	}
	if len(b.Worn) != 2 {
		t.Fatalf("expected coexistence after insert, got %+v", b.Worn)
	}
	var hasClone, hasLook bool
	for _, w := range b.Worn {
		if w.Slot == 1 && w.Template == 517560000 && w.Group == 0 {
			hasClone = true
		}
		if w.Slot == 1 && w.Template == 517562678 && w.Group == 1 {
			hasLook = true
		}
	}
	if !hasClone || !hasLook {
		t.Fatalf("coexistence broken: %+v", b.Worn)
	}
}

// The mirror image: equipping an appearance avatar while DestinationItem
// names the worn clone (other group) must also insert, not refuse.
func TestLookEquipOverCloneWithDisplayedDestinationItem(t *testing.T) {
	s, role := avatarCloneFixture(t)

	bag := Bag{
		Version: "ordinary-bag-v1",
		Worn: []BagEquipment{
			{Slot: 1, Template: 517560000, Group: 0}, // clone avatar worn
		},
		Special: map[byte][]BagEquipment{
			1: {
				{Slot: 0, Template: 517562678}, // appearance avatar in bag
			},
		},
	}
	raw0, err := SaveBag(json.RawMessage(`{"level":10,"advancement":0}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role.State = raw0

	req := protocol.ItemMoveRequest{
		SourceList:      1,
		SourceSlot:      0,
		SourceItem:      517562678,
		DestinationList: 3,
		DestinationSlot: 1,
		DestinationItem: 517560000, // displayed other-group item
		Count:           1,
		Selection:       0xffffffff,
		Flags:           [3]byte{0, 0, 1},
	}
	raw, err := s.MoveOrdinary(role, req)
	if err != nil {
		t.Fatalf("look equip over clone was refused: %v", err)
	}
	b, err := ReadBag(raw)
	if err != nil {
		t.Fatalf("ReadBag failed: %v", err)
	}
	if len(b.Worn) != 2 {
		t.Fatalf("expected coexistence after insert, got %+v", b.Worn)
	}
}

// The client can request an unequip as a swap from an empty bag cell into an
// occupied worn cell. Its flags do not identify the avatar group in this case;
// DestinationItem identifies the worn appearance item.
func TestLookUnequipFromEmptyBagCellUsesDestinationIdentity(t *testing.T) {
	s, role := avatarCloneFixture(t)
	bag := Bag{
		Version: "ordinary-bag-v1",
		Worn: []BagEquipment{
			{Slot: 1, Template: 517560000, Group: 0},
			{Slot: 1, Template: 517562678, Group: 1},
		},
	}
	state, err := SaveBag(json.RawMessage(`{"level":10,"advancement":0}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role.State = state
	raw, err := s.MoveOrdinary(role, protocol.ItemMoveRequest{
		SourceList: 1, SourceSlot: 16, SourceItem: 0,
		DestinationList: 3, DestinationSlot: 1, DestinationItem: 517562678,
		Count: 1, Selection: 0xffffffff, Flags: [3]byte{0, 0, 0},
	})
	if err != nil {
		t.Fatalf("unequip appearance from empty bag cell: %v", err)
	}
	got, err := ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Worn) != 1 || got.Worn[0].Template != 517560000 || got.Worn[0].Group != 0 {
		t.Fatalf("clone avatar should remain worn: %+v", got.Worn)
	}
	if len(got.Special[1]) != 1 || got.Special[1][0].Slot != 16 || got.Special[1][0].Template != 517562678 {
		t.Fatalf("appearance avatar should move into bag slot 16: %+v", got.Special[1])
	}
}
