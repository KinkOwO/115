package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"time"
)

// moveVault 处理背包与两个角色金库，以及同一金库内部的 CMD19 移动。
func (w *worldSession) moveVault(rules inventory.BagRules, r protocol.ItemMoveRequest) ([]outboundPacket, bool, error) {
	if (r.SourceList == 2 && r.DestinationList == 45) || (r.SourceList == 45 && r.DestinationList == 2) {
		plan, err := w.moveVaultCross(rules, r)
		return plan, true, err
	}
	space := byte(2)
	if r.SourceList == 45 || r.DestinationList == 45 {
		space = 45
	} else if r.SourceList != 2 && r.DestinationList != 2 {
		return nil, false, nil
	}
	if (r.SourceList != 0 && r.SourceList != space) || (r.DestinationList != 0 && r.DestinationList != 7 && r.DestinationList != space) {
		return nil, true, fmt.Errorf("个人金库不支持此容器组合：%d → %d", r.SourceList, r.DestinationList)
	}
	if r.DestinationList == 7 && r.SourceList != 2 {
		return nil, true, fmt.Errorf("宠物栏金库取出尚未确认容器组合：%d → %d", r.SourceList, r.DestinationList)
	}
	if w == nil || w.role.ID == 0 || w.vault == nil || w.vault.Store == nil {
		return nil, true, fmt.Errorf("vault move before character or vault service initialization")
	}

	var movedCount uint32
	var newBag inventory.Bag
	var newVault inventory.Vault

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// 领域移动复用同一套规则；只在事务选定的金库内部规范化编号。
	// 回执和客户端更新仍使用原始的 2/45，禁止让两个金库共用存档。
	move := r
	if move.SourceList == space {
		move.SourceList = 2
	}
	if move.DestinationList == space {
		move.DestinationList = 2
	}

	savedRole, _, err := w.store.CommitVaultMove(ctx, w.account, w.role.ID,
		func(curRole database.Character, curVaultState database.VaultState) (json.RawMessage, json.RawMessage, error) {
			if curVaultState.ConfigVersion != w.vault.Rules.SourceSHA256 {
				return nil, nil, fmt.Errorf("个人金库存档版本不匹配")
			}
			b, e := inventory.ReadBag(curRole.State)
			if e != nil {
				return nil, nil, e
			}
			v, e := inventory.ReadVault(curVaultState)
			if e != nil {
				return nil, nil, e
			}
			// 使用锁内存档核对两个槽位，旧请求不能移动后来放入的另一件物品。
			templateAt := func(list byte, slot uint16) uint32 {
				if list == 2 {
					if item := v.ItemAt(slot); item != nil {
						return item.Template
					}
					return 0
				}
				if list == 7 {
					for _, item := range b.Special[7] {
						if item.Slot == slot {
							return item.Template
						}
					}
					for _, item := range b.PetItems {
						if item.Slot == slot {
							return item.Template
						}
					}
					return 0
				}
				for _, item := range b.Items {
					if item.Slot == slot {
						return item.Template
					}
				}
				for _, item := range b.Equipment {
					if item.Slot == slot {
						return item.Template
					}
				}
				return 0
			}
			if templateAt(move.SourceList, move.SourceSlot) != move.SourceItem || templateAt(move.DestinationList, move.DestinationSlot) != move.DestinationItem {
				return nil, nil, fmt.Errorf("金库移动的槽位物品已经改变")
			}
			var moveErr error
			moveRules := rules
			limitTemplate := move.SourceItem
			if limitTemplate == 0 {
				limitTemplate = move.DestinationItem
			}
			moveRules.MissingStackLimit = inventory.StackLimitForTemplate(w.vault.Catalog, rules, limitTemplate)
			newBag, newVault, movedCount, moveErr = inventory.MoveVaultItem(b, v, moveRules, move, w.vault.Equipment)
			if moveErr != nil {
				return nil, nil, moveErr
			}
			savedBagRaw, e := inventory.SaveBag(curRole.State, newBag)
			if e != nil {
				return nil, nil, e
			}
			savedVaultRaw, e := inventory.SaveVault(newVault)
			if e != nil {
				return nil, nil, e
			}
			// 存档提交前确认两个容器均可编码，失败时不扣除任何物品。
			if _, e = protocol.PersonalVaultSpace(space, newVault.Slots, newVault.Rows()); e != nil {
				return nil, nil, e
			}
			if _, e = protocol.InventoryRestore(newBag.Rows(), newBag.Expansion); e != nil {
				return nil, nil, e
			}
			if move.DestinationList == 7 {
				if _, e = inventory.PetContainerBody(newBag, true); e != nil {
					return nil, nil, e
				}
			}
			return savedBagRaw, savedVaultRaw, nil
		}, space)
	if err != nil {
		return nil, true, err
	}

	w.role = savedRole

	plan := []outboundPacket{
		{"vault_move_committed", 1, 19, protocol.ItemMoveSuccess(r, movedCount)},
	}

	if r.SourceList == 0 || r.DestinationList == 0 {
		bagUpdate, e := protocol.InventoryRestore(newBag.Rows(), newBag.Expansion)
		if e != nil {
			return nil, true, e
		}
		plan = append(plan, outboundPacket{"vault_bag_restored", 0, 13, bagUpdate})
	}
	if r.DestinationList == 7 {
		petBody, e := inventory.PetContainerBody(newBag, true)
		if e != nil {
			return nil, true, e
		}
		plan = append(plan, outboundPacket{"vault_pet_container_restored", 0, 13, petBody})
		creatures, e := inventory.CreatureListPayload(savedRole.State)
		if e != nil {
			return nil, true, e
		}
		plan = append(plan, outboundPacket{"vault_creature_list_updated", 0, 105, creatures})
	}

	if r.SourceList == space || r.DestinationList == space {
		vaultRestore, e := protocol.PersonalVaultSpace(space, newVault.Slots, newVault.Rows())
		if e != nil {
			return nil, true, e
		}
		plan = append(plan, outboundPacket{"vault_restored", 0, 13, vaultRestore})
	}

	return plan, true, nil
}

