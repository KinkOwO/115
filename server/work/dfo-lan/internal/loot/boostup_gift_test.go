package loot

import (
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func TestBoostGiftCandidateIsCharacterScopedAndAtomic(t *testing.T) {
	s := &Service{Catalog: catalog.LootCatalog{Items: map[uint32]catalog.LootItem{
		3: {ID: 3, Kind: "stackable", StackableType: "[waste]", StackLimit: 1},
		4: {ID: 4, Kind: "stackable", StackableType: "[waste]", StackLimit: 1},
	}}, BagRules: inventory.BagRules{Slots: map[string][2]uint16{"[waste]": {65, 65}}, MissingStackLimit: 1}}
	role := Role{ID: 1, AccountID: 1, State: json.RawMessage(`{"unknown":{"preserve":42}}`)}
	// 本树 Bag.Add 把模板 0/1 保留为金币/游戏币，donor 夹具的 1/2 会落进
	// 货币分支而不是 [waste] 段；机械改用 3/4，断言语义不变。
	g := boostup.Gift{ID: 117, Direct: true, Trigger: "click button", Items: []uint32{3}}
	raw, receipt, e := s.PrepareBoostGift(role, g)
	if e != nil || len(receipt.Slots) != 1 || receipt.Slots[0] != 65 {
		t.Fatal(receipt, e)
	}
	state, e := boostup.ReadState(raw)
	if e != nil || !state.Gifts[117] {
		t.Fatal(state, e)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(raw, &fields); e != nil || string(fields["unknown"]) != "{\"preserve\":42}" {
		t.Fatal("unknown fields lost")
	}
	if original, e := boostup.ReadState(role.State); e != nil || original.Gifts[117] {
		t.Fatal("precommit mutation")
	}
	other := role
	other.ID = 2
	if _, _, e = s.PrepareBoostGift(other, g); e != nil {
		t.Fatal("another role on same account was blocked", e)
	}
	role.State = raw
	if _, _, e = s.PrepareBoostGift(role, g); e == nil {
		t.Fatal("second grant without receipt allowed")
	}
	g.ID = 118
	g.Items = []uint32{4}
	if changed, _, e := s.PrepareBoostGift(role, g); e == nil || changed != nil {
		t.Fatal("full bag partially marked claim")
	}
	if st, _ := boostup.ReadState(role.State); st.Gifts[118] {
		t.Fatal("full bag consumed entitlement")
	}
}
