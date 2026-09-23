package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// The live CMD 18 frame a player sends when a skill costs 无色小晶块 (3037),
// captured 2026-09-23 during dungeon 27:
//
//	10000000 1000 1a0a 0802 10ef02 18dd17 200f 200000000000000000
//
// The row names slot 367, which is the account-shared material store's cell for
// template 3037 (inventory.accountMaterialSlotByTemplate), not an ordinary-bag
// material cell. The parser used to admit only the bag's [121,176] window and
// rejected this frame outright, so the cost was never deducted and the client
// was left holding a pending reservation.
func TestLiveSkillMaterialFrameNamesAccountStorageSlot(t *testing.T) {
	const liveHex = "1000000010001a0a080210ef0218dd17200f2000000000000000000000000000"
	raw, err := hex.DecodeString(liveHex)
	if err != nil {
		t.Fatalf("hex: %v", err)
	}

	rows, err := DecodeMaterialDelete(raw)
	if err != nil {
		t.Fatalf("DecodeMaterialDelete refused the live frame: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Slot != 367 {
		t.Errorf("slot = %d, want 367 (account material store cell of 3037)", rows[0].Slot)
	}
	if rows[0].Template != 3037 {
		t.Errorf("template = %d, want 3037 (无色小晶块)", rows[0].Template)
	}
	if rows[0].Count == 0 {
		t.Error("count = 0")
	}

	// Read the protobuf by hand too, so the test still pins the wire evidence
	// rather than only the parser's agreement with itself.
	n := int(binary.LittleEndian.Uint32(raw))
	if n != 16 {
		t.Fatalf("declared body length = %d, want 16", n)
	}
	body := raw[4 : 4+n]
	var row []byte
	for len(body) > 0 {
		tag, k := binary.Uvarint(body)
		if k <= 0 {
			t.Fatalf("truncated tag in %x", body)
		}
		body = body[k:]
		switch tag {
		case 16, 32: // varints
			_, k := binary.Uvarint(body)
			if k <= 0 {
				t.Fatalf("truncated value after tag %d", tag)
			}
			body = body[k:]
		case 26: // the row
			l, k := binary.Uvarint(body)
			if k <= 0 || int(l) > len(body)-k {
				t.Fatalf("bad row length")
			}
			row = body[k : k+int(l)]
			body = body[k+int(l):]
		default:
			t.Fatalf("unexpected tag %d", tag)
		}
	}
	if row == nil {
		t.Fatal("no row found in the live frame")
	}
	fields := map[uint64]uint64{}
	for len(row) > 0 {
		rt, a := binary.Uvarint(row)
		if a <= 0 {
			t.Fatalf("truncated row tag")
		}
		row = row[a:]
		rv, b := binary.Uvarint(row)
		if b <= 0 {
			t.Fatalf("truncated row value after tag %d", rt)
		}
		row = row[b:]
		fields[rt] = rv
	}
	if got := fields[8]; got != 2 {
		t.Errorf("row reason = %d, want 2 (skill cost)", got)
	}
	if got := fields[16]; got != 367 {
		t.Errorf("row slot = %d, want 367", got)
	}
	if got := fields[24]; got != 3037 {
		t.Errorf("row template = %d, want 3037", got)
	}
}

// The widened window must stay narrow in the ways that matter: a slot in
// neither store, or a template other than the one skill costs are paid in, is
// still refused.
func TestSkillMaterialDeletionRejectsUnknownSlots(t *testing.T) {
	build := func(slot, template uint64) []byte {
		row := []byte{8, 2}
		row = binary.AppendUvarint(append(row, 16), slot)
		row = binary.AppendUvarint(append(row, 24), template)
		row = binary.AppendUvarint(append(row, 32), 1)
		// Outer shape mirrors the live frame: field 2 (tag 16) = 0, the row at
		// field 3 (tag 26), field 4 (tag 32) = 0.
		body := []byte{16, 0, 26}
		body = binary.AppendUvarint(body, uint64(len(row)))
		body = append(body, row...)
		body = append(body, 32, 0)
		p := make([]byte, 4)
		binary.LittleEndian.PutUint32(p, uint32(len(body)))
		return append(p, body...)
	}
	for _, c := range []struct {
		name     string
		slot     uint64
		template uint64
	}{
		{"bag material cell", 121, 3037},
		{"storage first cell", 363, 3037},
		{"storage last cell", 379, 3037},
	} {
		if _, err := DecodeMaterialDelete(build(c.slot, c.template)); err != nil {
			t.Errorf("%s (slot %d): refused: %v", c.name, c.slot, err)
		}
	}
	for _, c := range []struct {
		name     string
		slot     uint64
		template uint64
	}{
		{"below both stores", 120, 3037},
		{"between the stores", 200, 3037},
		{"above both stores", 380, 3037},
		{"storage cell with a foreign template", 367, 3033},
		{"bag cell with a foreign template", 121, 42},
	} {
		if _, err := DecodeMaterialDelete(build(c.slot, c.template)); err == nil {
			t.Errorf("%s (slot %d template %d): accepted", c.name, c.slot, c.template)
		}
	}
}
