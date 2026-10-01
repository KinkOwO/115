package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// odysseySource 是奥德赛创造奖励链（护甲/武器箱/药剂）所用角色与目录的源身份。
//
// 2026-10-01（next146）：直读模式下角色 ConfigVersion、护甲/武器目录的 source
// 全都等于当次内层 checksum，而不是编译期写死的 "7ef2db59…"。这里改为从
// catalog.OdysseySource（由直读目录准备阶段 SetOdysseySource 切好）**读取**，
// 保持与整族令牌一致；用函数而不是 const，避免包初始化顺序把旧值固化。
func odysseySource() string { return catalog.OdysseySource }
const odysseyArmorEvent = "odyssey-create-10417791-armor-10417790-v1"
const odysseyWeaponBoxEvent = "odyssey-create-10417791-weapon-box-10417789-v1"
const odysseyCreatePotionEvent = "odyssey-create-10417791-potion-10418028-v1"

// Source create reward10417791 contains armor box10417790. Its single
// selection-num0 category awards these eight pieces, not one random item.
// The third line of the same [stackable] block (potion10418028 x30) is settled
// by grantOdysseyCreatePotion below.
var odysseyArmor = [...]uint32{100051399, 100101277, 100151218, 100201190, 100251230, 100302054, 100313767, 100323647}

func isOdysseyRewardRole(role storage.Character) bool {
	return character.OdysseyRole(role)
}

func applyOdysseyArmor(role storage.Character, wear *inventory.WearService) (json.RawMessage, json.RawMessage, error) {
	if !isOdysseyRewardRole(role) || role.ConfigVersion != odysseySource() || wear == nil || wear.Catalog == nil || wear.Catalog.Source.Checksum != odysseySource() || wear.BagRules.Source != odysseySource() {
		return nil, nil, fmt.Errorf("Odyssey armor requires matching character and source catalogs")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	for _, id := range odysseyArmor {
		b, _, e = b.AddEquipment(wear.Catalog, wear.BagRules.EquipmentSlots, id, 1)
		if e != nil {
			return nil, nil, e
		}
	}
	raw, e := inventory.SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]any{"create_reward": 10417791, "armor_box": 10417790, "templates": odysseyArmor, "before": role.State, "after": raw, "weapon_settled": false, "potion_settled": false})
	return raw, receipt, e
}

func grantOdysseyArmor(ctx context.Context, store *storage.Store, wear *inventory.WearService, role storage.Character) (storage.Character, bool, error) {
	if !isOdysseyRewardRole(role) {
		return role, false, nil
	}
	return store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyArmorEvent, "odyssey-source-armor-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return applyOdysseyArmor(current, wear)
	})
}

func applyOdysseyWeaponBox(role storage.Character) (json.RawMessage, json.RawMessage, error) {
	if !isOdysseyRewardRole(role) || role.ConfigVersion != odysseySource() {
		return nil, nil, fmt.Errorf("Odyssey weapon box requires source mode")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	occupied := map[uint16]bool{}
	for _, v := range b.Items {
		occupied[v.Slot] = true
	}
	for _, v := range b.Equipment {
		occupied[v.Slot] = true
	}
	var slot uint16
	for n := uint16(65); n <= 120; n++ {
		if !occupied[n] {
			slot = n
			break
		}
	}
	if slot == 0 {
		return nil, nil, fmt.Errorf("consumable bag full; weapon box remains owed")
	}
	b.Items = append(b.Items, inventory.BagItem{Slot: slot, Template: 10417789, Amount: 1})
	raw, e := inventory.SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]any{"create_reward": 10417791, "template": 10417789, "quantity": 1, "slot": slot, "before": role.State, "after": raw, "selection_settled": false})
	return raw, receipt, e
}

func grantOdysseyWeaponBox(ctx context.Context, store *storage.Store, role storage.Character) (storage.Character, bool, error) {
	if !isOdysseyRewardRole(role) {
		return role, false, nil
	}
	return store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyWeaponBoxEvent, "odyssey-source-weapon-box-v1", applyOdysseyWeaponBox)
}

// Source create reward 10417791 has three lines in its [stackable] block:
//
//	10417789 1   10417790 1   10418028 30
//
// The first two are settled by applyOdysseyWeaponBox / applyOdysseyArmor; the
// 30 odyssey-only recovery potions were never awarded (手册 P3 子项 1).
const (
	odysseyCreatePotion      = uint32(10418028)
	odysseyCreatePotionCount = uint32(30)
)

// applyOdysseyCreatePotion 发放创建补给里的 30 瓶专属恢复药水。
//
// 走 Bag.Add 而不是照 applyOdysseyWeaponBox 手写槽位：药水是 [waste] 可叠加物，
// 角色包里往往已经有几十瓶（初始补给一路发到 73 个），必须并进同一叠；手写
// "找一个空格"会在每次重试时多占一格，30 个也只落一格。
func applyOdysseyCreatePotion(role storage.Character, cat catalog.LootCatalog, rules inventory.BagRules) (json.RawMessage, json.RawMessage, error) {
	if !isOdysseyRewardRole(role) || role.ConfigVersion != odysseySource() {
		return nil, nil, fmt.Errorf("Odyssey create potion requires source mode")
	}
	if cat.Source.Checksum != odysseySource() || rules.Source != odysseySource() {
		return nil, nil, fmt.Errorf("Odyssey create potion requires matching source catalogs")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	b, slot, e := b.Add(cat, rules, odysseyCreatePotion, odysseyCreatePotionCount)
	if e != nil {
		return nil, nil, e
	}
	raw, e := inventory.SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]any{
		"create_reward": 10417791,
		"template":      odysseyCreatePotion,
		"quantity":      odysseyCreatePotionCount,
		"slot":          slot,
		"before":        role.State,
		"after":         raw,
	})
	return raw, receipt, e
}

// grantOdysseyCreatePotion 用独立事件键结算药水，与武器盒/防具盒互不干扰。
// 满包时 Bag.Add 会报错，事件不落库 ⇒ 下次登录重试（与另外两项同策略）。
func grantOdysseyCreatePotion(ctx context.Context, store *storage.Store, cat catalog.LootCatalog, rules inventory.BagRules, role storage.Character) (storage.Character, bool, error) {
	if !isOdysseyRewardRole(role) {
		return role, false, nil
	}
	return store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyCreatePotionEvent, "odyssey-source-create-potion-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return applyOdysseyCreatePotion(current, cat, rules)
	})
}
