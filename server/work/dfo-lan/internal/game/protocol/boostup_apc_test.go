package protocol

import (
	"encoding/binary"
	"testing"
)

func TestBoostAPCNativeSelectionOffsets(t *testing.T) {
	p, err := AdventureEliteSelections([]AdventureEliteSelection{{Mode: 3, Slots: [3]int32{-1, -1, -1}, APCIndices: [3]uint32{1}}})
	if err != nil || len(p) != 532 || p[0] != 1 {
		t.Fatal(len(p), err)
	}
	body := p[1:]
	if binary.LittleEndian.Uint16(body[0x0d:]) != 3 || body[0x10] != 1 || binary.LittleEndian.Uint32(body[0x17:]) != 1 || binary.LittleEndian.Uint32(body[0x27:]) != ^uint32(0) {
		t.Fatal("NPC branch does not match142E5A4C0/142E5C590")
	}
	// The account persistence decoder must keep rejecting native NPC input.
	if _, err = DecodeAdventureEliteSelection(body); err == nil {
		t.Fatal("NPC special index could enter account character selections")
	}
	if _, err = AdventureEliteSelections([]AdventureEliteSelection{{Mode: 3, APCIndices: [3]uint32{1}}}); err == nil {
		t.Fatal("special index/role-slot collision accepted")
	}
	characters, err := AdventureEliteCharacterInfo(15, nil, nil)
	if err != nil || len(characters) != 4 || characters[0] != 1 || binary.LittleEndian.Uint16(characters[1:]) != 15 || characters[3] != 0 {
		t.Fatal("native empty type2 container header", characters, err)
	}
}

func TestBoostAPCRequestCapturedPadding(t *testing.T) {
	p := make([]byte, 16)
	p[0] = 1
	if err := DecodeBoostAPCRequest(p); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{nil, {0}, {2}, {1, 1}, append(p, 0)} {
		if err := DecodeBoostAPCRequest(bad); err == nil {
			t.Fatal("invalid request accepted", bad)
		}
	}
}
