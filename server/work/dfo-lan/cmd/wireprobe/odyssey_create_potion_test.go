package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"testing"
)

// odysseyCreatePotionCatalog 是药水测试用的最小掉落目录：只需要 10418028 一条，
// 但 Source.Checksum 必须是真源校验和 —— applyOdysseyCreatePotion 会核对它。
func odysseyCreatePotionCatalog() catalog.LootCatalog {
	c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{
		odysseyCreatePotion: {ID: odysseyCreatePotion, Kind: "stackable", StackableType: "[waste]"},
	}}
	c.Source.Checksum = odysseySource()
	return c
}

// 源里 10417791 的 [stackable] 块第三行是 10418028 x30（前两行是武器盒与防具盒）。
// 药水要并进已有的那叠（初始补给已经给到 73 瓶），而不是每次新占一格。
func TestOdysseyCreatePotionStacksIntoExistingRow(t *testing.T) {
	role, wear := odysseyRewardFixture(t)
	loot := odysseyCreatePotionCatalog()

	bag, e := inventory.ReadBag(role.State)
	if e != nil {
		t.Fatal(e)
	}
	bag, slot, e := bag.Add(loot, wear.BagRules, odysseyCreatePotion, 73)
	if e != nil {
		t.Fatal(e)
	}
	if role.State, e = inventory.SaveBag(role.State, bag); e != nil {
		t.Fatal(e)
	}

	raw, receipt, e := applyOdysseyCreatePotion(role, loot, wear.BagRules)
	if e != nil {
		t.Fatal(e)
	}
	out, e := inventory.ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Items) != 1 {
		t.Fatalf("药水被拆成了多叠: %+v", out.Items)
	}
	if out.Items[0].Slot != slot || out.Items[0].Template != odysseyCreatePotion || out.Items[0].Amount != 103 {
		t.Fatalf("药水没有并进原叠: %+v（期望 slot %d, amount 103）", out.Items[0], slot)
	}
	if len(receipt) == 0 {
		t.Fatal("receipt 为空")
	}

	// 普通角色（非奥德赛）不发这件补给
	role.Request[19] = 0
	if _, _, e := applyOdysseyCreatePotion(role, loot, wear.BagRules); e == nil {
		t.Fatal("普通角色也发了奥德赛创建补给")
	}
	// 目录校验和不匹配时拒绝，而不是照发
	other := loot
	other.Source.Checksum = "another-source"
	if _, _, e := applyOdysseyCreatePotion(role, other, wear.BagRules); e == nil {
		t.Fatal("校验和不匹配的目录被接受")
	}
	t.Log("30 potions stack into the existing row; ordinary role and foreign source rejected")
}
