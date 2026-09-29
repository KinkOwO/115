package loot

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func moonRewardFixture() *Service {
	h := strings.Repeat("a", 64)
	return &Service{
		Catalog:  catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: h}, Items: map[uint32]catalog.LootItem{101: {ID: 101, Kind: "stackable", StackableType: "material", StackLimit: 10}, 102: {ID: 102, Kind: "stackable", StackableType: "material", StackLimit: 10}}},
		BagRules: inventory.BagRules{Source: h, Slots: map[string][2]uint16{"material": {121, 121}}, MissingStackLimit: 10},
	}
}
func TestMoonRewardFullBagIsAtomicAndPreservesOtherState(t *testing.T) {
	s := moonRewardFixture()
	before := json.RawMessage(`{"unknown":{"kept":99},"inventory":{"version":"ordinary-bag-v1","gold":123,"coin":7}}`)
	original := append([]byte(nil), before...)
	_, e := s.applyMoonRewards(before, MoonRewardPlan{Grants: []MoonRewardGrant{{Template: 101, Count: 1}, {Template: 102, Count: 1}}})
	if !errors.Is(e, ErrMoonBagFull) || !bytes.Equal(before, original) {
		t.Fatal("partial full-bag mutation", e)
	}
	after, e := s.applyMoonRewards(before, MoonRewardPlan{Grants: []MoonRewardGrant{{Template: 101, Count: 2}}})
	if e != nil {
		t.Fatal(e)
	}
	bag, e := inventory.ReadBag(after)
	if e != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 2 || bag.Gold != 123 || bag.Coin != 7 {
		t.Fatal("bad grant", e)
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(after, &fields)
	if !bytes.Contains(fields["unknown"], []byte("99")) {
		t.Fatal("unrelated state lost")
	}
}
func TestMoonRewardPolicyAndEquipmentValueIsNotCount(t *testing.T) {
	s := moonRewardFixture()
	p := MoonRewardPolicy{Source: s.Catalog.Source.Checksum, Draws: 1, Choices: []MoonRewardChoice{{Template: 101, Count: 1, Weight: 80}, {Template: 102, Count: 1, Weight: 20}}}
	if e := s.ValidateMoonRewards(p); e != nil {
		t.Fatal(e)
	}
	p.Source = strings.Repeat("b", 64)
	if e := s.ValidateMoonRewards(p); e == nil {
		t.Fatal("foreign source")
	}
	row := protocol.OrdinaryItem(0, 12345, 999999999)
	plan := MoonRewardPlan{Grants: []MoonRewardGrant{{Template: 12345, Count: 1, Record: row[:]}}}
	got := plan.WireRows()
	if len(got) != 1 || !got[0].Equipment || got[0].Value != 999999999 || plan.Grants[0].Count != 1 {
		t.Fatal("instance value used as quantity")
	}
	bad := append([]byte(nil), row[:]...)
	binary.LittleEndian.PutUint32(bad[2:], 999)
	if _, e := s.applyMoonRewards(json.RawMessage("{}"), MoonRewardPlan{Grants: []MoonRewardGrant{{Template: 12345, Count: 1, Record: bad}}}); e == nil {
		t.Fatal("corrupt equipment accepted")
	}
}
