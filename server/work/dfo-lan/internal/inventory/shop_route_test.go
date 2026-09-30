package inventory

import (
	"encoding/hex"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// 端到端回归：用 2026-09-29 实机日志里的真实 CMD21 字节，验证修复后
// 「装备之力魔法书」的商品能落到正确的商店（而不是被当成圣诞树）。
//
// 日志原文（plain_hex）：
//
//	30da9d00 01000000 b6e3f505 eee7f505 073d0000 00000000
//
// 修复前：NpcID=p[8]=100000694（圣诞树）→ 查不到 → 走金币 → 无 buy 价 → 拒绝
// 修复后：ResolveShop(100000694, 100001774) = 100001774 → 有材料 → 正常购买
func TestBuyItemRequestResolvesEnchantBookShop(t *testing.T) {
	raw, err := hex.DecodeString("30da9d0001000000b6e3f505eee7f505073d000000000000")
	if err != nil {
		t.Fatal(err)
	}
	r, err := protocol.DecodeBuyItem(raw)
	if err != nil {
		t.Fatal(err)
	}
	if r.Template != 10345008 {
		t.Fatalf("Template=%d，期望 10345008", r.Template)
	}
	if r.Count != 1 {
		t.Fatalf("Count=%d，期望 1", r.Count)
	}
	if r.NpcID != 100000694 {
		t.Fatalf("NpcID=%d，期望 100000694", r.NpcID)
	}
	if r.ActorID != 100001774 {
		t.Fatalf("ActorID=%d，期望 100001774", r.ActorID)
	}

	// 真实目录里 100001774 是商店、100000694 不是。
	shops, err := catalog.LoadItemShops("../../configs/itemshop-candidate.json")
	if err != nil {
		t.Skip(err)
	}
	id, ok := shops.ResolveShop(r.NpcID, r.ActorID)
	if !ok {
		t.Fatal("ResolveShop 没找到商店 —— 修复失效")
	}
	if id != 100001774 {
		t.Fatalf("ResolveShop=%d，期望 100001774", id)
	}
	_, listed, paid := shops.Materials(id, r.Template)
	if !listed {
		t.Fatalf("商店 %d 没列模板 %d", id, r.Template)
	}
	// ⚠️ 与作者树的**刻意分歧**：他那边把物品脚本的 [need material] **烘焙进**
	// itemshop-candidate.json（`paid=true`），本仓的材料来自**运行时**解析
	// `configs/item-materials.json`（internal/catalog/item_materials.go），
	// `inventory.ShopService.Buy` 会把两条来源合起来用（先看 .shp 再看物品脚本）。
	// 所以这里按本仓的链路断言：两条来源合起来必须给出实机那 2 个材料。
	mats, _, _ := shops.Materials(id, r.Template) // .shp 侧（本仓通常为空）
	if !paid {
		im, e := catalog.LoadItemMaterials("../../configs/item-materials.json")
		if e != nil {
			t.Skipf("item-materials.json 不可用：%v", e)
		}
		itemMats, itemPaid := im.Materials(r.Template)
		if !itemPaid {
			t.Fatalf("两条材料来源都没给出模板 %d 的材料", r.Template)
		}
		if len(itemMats) != 2 {
			t.Fatalf("物品脚本材料数=%d，期望 2", len(itemMats))
		}
		t.Logf("解析成功：商店=%d 材料来自物品脚本：%v", id, itemMats)
		return
	}
	if len(mats) != 2 {
		t.Fatalf("材料数=%d，期望 2", len(mats))
	}
	t.Logf("解析成功：商店=%d 材料来自 .shp：%v", id, mats)
}
