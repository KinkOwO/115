package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestVaultLiveManualMerge130633(t *testing.T) {
	s, role, v := vaultFixture()
	v.Items = json.RawMessage(`[{"slot":0,"Template":15,"Amount":4},{"slot":1,"Template":15,"Amount":6}]`)
	raw, _ := hex.DecodeString("0200000f000000000000000201000f00000000000000ffffffff000000000000")
	r, e := protocol.DecodeItemMove(raw)
	if e != nil {
		t.Fatal(e)
	}
	_, items, e := s.TransferStacks(role, v, r)
	if e != nil {
		t.Fatal("live manual merge rejected", e)
	}
	v.Items = items
	rows, e := ReadVault(v)
	if e != nil || len(rows) != 1 || rows[0].Slot != 1 || rows[0].Amount != 10 {
		t.Fatal("manual 4+6 merge", rows, e)
	}
	t.Log("PASS live zero-count warehouse merge: slot0=4 + slot1=6 -> slot1=10")
}

func TestVaultLiveDepositAutoMerge130632(t *testing.T) {
	s, role, v := vaultFixture()
	role.State = json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":66,"Template":15,"Amount":46}]}}`)
	v.Items = json.RawMessage(`[{"slot":0,"Template":15,"Amount":4}]`)
	raw, _ := hex.DecodeString("0042000f000000060000000201000000000000000000ffffffff000000000000")
	r, e := protocol.DecodeItemMove(raw)
	if e != nil {
		t.Fatal(e)
	}
	state, items, e := s.TransferStacks(role, v, r)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := ReadBag(state)
	v.Items = items
	rows, e := ReadVault(v)
	if e != nil || len(rows) != 1 || rows[0].Slot != 0 || rows[0].Amount != 10 || b.Items[0].Amount != 40 {
		t.Fatal("deposit did not consolidate", b, rows, e)
	}
	t.Log("PASS live six-item deposit: bag46->40; existing vault4->10; requested empty slot stays empty")
}

func TestVaultMergeLimitsAndAtomicFailure(t *testing.T) {
	s, role, v := vaultFixture()
	v.Items = json.RawMessage(`[{"slot":0,"Template":15,"Amount":4},{"slot":1,"Template":15,"Amount":999}]`)
	before := string(v.Items)
	r := protocol.ItemMoveRequest{SourceList: 2, SourceSlot: 0, SourceItem: 15, DestinationList: 2, DestinationSlot: 1, DestinationItem: 15, Selection: 0xffffffff}
	if _, _, e := s.TransferStacks(role, v, r); e == nil {
		t.Fatal("overflow manual merge accepted")
	}
	if string(v.Items) != before {
		t.Fatal("failed merge mutated input")
	}
	v.Items = json.RawMessage(`[{"slot":0,"Template":15,"Amount":998}]`)
	r = protocol.ItemMoveRequest{SourceList: 0, SourceSlot: 65, SourceItem: 15, DestinationList: 2, DestinationSlot: 1, Count: 6, Selection: 0xffffffff}
	state, items, e := s.TransferStacks(role, v, r)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := ReadBag(state)
	v.Items = items
	rows, _ := ReadVault(v)
	if len(rows) != 2 || rows[0].Amount != 1000 || rows[1].Slot != 1 || rows[1].Amount != 4 || b.Items[0].Amount != 4 {
		t.Fatal("partial fill remainder", b, rows)
	}
	if string(role.State) == string(state) {
		t.Fatal("missing mutation")
	}
	r.Count = 0
	if _, _, e = s.TransferStacks(role, v, r); e == nil {
		t.Fatal("cross-container zero accepted")
	}
}
