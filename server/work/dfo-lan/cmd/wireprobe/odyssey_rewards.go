package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
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

// Keep the existing event keys: persisted receipts must continue to suppress
// repeated grants. Content identifiers and quantities come from native rules.

func isOdysseyRewardRole(role database.Character) bool {
	return character.OdysseyRole(role)
}

func applyOdysseyArmor(role database.Character, wear *workflow.WearService, rewards *catalog.OdysseyCreateRewards) (json.RawMessage, json.RawMessage, error) {
	// ⚠️ 别再往这里加「目录身份」子句：`X.Source.SaveIdentity()` 是**常量**
	// （`pvf.ArchiveSnapshot.SaveIdentity()` 直接返回 `savecontract.Identity()`），
	// 与 `savecontract.Identity()` 比较恒相等 ⇒ 那种子句恒假、等于不写（2026-10-01 清理）。
	// 真要校验目录来源（L3）必须比内层哈希 `.Source.Checksum` —— 见 internal/savecontract 的分级说明。
	if !isOdysseyRewardRole(role) || role.ConfigVersion != savecontract.Identity() || wear == nil || wear.Catalog == nil || rewards == nil || len(rewards.Armor) == 0 {
		return nil, nil, fmt.Errorf("Odyssey armor requires matching character and source catalogs")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	for _, row := range rewards.Armor {
		b, _, e = b.AddEquipment(wear.Catalog, wear.BagRules.EquipmentSlots, row.Template, row.Count)
		if e != nil {
			return nil, nil, e
		}
	}
	raw, e := inventory.SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]any{"create_reward": rewards.Template, "armor_box": rewards.ArmorBox, "templates": rewards.ArmorTemplates(), "before": role.State, "after": raw, "weapon_settled": false, "potion_settled": false})
	return raw, receipt, e
}

func grantOdysseyArmor(ctx context.Context, store *database.Store, wear *workflow.WearService, role database.Character, rewards *catalog.OdysseyCreateRewards) (database.Character, bool, error) {
	if !isOdysseyRewardRole(role) {
		return role, false, nil
	}
	return store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyArmorEvent, "odyssey-source-armor-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return applyOdysseyArmor(current, wear, rewards)
	})
}

func applyOdysseyWeaponBox(role database.Character, rewards *catalog.OdysseyCreateRewards) (json.RawMessage, json.RawMessage, error) {
	if !isOdysseyRewardRole(role) || role.ConfigVersion != savecontract.Identity() || rewards == nil || rewards.Weapon.Template == 0 || rewards.Weapon.Count != 1 {
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
	b.Items = append(b.Items, inventory.BagItem{Slot: slot, Template: rewards.Weapon.Template, Amount: rewards.Weapon.Count})
	raw, e := inventory.SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]any{"create_reward": rewards.Template, "template": rewards.Weapon.Template, "quantity": rewards.Weapon.Count, "slot": slot, "before": role.State, "after": raw, "selection_settled": false})
	return raw, receipt, e
}

func grantOdysseyWeaponBox(ctx context.Context, store *database.Store, role database.Character, rewards *catalog.OdysseyCreateRewards) (database.Character, bool, error) {
	if !isOdysseyRewardRole(role) {
		return role, false, nil
	}
	return store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyWeaponBoxEvent, "odyssey-source-weapon-box-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return applyOdysseyWeaponBox(current, rewards)
	})
}

// Grant the source-defined consumable line through the existing stack-aware
// bag implementation, under its unchanged independent transaction receipt.
func applyOdysseyCreatePotion(role database.Character, cat catalog.LootCatalog, rules inventory.BagRules, rewards *catalog.OdysseyCreateRewards) (json.RawMessage, json.RawMessage, error) {
	if !isOdysseyRewardRole(role) || role.ConfigVersion != savecontract.Identity() || rewards == nil || len(rewards.Supplies) != 1 {
		return nil, nil, fmt.Errorf("Odyssey create potion requires source mode")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	b, slot, e := b.Add(cat, rules, rewards.Supplies[0].Template, rewards.Supplies[0].Count)
	if e != nil {
		return nil, nil, e
	}
	raw, e := inventory.SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]any{
		"create_reward": rewards.Template,
		"template":      rewards.Supplies[0].Template,
		"quantity":      rewards.Supplies[0].Count,
		"slot":          slot,
		"before":        role.State,
		"after":         raw,
	})
	return raw, receipt, e
}

// grantOdysseyCreatePotion 用独立事件键结算药水，与武器盒/防具盒互不干扰。
// 满包时 Bag.Add 会报错，事件不落库 ⇒ 下次登录重试（与另外两项同策略）。
func grantOdysseyCreatePotion(ctx context.Context, store *database.Store, cat catalog.LootCatalog, rules inventory.BagRules, role database.Character, rewards *catalog.OdysseyCreateRewards) (database.Character, bool, error) {
	if !isOdysseyRewardRole(role) {
		return role, false, nil
	}
	return store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyCreatePotionEvent, "odyssey-source-create-potion-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return applyOdysseyCreatePotion(current, cat, rules, rewards)
	})
}