func (w *worldSession) moveVaultCross(rules inventory.BagRules, r protocol.ItemMoveRequest) ([]outboundPacket, error) {
	if w == nil || w.vault == nil || w.vault.Store == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("个人金库尚未就绪")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var moved uint32
	_, first, second, err := w.store.CommitVaultCrossMove(ctx, w.account, w.role.ID, func(role database.Character, a, b database.VaultState) (json.RawMessage, json.RawMessage, json.RawMessage, error) {
		if a.ConfigVersion != w.vault.Rules.SourceSHA256 || b.ConfigVersion != w.vault.Rules.SourceSHA256 {
			return nil, nil, nil, fmt.Errorf("个人金库存档版本不匹配")
		}
		v1, e := inventory.ReadExtendedVault(a)
		if e != nil {
			return nil, nil, nil, e
		}
		v2, e := inventory.ReadExtendedVault(b)
		if e != nil {
			return nil, nil, nil, e
		}
		at := func(space byte, slot uint16) uint32 {
			v := v1
			if space == 45 {
				v = v2
			}
			if item := v.ItemAt(slot); item != nil {
				return item.Template
			}
			return 0
		}
		if at(r.SourceList, r.SourceSlot) != r.SourceItem || at(r.DestinationList, r.DestinationSlot) != r.DestinationItem {
			return nil, nil, nil, fmt.Errorf("跨库槽位物品已经改变")
		}
		limitTemplate := r.SourceItem
		if limitTemplate == 0 {
			limitTemplate = r.DestinationItem
		}
		v1, v2, moved, e = inventory.MoveVaultCross(v1, v2, inventory.StackLimitForTemplate(w.vault.Catalog, rules, limitTemplate), r)
		if e != nil {
			return nil, nil, nil, e
		}
		x, y := inventory.SaveVault(v1)
		if y != nil {
			return nil, nil, nil, y
		}
		z, y := inventory.SaveVault(v2)
		if y != nil {
			return nil, nil, nil, y
		}
		if _, y = protocol.PersonalVaultSpace(2, v1.Slots, v1.Rows()); y != nil {
			return nil, nil, nil, y
		}
		if _, y = protocol.PersonalVaultSpace(45, v2.Slots, v2.Rows()); y != nil {
			return nil, nil, nil, y
		}
		return role.State, x, z, nil
	})
	if err != nil {
		return nil, err
	}
	v1, err := inventory.VaultPayload(first, 2)
	if err != nil {
		return nil, err
	}
	v2, err := inventory.VaultPayload(second, 45)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"vault_cross_move_committed", 1, 19, protocol.ItemMoveSuccess(r, moved)}, {"vault_cross_vault_restored", 0, 13, v1}, {"vault_cross_cargo_restored", 0, 13, v2}}, nil
}

func (w *worldSession) sortVaultSpace(space byte) ([]outboundPacket, error) {
	if w == nil || w.vault == nil || w.vault.Store == nil || (space != 2 && space != 45) {
		return nil, fmt.Errorf("个人金库排序不可用")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, saved, err := w.store.CommitVaultMove(ctx, w.account, w.role.ID, func(role database.Character, state database.VaultState) (json.RawMessage, json.RawMessage, error) {
		if state.ConfigVersion != w.vault.Rules.SourceSHA256 {
			return nil, nil, fmt.Errorf("个人金库存档版本不匹配")
		}
		v, e := inventory.ReadExtendedVault(state)
		if e != nil {
			return nil, nil, e
		}
		v = inventory.SortVaultSpace(v)
		items, e := inventory.SaveVault(v)
		if e != nil {
			return nil, nil, e
		}
		if _, e = protocol.PersonalVaultSpace(space, v.Slots, v.Rows()); e != nil {
			return nil, nil, e
		}
		return role.State, items, nil
	}, space)
	if err != nil {
		return nil, err
	}
	body, err := inventory.VaultPayload(saved, space)
	if err != nil {
		return nil, err
	}
	name := "vault_sorted"
	if space == 45 {
		name = "cargo2_sorted"
	}
	return []outboundPacket{{name, 0, 13, body}}, nil
}
