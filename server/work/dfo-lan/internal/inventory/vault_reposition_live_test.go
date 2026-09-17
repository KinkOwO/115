package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestVaultLiveReposition131224(t *testing.T) {
	s, role, v := vaultFixture()
	v.Items = json.RawMessage(`[{"slot":4,"Template":21,"Amount":16}]`)
	item := s.Catalog.Items[15]
	item.ID = 21
	s.Catalog.Items[21] = item
	raw, _ := hex.DecodeString("02030000000000000000000204001500000000000000ffffffff000000000000")
	r, e := protocol.DecodeItemMove(raw)
	if e != nil {
		t.Fatal(e)
	}
	before := string(role.State)
	state, items, e := s.TransferStacks(role, v, r)
	if e != nil {
		t.Fatal("live empty-slot move rejected", e)
	}
	v.Items = items
	rows, e := ReadVault(v)
	if e != nil || len(rows) != 1 || rows[0].Slot != 3 || rows[0].Amount != 16 || rows[0].Template != 21 {
		t.Fatal(rows, e)
	}
	var a, b any
	json.Unmarshal([]byte(before), &a)
	json.Unmarshal(state, &b)
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if string(aa) != string(bb) {
		t.Fatal("warehouse move changed bag")
	}
	r.SourceSlot = 4
	r.DestinationSlot = 3
	_, items, e = s.TransferStacks(role, v, r)
	if e != nil {
		t.Fatal(e)
	}
	v.Items = items
	rows, _ = ReadVault(v)
	if len(rows) != 1 || rows[0].Slot != 4 || rows[0].Amount != 16 {
		t.Fatal("roundtrip", rows)
	}
	r.SourceSlot = 8
	if _, _, e = s.TransferStacks(role, v, r); e == nil {
		t.Fatal("locked slot accepted")
	}
	t.Log("PASS live warehouse reposition: slot4->3->4; template21 count16 unchanged; bag unchanged")
}
