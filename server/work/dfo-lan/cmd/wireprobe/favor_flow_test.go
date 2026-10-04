package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/world"
	"strings"
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

// 染色(p[0]=1)应答必须是 0x01(result) + 请求体原样回显（逆向 parser
// 0x14528f570 定论：byte1=op，1=染色分支，本地写颜色、不弹好感度窗）。
// 实测染色帧 01 79 eb f5 05 01 00 11（npc=100002681, slot=0,
// color=0x11），正确应答 9 字节；退回 16B 送礼 ack 会让 byte1=0 误走
// 送礼分支弹"好感度增加了X"。
func TestFavorDyeAckEchoesRequestBody(t *testing.T) {
	req := []byte{0x01, 0x79, 0xeb, 0xf5, 0x05, 0x01, 0x00, 0x11}
	decoded, err := protocol.DecodeFavorGift(req)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.IsDye() || decoded.NPCID != 100002681 {
		t.Fatalf("decode dye request wrong: %+v", decoded)
	}
	ack := protocol.FavorDyeAck(req)
	want := []byte{0x01, 0x01, 0x79, 0xeb, 0xf5, 0x05, 0x01, 0x00, 0x11}
	if len(ack) != len(want) {
		t.Fatalf("dye ack len = %d, want %d", len(ack), len(want))
	}
	for i := range want {
		if ack[i] != want[i] {
			t.Fatalf("dye ack = %x, want %x", ack, want)
		}
	}
}

// [favor level point up] 的键是礼物物品，数值是每次送礼的随机点数区间
// [min,max]（[extra favor gift] 的 "物品 1 5000 7000" 条目证实）：
// 无色 100/300，黑白红蓝 200/600，金色 400/900；未知礼物必须拒绝。
// 档位来自 [favor level point down]：500/1000/1500 累计门槛，1500 满。
func TestFavorPointUpFollowsGiftTemplate(t *testing.T) {
	favor := favorRulesFixture(t)
	cases := map[uint32][2]int64{
		3037: {100, 300},
		3033: {200, 600},
		3034: {200, 600},
		3035: {200, 600},
		3036: {200, 600},
		3262: {400, 900},
	}
	for tpl, want := range cases {
		minPoint, maxPoint := favor.PointRange(tpl)
		if minPoint != want[0] || maxPoint != want[1] {
			t.Fatalf("favorPointUp(%d) = %d/%d, want %d/%d", tpl, minPoint, maxPoint, want[0], want[1])
		}
	}
	if minPoint, _ := favor.PointRange(10100115); minPoint != 0 {
		t.Fatal("non-cube account material must not be a favor gift")
	}
	if favor.MaxPoint() != 1500 || len(favor.Levels) != 3 || favor.Levels[2] != 1500 {
		t.Fatalf("favor level gates wrong: max=%d levels=%v", favor.MaxPoint(), favor.Levels)
	}
}

// 礼物按所选槽位扣材料：白色小晶块（槽 364 → 3034）必须扣白色，
// 无色库存不受影响；材料不足时拒绝（此前硬编码 3037 扣错颜色）。
func TestFavorGiftSpendsSelectedCube(t *testing.T) {
	favor := favorRulesFixture(t)
	if favor.DailyLimit != 5 || favor.GiftCount != 100 {
		t.Fatalf("favor source rules changed: limit=%d count=%d", favor.DailyLimit, favor.GiftCount)
	}
	template, ok := inventory.StorageRowTemplate(protocol.FavorGiftRequest{Slot: 0x6c}.GiftSlot())
	if !ok {
		t.Fatal("slot 364 must resolve")
	}
	materials, _, err := inventory.NewAccountMaterials().Add(template, favor.GiftCount)
	if err != nil {
		t.Fatal(err)
	}
	after, slot, err := materials.Spend(template, favor.GiftCount)
	if err != nil || slot != 364 || after.Count(template) != 0 {
		t.Fatalf("white cube spend = slot %d, remaining %d, err %v", slot, after.Count(template), err)
	}
	if after.Count(3037) != 0 {
		t.Fatal("clear cubes must stay untouched when gifting white cubes")
	}
}

// Observed source values belong to this test input, not the runtime executor.
func favorRulesFixture(t *testing.T) *catalog.NPCFavorRules {
	t.Helper()
	var cells []pvf.Token
	add := func(tag string, nums ...int32) {
		cells = append(cells, pvf.Token{Type: 3, Text: tag})
		for _, n := range nums {
			cells = append(cells, pvf.Token{Type: 0, Value: n})
		}
	}
	add("[favor condition level]", 20)
	add("[favor gift item count]", 100)
	add("[favor gift limit]", 5)
	add("[favor level point up]", 3037, 100, 300, 3033, 200, 600, 3034, 200, 600, 3035, 200, 600, 3036, 200, 600, 3262, 400, 900)
	add("[/favor level point up]")
	add("[favor level point down]", 0, 21, 500, 1, 14, 1000, 2, 7, 1500)
	add("[/favor level point down]")
	r, e := catalog.ParseNPCFavorRules("test-source", catalog.ScriptRecord{Cells: cells})
	if e != nil {
		t.Fatal(e)
	}
	return r
}

func TestFavorGiftUsesNativeEligibilityAndRejectsMissingRules(t *testing.T) {
	rules := favorRulesFixture(t)
	rules.OpenLevel = 27
	w := &worldSession{role: database.Character{ID: 1, State: []byte(`{"level":26}`)}, store: &database.Store{}, loot: &loot.Service{}, service: &world.Service{Catalog: catalog.WorldCatalog{Favor: rules}}}
	p := []byte{0, 0x9d, 0, 0, 0, 0x23, 0x6c, 1}
	if _, err := w.giveFavor(p); err == nil || !strings.Contains(err.Error(), "level 27") {
		t.Fatalf("source eligibility ignored: %v", err)
	}
	w.service.Catalog.Favor = nil
	if _, err := w.giveFavor(p); err == nil || !strings.Contains(err.Error(), "native favor rules unavailable") {
		t.Fatalf("missing source fell back: %v", err)
	}
}
