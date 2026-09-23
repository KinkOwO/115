package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"strings"
)

// 账号金库只接受可交易或账号绑定实例；不会借共享存储把角色绑定变成可转移。
func accountVaultItemAllowed(item VaultItem, items catalog.LootCatalog, equipment *EquipmentCatalog) error {
	if !item.IsEquip {
		def, ok := items.Items[item.Template]
		if !ok || def.Kind != "stackable" || strings.Contains(def.StackableType, "quest") {
			return fmt.Errorf("物品不属于可存入账号金库的普通物品")
		}
		allowed := false
		for i, token := range def.Script.Cells {
			if token.Type != 3 {
				continue
			}
			switch token.Text {
			case "[attach type]":
				if i+1 < len(def.Script.Cells) {
					attach := def.Script.Cells[i+1].Text
					allowed = attach == "[free]" || attach == "[account]"
				}
			case "[cannot store]", "[cannot put in cargo]", "[impossible contents]":
				return fmt.Errorf("物品包含尚不支持的金库存放限制")
			}
		}
		if !allowed {
			return fmt.Errorf("角色绑定物品不能存入账号金库")
		}
		return nil
	}
	if equipment == nil || item.Equipment == nil {
		return fmt.Errorf("账号金库装备缺少完整实例或源目录")
	}
	instance := item.Equipment
	if err := instance.ValidateRecord(); err != nil {
		return err
	}
	def, err := equipment.Definition(item.Template)
	if err != nil {
		return err
	}
	attach, kind := def.Fields["[attach type]"], def.Fields["[equipment type]"]
	if len(attach) != 1 || (attach[0].Text != "[free]" && attach[0].Text != "[account]") || len(kind) == 0 || len(instance.AvatarOptions) != 0 || len(instance.AvatarSockets) != 0 {
		return fmt.Errorf("该装备不是可存入账号金库的普通可交易或账号绑定装备")
	}
	switch kind[0].Text {
	case "[avatar]", "[creature]", "[aura]":
		return fmt.Errorf("账号金库不支持此类装备")
	}
	return nil
}

func MoveAccountVault(role storage.Character, saved storage.AccountVaultState, rules BagRules, items catalog.LootCatalog, equipment *EquipmentCatalog, request protocol.ItemMoveRequest) (json.RawMessage, storage.AccountVaultState, uint32, error) {
	fail := func(reason string) (json.RawMessage, storage.AccountVaultState, uint32, error) {
		return nil, saved, 0, fmt.Errorf("%s", reason)
	}
	if saved.Slots == 0 || (request.SourceList != 12 && request.DestinationList != 12) || (request.SourceList != 0 && request.SourceList != 12) || (request.DestinationList != 0 && request.DestinationList != 12) || request.Extra != 0 || request.Selection != 0xffffffff || request.Flags != [3]byte{} {
		return fail("账号金库未开通或存取请求无效")
	}
	bag, err := ReadBag(role.State)
	if err != nil {
		return fail(err.Error())
	}
	vault, err := ReadExtendedVault(storage.VaultState{Slots: saved.Slots, Items: saved.Items})
	if err != nil {
		return fail(err.Error())
	}
	find := func(space byte, slot uint16) *VaultItem {
		if space == 12 {
			return vault.ItemAt(slot)
		}
		for _, row := range bag.Items {
			if row.Slot == slot {
				return &VaultItem{Slot: slot, Template: row.Template, Amount: row.Amount, ExpireTime: row.ExpireTime}
			}
		}
		for _, row := range bag.Equipment {
			if row.Slot == slot {
				return &VaultItem{Slot: slot, Template: row.Template, Amount: 1, IsEquip: true, Equipment: &row, Durability: row.Durability}
			}
		}
		return nil
	}
	from, to := find(request.SourceList, request.SourceSlot), find(request.DestinationList, request.DestinationSlot)
	template := func(item *VaultItem) uint32 {
		if item == nil {
			return 0
		}
		return item.Template
	}
	if template(from) != request.SourceItem || template(to) != request.DestinationItem {
		return fail("账号金库物品槽位已经改变")
	}
	if from == nil && request.SourceList == 12 && request.DestinationList == 12 && request.Count == 0 {
		from = to
	}
	if from == nil || (!from.IsEquip && request.Count > from.Amount) || (from.IsEquip && request.Count > 1) {
		return fail("账号金库源物品或数量无效")
	}
	if request.SourceList == 0 {
		if err = accountVaultItemAllowed(*from, items, equipment); err != nil {
			return fail(err.Error())
		}
	}
	if !from.IsEquip {
		if def, ok := items.Items[from.Template]; ok {
			rules.MissingStackLimit = stackLimitFor(rules, def.StackableType, def.StackLimit)
			if request.DestinationList == 0 {
				slots, ok := rules.Slots[def.StackableType]
				if !ok {
					return fail("物品的背包分类规则缺失")
				}
				if request.DestinationSlot < slots[0] || request.DestinationSlot > slots[1] {
					return fail("物品不能放入指定背包栏")
				}
			}
		} else {
			return fail("账号金库物品源规则缺失")
		}
	}
	move := request
	if move.SourceList == 12 {
		move.SourceList = 2
	}
	if move.DestinationList == 12 {
		move.DestinationList = 2
	}
	bag, vault, count, err := MoveVaultItem(bag, vault, rules, move)
	if err != nil {
		return fail(err.Error())
	}
	state, err := SaveBag(role.State, bag)
	if err != nil {
		return fail(err.Error())
	}
	saved.Items, err = SaveVault(vault)
	return state, saved, count, err
}
