package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestTagEquipmentPreservesStoredAvatarLook(t *testing.T) {
	r := OrdinaryItem(1, 769900045, 0)
	binary.LittleEndian.PutUint32(r[24:], 760050711)
	r[10], r[18], r[19] = 12, 2, 3
	binary.LittleEndian.PutUint32(r[14:], 12345)
	before := bytes.Clone(r[:])
	item := DetailedWorn{Slot: 1, Template: 769900045, Record: r[:], AvatarOptions: []byte{1, 2}, AvatarSockets: []byte{3, 4}}
	p, err := TagEquipment([]DetailedWorn{item})
	if err != nil {
		t.Fatal(err)
	}
	// Pin the existing native 40-byte header: its +24 attachment is distinct
	// from the 181-byte inventory row's +24 cell, even though values agree.
	if binary.LittleEndian.Uint32(p[1+24:]) != 760050711 || p[1+9] != 12 ||
		binary.LittleEndian.Uint32(p[1+32:]) != 12345 || p[1+36] != 2 || p[1+37] != 3 ||
		!bytes.Equal(p[1+40:1+52], []byte{2, 0, 0, 0, 1, 2, 2, 0, 0, 0, 3, 4}) {
		t.Fatal("avatar attachment, enhancements or extension framing changed")
	}
	if !bytes.Equal(before, r[:]) {
		t.Fatal("projection rewrote the stored avatar instance")
	}
	item.HeaderTemplateA = 760050711
	if matched, err := TagEquipment([]DetailedWorn{item}); err != nil || !bytes.Equal(matched, p) {
		t.Fatal("resolved and persisted matching attachments differ", err)
	}
	item.HeaderTemplateA++
	resolved, err := TagEquipment([]DetailedWorn{item})
	if err != nil || binary.LittleEndian.Uint32(resolved[1+24:]) != item.HeaderTemplateA || !bytes.Equal(before, r[:]) {
		t.Fatal("stale cached look overrode current owned clone relationship", err)
	}
}

func TestTagEquipmentPreservesPetInstanceKey(t *testing.T) {
	for _, slot := range []uint16{26, 32} {
		r := OrdinaryItem(slot, 500991107, 7)
		binary.LittleEndian.PutUint32(r[24:], 7)
		before := bytes.Clone(r[:])
		p, err := TagEquipment([]DetailedWorn{{Slot: slot, Template: 500991107, Record: r[:]}})
		if err != nil || binary.LittleEndian.Uint32(p[1+5:]) != 7 || binary.LittleEndian.Uint32(p[1+24:]) != 0 || !bytes.Equal(before, r[:]) {
			t.Fatal("pet key was dropped, misread as a look or persisted differently", slot, err)
		}
		binary.LittleEndian.PutUint32(r[24:], 8)
		if _, err := TagEquipment([]DetailedWorn{{Slot: slot, Template: 500991107, Record: r[:]}}); err == nil {
			t.Fatal("conflicting pet key accepted", slot)
		}
	}
}

func TestTagEquipmentAcceptsCurrentWornSlotsAndKeepsUnknownGuards(t *testing.T) {
	for _, slot := range []uint16{30, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47} {
		p, err := TagEquipment([]DetailedWorn{{Slot: slot, Template: 100313757}})
		if err != nil || p[0] != 1 || p[1] != byte(slot) {
			t.Fatal("native 48-slot table truncated", slot, err)
		}
	}
	if _, err := TagEquipment([]DetailedWorn{{Slot: 48, Template: 100313757}}); err == nil {
		t.Fatal("out-of-table slot accepted")
	}
	for _, offset := range []int{13, 22, 23, 28, 55, 60, 75, 82, 99, 102} {
		r := OrdinaryItem(1, 769900045, 0)
		r[offset] = 1
		if _, err := TagEquipment([]DetailedWorn{{Slot: 1, Template: 769900045, Record: r[:]}}); err == nil {
			t.Fatal("unsupported independent instance byte discarded", offset)
		}
	}
	r := OrdinaryItem(12, 101000013, 0)
	r[24] = 1
	if _, err := TagEquipment([]DetailedWorn{{Slot: 12, Template: 101000013, Record: r[:]}}); err == nil {
		t.Fatal("non-avatar/non-pet attachment accepted without native semantics")
	}
}

// Optional local audit consumes a read-only export of actual worn rows, never
// opens the player database, and is not a shipped gameplay content table.
func TestTagEquipmentLocalSavedInstances(t *testing.T) {
	path := os.Getenv("DFO_TAG_EQUIPMENT_TEST_FIXTURE")
	if path == "" {
		t.Skip("set DFO_TAG_EQUIPMENT_TEST_FIXTURE for local saved-equipment audit")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var roles []struct {
		Name string         `json:"name"`
		Worn []DetailedWorn `json:"worn"`
	}
	if err := json.Unmarshal(raw, &roles); err != nil || len(roles) == 0 {
		t.Fatal("missing local audit rows", err)
	}
	for _, role := range roles {
		before, _ := json.Marshal(role.Worn)
		p, err := TagEquipment(role.Worn)
		if err != nil {
			t.Fatalf("%s: %v", role.Name, err)
		}
		after, _ := json.Marshal(role.Worn)
		if !bytes.Equal(before, after) || int(p[0]) != len(role.Worn) {
			t.Fatal("local audit lost or modified an equipment row", role.Name)
		}
		t.Logf("%s: %d worn instances encoded without changing records", role.Name, len(role.Worn))
	}
}
