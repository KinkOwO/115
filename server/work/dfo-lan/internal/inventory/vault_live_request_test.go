package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// Captured 2026-09-16T13:02:09Z: slot66/template15/count4 -> vault slot0.
// Keep the literal wire body independent of the transfer implementation.
func TestVaultLiveDeposit130209(t *testing.T) {
	raw, e := hex.DecodeString("0042000f000000040000000200000000000000000000ffffffff000000000000")
	if e != nil {
		t.Fatal(e)
	}
	r, e := protocol.DecodeItemMove(raw)
	if e != nil {
		t.Fatal(e)
	}
	if r.SourceList != 0 || r.SourceSlot != 66 || r.SourceItem != 15 || r.DestinationList != 2 || r.DestinationSlot != 0 || r.DestinationItem != 0 || r.Count != 4 {
		t.Fatal("live request decode", r)
	}
	s, role, v := vaultFixture()
	role.State = json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":66,"Template":15,"Amount":50}]}}`)
	state, items, e := s.TransferStacks(role, v, r)
	if e != nil {
		t.Fatal("captured deposit rejected", e)
	}
	b, e := ReadBag(state)
	if e != nil {
		t.Fatal(e)
	}
	v.Items = items
	rows, e := ReadVault(v)
	if e != nil || len(b.Items) != 1 || b.Items[0].Amount != 46 || len(rows) != 1 || rows[0].Slot != 0 || rows[0].Amount != 4 {
		t.Fatal("deposit conservation", b, rows, e)
	}
	// A withdrawal uses source vault and destination bag in the same format.
	role.State = state
	withdraw := protocol.ItemMoveRequest{SourceList: 2, SourceSlot: 0, SourceItem: 15, DestinationList: 0, DestinationSlot: 66, DestinationItem: 15, Count: 4, Selection: 0xffffffff}
	state, items, e = s.TransferStacks(role, v, withdraw)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = ReadBag(state)
	v.Items = items
	rows, e = ReadVault(v)
	if e != nil || len(rows) != 0 || b.Items[0].Amount != 50 {
		t.Fatal("roundtrip conservation", b, rows, e)
	}
	t.Log("live deposit PASS: bag66 50->46; vault0 0->4; withdrawal restores bag50/vault0")
}
