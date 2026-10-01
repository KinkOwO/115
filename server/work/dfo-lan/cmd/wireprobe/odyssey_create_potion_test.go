package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"testing"
)

// odysseyCreatePotionCatalog 是药水测试用的最小掉落目录：只需要 10418028 一条。
//
// 注：2026-10-01 起 `applyOdysseyCreatePotion` **不再**比对目录身份 —— 曾经那行
// `cat.Source.SaveIdentity() != savecontract.Identity()` 两侧恒等、永远通过（等于不写），
// 已随「恒假冗余检查」一起清理；目录来源的 L3 校验若要做，必须比 `.Source.Checksum`。
// Source.Checksum 保留在 fixture 里只是让它保持一个形状完整的最小目录。
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
	// ⚠️ 这里曾经写着「目录校验和不匹配时拒绝」—— 那是**名不副实**：删掉的那行
	// `cat.Source.SaveIdentity() != savecontract.Identity()` 两侧恒等、从来拦不住任何东西，
	// 而真正拦住它的是上面那句「普通角色不发」。现在显式钉住**真实**行为：
	// 角色合法时，换一份目录照样发放（目录身份不参与门禁）。
	role, wear = odysseyRewardFixture(t)
	other := loot
	other.Source.Checksum = "another-source"
	if _, _, e := applyOdysseyCreatePotion(role, other, wear.BagRules); e != nil {
		t.Fatalf("目录身份不应参与门禁（角色合法就该发放）: %v", e)
	}
	t.Log("30 potions stack into the existing row; ordinary role rejected; catalog identity no longer gates the grant")
}
