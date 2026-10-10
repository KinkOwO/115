package main

import (
	"encoding/json"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/reward"
)

// ---------------------------------------------------------------------------
// 角色待遇（存档字段，不是物品）：挂锁 / 扩容档位 / 复活币
// ---------------------------------------------------------------------------

// entitlementState 造一份带背包的角色 state（字段值直接写在 Bag 上）。
func entitlementState(t *testing.T, bag inventory.Bag) json.RawMessage {
	t.Helper()
	if bag.Version == "" {
		bag.Version = "ordinary-bag-v1"
	}
	raw, err := inventory.SaveBag(json.RawMessage(`{}`), bag)
	if err != nil {
		t.Fatalf("造 state 失败：%v", err)
	}
	return raw
}

func readEntitlementBag(t *testing.T, state json.RawMessage) inventory.Bag {
	t.Helper()
	bag, err := inventory.ReadBag(state)
	if err != nil {
		t.Fatalf("读回背包失败：%v", err)
	}
	return bag
}

// TestApplyEntitlementsUnlocksSlotsByMask：挂锁是**按位或**，已有位不丢。
func TestApplyEntitlementsUnlocksSlotsByMask(t *testing.T) {
	state := entitlementState(t, inventory.Bag{ExpandEquipFlags: 3}) // support + magic stone
	out, receipt, err := applyRewardEntitlements(state, reward.Entitlements{EquipSlotMask: 16 | 32}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := readEntitlementBag(t, out).ExpandEquipFlags
	if got != 3|16|32 {
		t.Fatalf("挂锁位不对：%d，期望 %d", got, 3|16|32)
	}
	if receipt["equip_slot_mask"] == nil {
		t.Fatalf("回执里应当留下这次改动：%+v", receipt)
	}
}

// TestApplyEntitlementsExpansionOnlyRises：扩容档位**只升不降**（写低等于抹掉玩家的格子）。
func TestApplyEntitlementsExpansionOnlyRises(t *testing.T) {
	lower := byte(1)
	state := entitlementState(t, inventory.Bag{Expansion: 2, AvatarExpansion: 15})
	out, receipt, err := applyRewardEntitlements(state, reward.Entitlements{BagTier: &lower, AvatarTier: &lower}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bag := readEntitlementBag(t, out)
	if bag.Expansion != 2 || bag.AvatarExpansion != 15 {
		t.Fatalf("往低改动了：expansion=%d avatar=%d", bag.Expansion, bag.AvatarExpansion)
	}
	if len(receipt) != 0 {
		t.Fatalf("没实际改动就不该写回执：%+v", receipt)
	}

	high := byte(2)
	out, _, err = applyRewardEntitlements(entitlementState(t, inventory.Bag{}), reward.Entitlements{BagTier: &high}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readEntitlementBag(t, out).Expansion; got != 2 {
		t.Fatalf("该升的没升：%d", got)
	}
}

// TestApplyEntitlementsClampsTiers：档位超上限要钳到上限（背包 2 / 时装栏协议常量）。
func TestApplyEntitlementsClampsTiers(t *testing.T) {
	huge := byte(200)
	out, _, err := applyRewardEntitlements(entitlementState(t, inventory.Bag{}),
		reward.Entitlements{BagTier: &huge, AvatarTier: &huge}, nil)
	if err != nil {
		t.Fatal(err)
	}
	bag := readEntitlementBag(t, out)
	if bag.Expansion != maxRewardBagTier {
		t.Fatalf("背包档位没钳住：%d，期望 %d", bag.Expansion, maxRewardBagTier)
	}
	if want := byte(protocol.MaxAvatarInventoryExpansion); bag.AvatarExpansion != want {
		t.Fatalf("时装栏档位没钳住：%d，期望 %d", bag.AvatarExpansion, want)
	}
}

// TestApplyEntitlementsReviveCoinAddsAndSaturates：复活币累加，到顶不回绕成 0。
func TestApplyEntitlementsReviveCoinAddsAndSaturates(t *testing.T) {
	state := entitlementState(t, inventory.Bag{Coin: 5})
	out, _, err := applyRewardEntitlements(state, reward.Entitlements{ReviveCoins: 100}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readEntitlementBag(t, out).Coin; got != 105 {
		t.Fatalf("复活币没累加：%d", got)
	}

	near := entitlementState(t, inventory.Bag{Coin: ^uint32(0) - 1})
	out, _, err = applyRewardEntitlements(near, reward.Entitlements{ReviveCoins: 100}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readEntitlementBag(t, out).Coin; got != ^uint32(0) {
		t.Fatalf("到顶应当停在 MaxUint32，实际 %d", got)
	}
}

// TestApplyEntitlementsEmptyLeavesStateUntouched：零值批次不动 state（连字节都不重写）。
func TestApplyEntitlementsEmptyLeavesStateUntouched(t *testing.T) {
	state := entitlementState(t, inventory.Bag{Coin: 7, Expansion: 2})
	out, receipt, err := applyRewardEntitlements(state, reward.Entitlements{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(state) {
		t.Fatal("空批次不该改写 state")
	}
	if len(receipt) != 0 {
		t.Fatalf("空批次不该有回执：%+v", receipt)
	}
}

// ---------------------------------------------------------------------------
// 宠物本体 / 宠物用品（也是"角色待遇"，但落在宠物容器里）
// ---------------------------------------------------------------------------

// petTestAwarder 造一个能发宠物的目录：一只 `[creature]` 本体 + 一件 `[feed]` 宠物用品。
func petTestAwarder(t *testing.T) *inventory.Awarder {
	t.Helper()
	const source = "test-source"
	cat, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: source},
		Rows: []inventory.EquipmentDefinition{{
			ID: 63003, Path: "equipment/creature/63003.equ", SHA256: strings.Repeat("b", 64),
			Fields: map[string][]pvf.Token{
				"[equipment type]": {{Type: 6, Text: "[creature]"}},
			},
		}},
	}, source)
	if err != nil {
		t.Fatalf("装备目录夹具建不起来：%v", err)
	}
	return &inventory.Awarder{
		Catalog: catalog.LootCatalog{
			Source: pvf.ArchiveSnapshot{Checksum: source},
			Items: map[uint32]catalog.LootItem{
				10418035: {ID: 10418035, Kind: "stackable", StackableType: "[feed]", StackLimit: 1000},
				10356186: {ID: 10356186, Kind: "stackable"},
			},
		},
		Rules:     inventory.BagRules{Model: "test-bag-v1", Source: source},
		Equipment: cat,
	}
}

// TestApplyEntitlementsPetCreatureLandsInPetContainer：本体要落进宠物容器 0..139，
// 而不是普通装备栏（这正是新能力的意义：旧口径 grant_item 会把本体塞进装备栏）。
func TestApplyEntitlementsPetCreatureLandsInPetContainer(t *testing.T) {
	out, receipt, err := applyRewardEntitlements(entitlementState(t, inventory.Bag{}),
		reward.Entitlements{Pets: []reward.ItemGrant{{ID: 63003, Count: 1}}}, petTestAwarder(t))
	if err != nil {
		t.Fatalf("发本体失败：%v", err)
	}
	bag := readEntitlementBag(t, out)
	rows := bag.Special[7]
	if len(rows) != 1 || rows[0].Template != 63003 {
		t.Fatalf("本体没落进宠物容器：%+v", rows)
	}
	if rows[0].Slot > 139 {
		t.Fatalf("本体槽位不在 0..139：%d", rows[0].Slot)
	}
	if len(rows[0].Record) != 0 || rows[0].Period != 0 || rows[0].Durability != 0 {
		t.Fatalf("本体不该带 Record/Period/Durability：%+v", rows[0])
	}
	if receipt["pet_63003"] == nil {
		t.Fatalf("回执里应当留下落位槽号：%+v", receipt)
	}
}

// TestApplyEntitlementsPetItemLandsInConsumableRange：宠物用品要落进 376..431。
func TestApplyEntitlementsPetItemLandsInConsumableRange(t *testing.T) {
	out, _, err := applyRewardEntitlements(entitlementState(t, inventory.Bag{}),
		reward.Entitlements{PetItems: []reward.ItemGrant{{ID: 10418035, Count: 1000}}}, petTestAwarder(t))
	if err != nil {
		t.Fatalf("发宠物用品失败：%v", err)
	}
	bag := readEntitlementBag(t, out)
	if len(bag.PetItems) != 1 || bag.PetItems[0].Template != 10418035 || bag.PetItems[0].Amount != 1000 {
		t.Fatalf("宠物用品没落进 PetItems：%+v", bag.PetItems)
	}
	if s := bag.PetItems[0].Slot; s < inventory.PetConsumableFirst || s > inventory.PetConsumableLast {
		t.Fatalf("宠物用品槽位不在 %d..%d：%d", inventory.PetConsumableFirst, inventory.PetConsumableLast, s)
	}
	if bag.PetItems[0].ExpireTime == 0 {
		t.Fatal("宠物用品应当带永不过期哨兵（0 会被客户端判成已过期）")
	}
}

// TestApplyEntitlementsPetRejectsWrongTemplate：不是本体的模板要给带模板号的报错。
func TestApplyEntitlementsPetRejectsWrongTemplate(t *testing.T) {
	_, _, err := applyRewardEntitlements(entitlementState(t, inventory.Bag{}),
		reward.Entitlements{Pets: []reward.ItemGrant{{ID: 100051381, Count: 1}}}, petTestAwarder(t))
	if err == nil || !strings.Contains(err.Error(), "100051381") {
		t.Fatalf("应当报错并带上模板号，实际：%v", err)
	}
}

// TestApplyEntitlementsPetItemRejectsNonPetConsumable：堆叠物但不是宠物用品 → 报错。
func TestApplyEntitlementsPetItemRejectsNonPetConsumable(t *testing.T) {
	_, _, err := applyRewardEntitlements(entitlementState(t, inventory.Bag{}),
		reward.Entitlements{PetItems: []reward.ItemGrant{{ID: 10356186, Count: 1}}}, petTestAwarder(t))
	if err == nil || !strings.Contains(err.Error(), "10356186") {
		t.Fatalf("应当报错并带上模板号，实际：%v", err)
	}
}

// reward_flow_test.go —— 奖励邮件附件的能力边界。
//
// 业主 2026-10-06 定调：**能不能发是服务端能力，发什么由 mod 的脚本决定**。
// 所以这里钉住三件事：
//
//  1. 堆叠物照旧按目录发（邮件里是堆叠行）；
//  2. **不在掉落目录里、但 PVF 里有定义的装备也要能发** —— 旧口径会一律回
//     「不在奖励目录里」，等于把"发什么"锁死在掉落表上（现场：模板 500950043）；
//  3. 两边都取不到时，报错**必须带上模板号**（池子是一串手抄号，没号就没法查）。

// mailTestAwarder 造一份最小装备目录：一件"上衣"，耐久 60。
func mailTestAwarder(t *testing.T) *inventory.Awarder {
	t.Helper()
	const source = "test-source"
	cat, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: source},
		Rows: []inventory.EquipmentDefinition{{
			ID: 100051381, Path: "equipment/character/common/jacket/cloth/test.equ",
			SHA256: strings.Repeat("a", 64),
			Fields: map[string][]pvf.Token{
				"[rarity]":         {{Type: 0, Value: 0}},
				"[equipment type]": {{Type: 6, Text: "[coat]"}},
				"[durability]":     {{Type: 0, Value: 60}},
			},
		}},
	}, source)
	if err != nil {
		t.Fatalf("装备目录夹具建不起来：%v", err)
	}
	return &inventory.Awarder{
		Catalog:   catalog.LootCatalog{Items: map[uint32]catalog.LootItem{10356186: {ID: 10356186, Kind: "stackable"}}},
		Equipment: cat,
	}
}

func TestRewardMailStackableStaysAStack(t *testing.T) {
	got, err := rewardMailItem(mailTestAwarder(t), reward.ItemGrant{ID: 10356186, Count: 12})
	if err != nil {
		t.Fatalf("堆叠物发不出去：%v", err)
	}
	if got.Stack == nil || got.Equipment != nil {
		t.Fatalf("堆叠物应当走堆叠行：%+v", got)
	}
	if got.Stack.Amount != 12 || got.Stack.ExpireTime == 0 {
		t.Fatalf("堆叠行内容不对：%+v", got.Stack)
	}
}

// TestRewardMailEquipmentOutsideDropCatalog：只带 PVF 定义的装备要能发（本次放宽的能力）。
func TestRewardMailEquipmentOutsideDropCatalog(t *testing.T) {
	got, err := rewardMailItem(mailTestAwarder(t), reward.ItemGrant{ID: 100051381, Count: 1})
	if err != nil {
		t.Fatalf("不在掉落目录、但 PVF 有定义的装备应当能发：%v", err)
	}
	if got.Equipment == nil || got.Stack != nil {
		t.Fatalf("应当走装备行：%+v", got)
	}
	if got.Equipment.Template != 100051381 || got.Equipment.Durability != 60 {
		t.Fatalf("装备行内容不对：%+v", got.Equipment)
	}
}

// TestRewardMailUnknownTemplateNamesTheTemplate：两处都取不到时，报错要带模板号。
func TestRewardMailUnknownTemplateNamesTheTemplate(t *testing.T) {
	_, err := rewardMailItem(mailTestAwarder(t), reward.ItemGrant{ID: 599999999, Count: 1})
	if err == nil {
		t.Fatal("两边都不认识的模板应当报错")
	}
	if !strings.Contains(err.Error(), "599999999") {
		t.Fatalf("报错必须带上模板号，实际：%v", err)
	}
	if !strings.Contains(err.Error(), "取不到奖励耐久") {
		t.Fatalf("报错要说清是哪一步不行，实际：%v", err)
	}
}

// TestRewardMailEquipmentCountMustBeOne：装备附件数量只能是 1（堆叠物才有多件）。
func TestRewardMailEquipmentCountMustBeOne(t *testing.T) {
	_, err := rewardMailItem(mailTestAwarder(t), reward.ItemGrant{ID: 100051381, Count: 2})
	if err == nil || !strings.Contains(err.Error(), "数量必须是 1") {
		t.Fatalf("装备附件数量不是 1 时应明确拒绝，实际：%v", err)
	}
}
