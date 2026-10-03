package inventory

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/savecontract"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func vaultFixture() (*VaultService, Role, VaultState) {
	h := strings.Repeat("a", 64)
	s := &VaultService{Rules: VaultRules{SourceSHA256: h, InitialSlots: 8, VerifiedSlots: []uint16{8, 24}}, BagRules: BagRules{Source: h, Slots: map[string][2]uint16{"[material]": {121, 176}}, MissingStackLimit: 1000, QuickSlots: [2]uint16{0, 8}}, Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: h}, Items: map[uint32]catalog.LootItem{15: {ID: 15, Kind: "stackable", StackableType: "[etc]", StackLimit: 1000}, 16: {ID: 16, Kind: "stackable", StackableType: "[material]", StackLimit: 100}}}}
	role := Role{AccountID: 1, ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"level":55,"inventory":{"version":"ordinary-bag-v1","gold":100,"items":[{"slot":65,"Template":15,"Amount":10}]}}`)}
	v := VaultState{Slots: 8, ConfigVersion: h, Items: json.RawMessage(`[]`)}
	return s, role, v
}

func TestVaultSplitMergeWithdrawAndRestore(t *testing.T) {
	s, role, v := vaultFixture()
	original := append([]byte(nil), role.State...)
	r := protocol.ItemMoveRequest{DestinationList: 2, DestinationSlot: 0, SourceList: 0, SourceSlot: 65, SourceItem: 15, Count: 4, Selection: 0xffffffff}
	state, items, e := s.TransferStacks(role, v, r)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(original, role.State) || string(v.Items) != "[]" {
		t.Fatal("input changed before commit")
	}
	role.State = state
	v.Items = items
	b, _ := ReadBag(state)
	rows, _ := ReadVaultBagItems(v)
	if b.Gold != 100 || b.Items[0].Amount != 6 || rows[0].Slot != 0 || rows[0].Amount != 4 {
		t.Fatal(b, rows)
	}
	p, e := VaultPayload(v)
	if e != nil || len(p) != 187 || binary.LittleEndian.Uint16(p[3:]) != 1 || binary.LittleEndian.Uint32(p[11:]) != 4 {
		t.Fatal("native vault cursor", e, p)
	}
	r.DestinationItem = 15
	r.Count = 6
	role.State, v.Items, e = s.TransferStacks(role, v, r)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = ReadBag(role.State)
	if len(b.Items) != 0 {
		t.Fatal("full deposit left source stack")
	}
	r = protocol.ItemMoveRequest{DestinationList: 0, DestinationSlot: 66, SourceList: 2, SourceSlot: 0, SourceItem: 15, Count: 10, Selection: 0xffffffff}
	role.State, v.Items, e = s.TransferStacks(role, v, r)
	if e != nil {
		t.Fatal(e)
	}
	b, _ = ReadBag(role.State)
	rows, _ = ReadVaultBagItems(v)
	if len(rows) != 0 || len(b.Items) != 1 || b.Items[0].Amount != 10 || b.Items[0].Slot != 66 {
		t.Fatal("withdraw conservation", b, rows)
	}
	p, e = VaultPayload(v)
	if e != nil || !bytes.Equal(p, []byte{2, 8, 0, 0, 0, 0}) {
		t.Fatal("empty restore changed", e, p)
	}
	var extra map[string]json.RawMessage
	json.Unmarshal(role.State, &extra)
	if string(extra["level"]) != "55" {
		t.Fatal("lost unrelated state")
	}
}

func TestVaultRejectsInvalidMovesWithoutMutation(t *testing.T) {
	for name, edit := range map[string]func(*protocol.ItemMoveRequest){
		"locked slot":  func(r *protocol.ItemMoveRequest) { r.DestinationSlot = 8 },
		"stale source": func(r *protocol.ItemMoveRequest) { r.SourceItem = 16 },
		"stale target": func(r *protocol.ItemMoveRequest) { r.DestinationItem = 15 },
		"zero":         func(r *protocol.ItemMoveRequest) { r.Count = 0 },
		"excess":       func(r *protocol.ItemMoveRequest) { r.Count = 11 },
		"other vault":  func(r *protocol.ItemMoveRequest) { r.DestinationList = 45 },
		"extra":        func(r *protocol.ItemMoveRequest) { r.Extra = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			s, role, v := vaultFixture()
			r := protocol.ItemMoveRequest{DestinationList: 2, SourceSlot: 65, SourceItem: 15, Count: 1, Selection: 0xffffffff}
			edit(&r)
			state := string(role.State)
			if _, _, e := s.TransferStacks(role, v, r); e == nil {
				t.Fatal("accepted invalid move")
			}
			if state != string(role.State) || string(v.Items) != "[]" {
				t.Fatal("input mutated")
			}
		})
	}
	s, role, v := vaultFixture()
	v.Items = json.RawMessage(`[{"slot":0,"Template":15,"Amount":999}]`)
	r := protocol.ItemMoveRequest{DestinationList: 2, DestinationItem: 15, SourceSlot: 65, SourceItem: 15, Count: 2, Selection: 0xffffffff}
	if _, _, e := s.TransferStacks(role, v, r); e == nil {
		t.Fatal("overflow merge")
	}
	v.Items = json.RawMessage(`[{"slot":0,"Template":16,"Amount":10}]`)
	r = protocol.ItemMoveRequest{DestinationList: 0, DestinationSlot: 66, SourceList: 2, SourceSlot: 0, SourceItem: 16, Count: 10, Selection: 0xffffffff}
	if _, _, e := s.TransferStacks(role, v, r); e == nil {
		t.Fatal("material placed into consumables")
	}
	r.DestinationSlot = 121
	if _, _, e := s.TransferStacks(role, v, r); e != nil {
		t.Fatal(e)
	}
}

func TestVaultSavedRowsAndCapacity(t *testing.T) {
	_, _, v := vaultFixture()
	for _, raw := range []string{`null`, `{}`, `[] []`, `[{"slot":8,"Template":15,"Amount":1}]`, `[{"slot":0,"Template":15,"Amount":1,"expiry":1}]`, `[{"slot":0,"Template":15,"Amount":1},{"slot":0,"Template":16,"Amount":1}]`, `[{"slot":0,"Template":15,"Amount":0}]`} {
		v.Items = []byte(raw)
		if _, e := VaultPayload(v); e == nil {
			t.Fatal("bad saved rows", raw)
		}
	}
	rows := [][protocol.CurrentItemRecordSize]byte{protocol.OrdinaryItem(0, 15, 1), protocol.OrdinaryItem(7, 16, 2)}
	if p, e := protocol.PersonalVault(8, rows); e != nil || len(p) != 368 {
		t.Fatal(e, len(p))
	}
	rows[1] = protocol.OrdinaryItem(8, 16, 2)
	if _, e := protocol.PersonalVault(8, rows); e == nil {
		t.Fatal("capacity off-by-one")
	}
}
