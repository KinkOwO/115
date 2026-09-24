package inventory

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

type AccountVaultError struct {
	Code   uint16
	Reason string
}

func (e *AccountVaultError) Error() string { return e.Reason }

// 每档六列保持 Etc/AccountCargo.etc 的顺序：容量、金币上限、材料编号、
// 材料数量、金币费用、商城扩容模板。负材料编号表示只能走商城升级。
type AccountVaultRules struct {
	RequiredLevel uint16     `json:"required_level"`
	Upgrades      [][6]int64 `json:"upgrades"`
}

func (r AccountVaultRules) Validate() error {
	if r.RequiredLevel == 0 || len(r.Upgrades) == 0 || len(r.Upgrades) > 40 {
		return fmt.Errorf("账号金库源规则缺失")
	}
	for i, row := range r.Upgrades {
		if row[0] != int64((i+1)*8) || row[1] <= 0 || row[1] > 800000000 || row[2] < -1 || row[2] > 0xffffffff || row[3] < 0 || row[3] > 0xffffffff || row[4] < 0 || row[4] > 0xffffffff || row[5] < -1 || row[5] > 0xffffffff || (row[3] > 0 && row[2] <= 0) {
			return fmt.Errorf("账号金库第 %d 档源规则无效", i+1)
		}
	}
	return nil
}

func (r AccountVaultRules) GoldLimit(slots uint16) (uint32, error) {
	if slots == 0 {
		return 0, nil
	}
	if slots%8 != 0 || int(slots/8) > len(r.Upgrades) {
		return 0, fmt.Errorf("账号金库存档容量不在源规则内")
	}
	return uint32(r.Upgrades[slots/8-1][1]), nil
}

func AccountVaultPayload(v storage.AccountVaultState, rules AccountVaultRules) ([]byte, error) {
	limit, err := rules.GoldLimit(v.Slots)
	if err != nil || v.Gold > limit {
		return nil, fmt.Errorf("账号金库容量或金币超过源上限")
	}
	if v.Slots == 0 {
		var items []VaultItem
		if json.Unmarshal(v.Items, &items) != nil || len(items) != 0 {
			return nil, fmt.Errorf("未开通的账号金库含有物品")
		}
		return protocol.AccountVaultRestore(0, 0, nil)
	}
	items, err := ReadExtendedVault(storage.VaultState{Slots: v.Slots, Items: v.Items})
	if err != nil {
		return nil, err
	}
	return protocol.AccountVaultRestore(v.Slots, v.Gold, items.Rows())
}

func SortAccountVaultItems(v storage.AccountVaultState) (json.RawMessage, error) {
	if v.Slots == 0 {
		return json.RawMessage("[]"), nil
	}
	items, err := ReadExtendedVault(storage.VaultState{Slots: v.Slots, Items: v.Items})
	if err != nil {
		return nil, err
	}
	return SaveVault(SortVaultSpace(items))
}

// 仅按源表处理 CMD305 材料开通与 CMD306 金币升级。负材料编号的商城
// 档位绝不能作为零费用档位处理；服务端不信任客户端显示的材料数量。
func UpgradeAccountVault(role storage.Character, raw json.RawMessage, vault storage.AccountVaultState, rules AccountVaultRules, create bool) (json.RawMessage, json.RawMessage, storage.AccountVaultState, error) {
	fail := func(code uint16, reason string) (json.RawMessage, json.RawMessage, storage.AccountVaultState, error) {
		return nil, nil, vault, &AccountVaultError{code, reason}
	}
	if err := rules.Validate(); err != nil {
		return fail(19, err.Error())
	}
	if create && vault.Slots != 0 {
		return fail(20, "账号金库已经开通，不再扣除材料")
	}
	if !create && vault.Slots == 0 {
		return fail(21, "账号金库尚未开通")
	}
	var state struct {
		Level uint16 `json:"level"`
	}
	if err := json.Unmarshal(role.State, &state); err != nil {
		return fail(19, "角色存档无效")
	}
	if create && uint16(state.Level) < rules.RequiredLevel {
		return fail(14, fmt.Sprintf("开通账号金库要求 %d 级", rules.RequiredLevel))
	}
	if _, err := AccountVaultPayload(vault, rules); err != nil {
		return fail(19, err.Error())
	}
	index := int(vault.Slots / 8)
	if index >= len(rules.Upgrades) {
		return fail(19, "账号金库已经达到最大容量")
	}
	next := rules.Upgrades[index]
	if next[2] < 0 {
		return fail(19, "本档账号金库需要使用商城扩容商品")
	}
	m, err := ReadAccountMaterials(raw)
	if err != nil {
		return fail(22, err.Error())
	}
	bag, err := ReadBag(role.State)
	if err != nil {
		return fail(22, err.Error())
	}
	// 兼容尚未归集材料的旧角色，与共享材料合并后只扣一次。
	bag, deltas, err := SweepAccountMaterials(bag)
	if err != nil {
		return fail(22, err.Error())
	}
	m, err = m.ApplyDeltas(deltas)
	if err != nil {
		return fail(22, err.Error())
	}
	if next[3] > 0 {
		template, amount := uint32(next[2]), uint32(next[3])
		slot, known := AccountMaterialSlot(template)
		if !known || m.Count(template) < amount {
			return fail(22, "账号共享材料不足，无法开通金库")
		}
		m.Counts[slot] -= amount
		if m.Counts[slot] == 0 {
			delete(m.Counts, slot)
		}
	}
	if bag.Gold < uint32(next[4]) {
		return fail(22, "金币不足，无法升级账号金库")
	}
	bag.Gold -= uint32(next[4])
	vault.Slots = uint16(next[0])
	saved, err := SaveBag(role.State, bag)
	if err != nil {
		return fail(22, err.Error())
	}
	materials, err := m.Save()
	return saved, materials, vault, err
}
