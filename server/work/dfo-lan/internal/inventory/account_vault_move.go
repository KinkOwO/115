package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

func applyAccountVaultGold(bag Bag, saved storage.AccountVaultState, rules AccountVaultRules, amount uint32, deposit bool) (Bag, storage.AccountVaultState, error) {
	if amount == 0 {
		return bag, saved, fmt.Errorf("金币金额不能为零")
	}
	limit, err := rules.GoldLimit(saved.Slots)
	if err != nil {
		return bag, saved, err
	}
	if deposit {
		if bag.Gold < amount || uint64(saved.Gold)+uint64(amount) > uint64(limit) {
			return bag, saved, fmt.Errorf("背包金币不足或账号金库金币已达上限")
		}
		bag.Gold -= amount
		saved.Gold += amount
	} else {
		if saved.Gold < amount || uint64(bag.Gold)+uint64(amount) > math.MaxUint32 {
			return bag, saved, fmt.Errorf("账号金库金币不足或背包金币溢出")
		}
		saved.Gold -= amount
		bag.Gold += amount
	}
	return bag, saved, nil
}

func moveAccountVaultGold(role storage.Character, saved storage.AccountVaultState, rules AccountVaultRules, amount uint32, deposit bool) (json.RawMessage, storage.AccountVaultState, error) {
	if saved.Slots == 0 {
		return nil, saved, fmt.Errorf("账号金库尚未开通")
	}
	bag, err := ReadBag(role.State)
	if err != nil {
		return nil, saved, err
	}
	bag, saved, err = applyAccountVaultGold(bag, saved, rules, amount, deposit)
	if err != nil {
		return nil, saved, err
	}
	state, err := SaveBag(role.State, bag)
	return state, saved, err
}

func DepositAccountVaultGold(role storage.Character, saved storage.AccountVaultState, rules AccountVaultRules, amount uint32) (json.RawMessage, storage.AccountVaultState, error) {
	return moveAccountVaultGold(role, saved, rules, amount, true)
}
func WithdrawAccountVaultGold(role storage.Character, saved storage.AccountVaultState, rules AccountVaultRules, amount uint32) (json.RawMessage, storage.AccountVaultState, error) {
	return moveAccountVaultGold(role, saved, rules, amount, false)
}

func AccountVaultGoldMove(r protocol.ItemMoveRequest) (bool, bool) {
	if r.Count == 0 || r.Selection != 0xffffffff || r.Extra != 0 || r.Flags != [3]byte{} {
		return false, false
	}
	if r.SourceList == 0 && r.SourceSlot == 0 && r.SourceItem == 0 && r.DestinationList == 12 && r.DestinationItem == 0 {
		return true, true
	}
	if r.SourceList == 12 && r.SourceSlot == 0 && r.SourceItem == 0 && r.DestinationList == 0 && r.DestinationSlot == 0 && r.DestinationItem == 0 {
		return true, false
	}
	return false, false
}

// 账号金库只接受可交易或账号绑定实例；不会借共享存储把角色绑定变成可转移。
func accountVaultItemAllowed(item VaultItem, items catalog.LootCatalog, equipment *EquipmentCatalog) error {
	if !item.IsEquip {
		def, ok := items.Items[item.Template]
		if !ok || def.Kind != "stackable" || strings.Contains(def.StackableType, "quest") {
			return fmt.Errorf("物品不属于可存入账号金库的普通物品")
		}
		characterBound := false
		contents := false
		for i, token := range def.Script.Cells {
			if contents && (token.Type != 6 || token.Text != "gift") && token.Text != "[/impossible contents]" {
				return fmt.Errorf("物品包含尚未支持的礼包内容物限制")
			}
			if token.Type != 3 {
				continue
			}
			switch token.Text {
			case "[attach type]":
				if i+1 < len(def.Script.Cells) {
					attach := def.Script.Cells[i+1].Text
					characterBound = attach == "[character]"
				}
			case "[impossible contents]":
				contents = true
			case "[/impossible contents]":
				contents = false
			case "[cannot store]", "[cannot put in cargo]":
				return fmt.Errorf("物品包含尚不支持的金库存放限制")
			}
		}
		if characterBound {
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

func MoveAccountVault(role storage.Character, saved storage.AccountVaultState, rules BagRules, items catalog.LootCatalog, equipment *EquipmentCatalog, request protocol.ItemMoveRequest, goldRules ...AccountVaultRules) (json.RawMessage, storage.AccountVaultState, uint32, error) {
	fail := func(reason string) (json.RawMessage, storage.AccountVaultState, uint32, error) {
		return nil, saved, 0, fmt.Errorf("%s", reason)
	}
	if saved.Slots == 0 || (request.SourceList != 12 && request.DestinationList != 12) || (request.SourceList != 0 && request.SourceList != 12) || (request.DestinationList != 0 && request.DestinationList != 12) || request.Extra != 0 || request.Selection != 0xffffffff || request.Flags != [3]byte{} {
		return fail("账号金库未开通或存取请求无效")
	}
	if gold, deposit := AccountVaultGoldMove(request); gold {
		if len(goldRules) == 0 {
			return fail("账号金库金币规则缺失")
		}
		state, next, err := moveAccountVaultGold(role, saved, goldRules[0], request.Count, deposit)
		return state, next, request.Count, err
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
