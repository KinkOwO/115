package loot

import (
	"bytes"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
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
// 件数分布：牌面 6 个格子、装备位最多 4 件（玩家 2026-10-02 核实），而"经常只出一件"
// 的体感来自等概率分布 —— 这条钉住加权后的期望落在 2~3 件。
func TestMoonEquipmentCountExpectation(t *testing.T) {
	p := MoonEquipmentPolicy{Min: 1, Max: 4, Weights: []uint32{1, 3, 3, 2}}
	sum, n := 0, 2000
	for i := 0; i < n; i++ {
		seed := sha256.Sum256([]byte(fmt.Sprintf("seed/%d", i)))
		count, e := moonEquipmentCount(p, seed[:])
		if e != nil {
			t.Fatal(e)
		}
		if count < p.Min || count > p.Max {
			t.Fatalf("件数越界：%d", count)
		}
		sum += int(count)
	}
	mean := float64(sum) / float64(n)
	if mean < 2.0 || mean > 3.0 {
		t.Fatalf("装备件数均值 = %.2f，期望落在 2~3 件", mean)
	}
	t.Logf("装备件数均值 = %.2f（权重 1/3/3/2）", mean)
	// 权重表必须覆盖整个区间，否则整段策略无效。
	if _, e := moonEquipmentCount(MoonEquipmentPolicy{Min: 1, Max: 4, Weights: []uint32{1, 1}}, nil); e == nil {
		t.Fatal("权重长度不符应被拒绝")
	}
	if _, e := moonEquipmentCount(MoonEquipmentPolicy{Min: 1, Max: 2, Weights: []uint32{0, 0}}, nil); e == nil {
		t.Fatal("全零权重应被拒绝")
	}
}

func TestMoonRewardPolicyAndEquipmentValueIsNotCount(t *testing.T) {
	s := moonRewardFixture()
	p := MoonRewardPolicy{Source: s.Catalog.Source.SaveIdentity(), Draws: 1, Choices: []MoonRewardChoice{{Template: 101, Count: 1}, {Template: 102, Count: 1}}}
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
