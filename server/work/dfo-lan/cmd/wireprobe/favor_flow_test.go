package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"testing"
)

// p[6] 是账号材料栏槽位-256：同一 NPC 连送两次的实测帧
// （events.jsonl 2026-09-27）0x6f→367 无色小晶块、0x6c→364 白色小晶块。
func TestFavorGiftSlotDecodesToAccountMaterialSlot(t *testing.T) {
	clear, err := protocol.DecodeFavorGift([]byte{0x00, 0x9d, 0x00, 0x00, 0x00, 0x23, 0x6f, 0x01})
	if err != nil {
		t.Fatal(err)
	}
	if clear.GiftSlot() != 367 {
		t.Fatalf("clear cube gift slot = %d, want 367", clear.GiftSlot())
	}
	white, err := protocol.DecodeFavorGift([]byte{0x00, 0x9d, 0x00, 0x00, 0x00, 0x23, 0x6c, 0x01})
	if err != nil {
		t.Fatal(err)
	}
	if white.GiftSlot() != 364 {
		t.Fatalf("white cube gift slot = %d, want 364", white.GiftSlot())
	}
	tpl, ok := inventory.StorageRowTemplate(white.GiftSlot())
	if !ok || tpl != 3034 {
		t.Fatalf("slot 364 template = %d (%v), want 3034 白色小晶块", tpl, ok)
	}
}

// [favor level point up] 的键是礼物物品，数值是每次送礼的随机点数区间
// [min,max]（[extra favor gift] 的 "物品 1 5000 7000" 条目证实）：
// 无色 100/300，黑白红蓝 200/600，金色 400/900；未知礼物必须拒绝。
// 档位来自 [favor level point down]：500/1000/1500 累计门槛，1500 满。
func TestFavorPointUpFollowsGiftTemplate(t *testing.T) {
	cases := map[uint32][2]int64{
		3037: {100, 300},
		3033: {200, 600},
		3034: {200, 600},
		3035: {200, 600},
		3036: {200, 600},
		3262: {400, 900},
	}
	for tpl, want := range cases {
		minPoint, maxPoint := favorPointUp(tpl)
		if minPoint != want[0] || maxPoint != want[1] {
			t.Fatalf("favorPointUp(%d) = %d/%d, want %d/%d", tpl, minPoint, maxPoint, want[0], want[1])
		}
	}
	if minPoint, _ := favorPointUp(10100115); minPoint != 0 {
		t.Fatal("non-cube account material must not be a favor gift")
	}
	if favorMaxPoint != 1500 || len(favorLevels) != 3 || favorLevels[2] != 1500 {
		t.Fatalf("favor level gates wrong: max=%d levels=%v", favorMaxPoint, favorLevels)
	}
}

// 礼物按所选槽位扣材料：白色小晶块（槽 364 → 3034）必须扣白色，
// 无色库存不受影响；材料不足时拒绝（此前硬编码 3037 扣错颜色）。
func TestFavorGiftSpendsSelectedCube(t *testing.T) {
	if favorDailyLimit != 5 || favorGiftCount != 100 {
		t.Fatalf("favor source rules changed: limit=%d count=%d", favorDailyLimit, favorGiftCount)
	}
	template, ok := inventory.StorageRowTemplate(protocol.FavorGiftRequest{Slot: 0x6c}.GiftSlot())
	if !ok {
		t.Fatal("slot 364 must resolve")
	}
	materials, _, err := inventory.NewAccountMaterials().Add(template, favorGiftCount)
	if err != nil {
		t.Fatal(err)
	}
	after, slot, err := materials.Spend(template, favorGiftCount)
	if err != nil || slot != 364 || after.Count(template) != 0 {
		t.Fatalf("white cube spend = slot %d, remaining %d, err %v", slot, after.Count(template), err)
	}
	if after.Count(3037) != 0 {
		t.Fatal("clear cubes must stay untouched when gifting white cubes")
	}
}
