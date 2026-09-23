package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// moveVault 处理背包与两个角色金库，以及同一金库内部的 CMD19 移动。
func (w *worldSession) moveVault(rules inventory.BagRules, r protocol.ItemMoveRequest) ([]outboundPacket, bool, error) {
	space := byte(2)
	if r.SourceList == 45 || r.DestinationList == 45 {
		space = 45
	} else if r.SourceList != 2 && r.DestinationList != 2 {
		return nil, false, nil
	}
	if (r.SourceList != 0 && r.SourceList != space) || (r.DestinationList != 0 && r.DestinationList != space) {
		return nil, true, fmt.Errorf("个人金库不支持此容器组合：%d → %d", r.SourceList, r.DestinationList)
	}
	if w == nil || w.role.ID == 0 || w.vault == nil || w.vault.Store == nil {
		return nil, true, fmt.Errorf("vault move before character or vault service initialization")
	}

	var movedCount uint32
	var newBag inventory.Bag
	var oldVault inventory.Vault
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

	savedRole, _, err := w.vault.Store.CommitVaultMove(ctx, w.account, w.role.ID,
		func(curRole storage.Character, curVaultState storage.VaultState) (json.RawMessage, json.RawMessage, error) {
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
			oldVault = v
			var moveErr error
			newBag, newVault, movedCount, moveErr = inventory.MoveVaultItem(b, v, rules, move)
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
			if _, e = protocol.InventoryUpdate(newBag.Rows()); e != nil {
				return nil, nil, e
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
		bagUpdate, e := protocol.InventoryUpdate(newBag.Rows())
		if e != nil {
			return nil, true, e
		}
		plan = append(plan, outboundPacket{"vault_bag_updated", 0, 14, bagUpdate})
	}

	if r.SourceList == space || r.DestinationList == space {
		affectedMap := make(map[uint16]bool)
		if r.SourceList == space {
			affectedMap[r.SourceSlot] = true
		}
		if r.DestinationList == space {
			affectedMap[r.DestinationSlot] = true
		}
		for _, it := range oldVault.Items {
			newIt := newVault.ItemAt(it.Slot)
			if newIt == nil || newIt.Template != it.Template || newIt.Amount != it.Amount || newIt.Durability != it.Durability {
				affectedMap[it.Slot] = true
			}
		}
		for _, it := range newVault.Items {
			oldIt := oldVault.ItemAt(it.Slot)
			if oldIt == nil || oldIt.Template != it.Template || oldIt.Amount != it.Amount || oldIt.Durability != it.Durability {
				affectedMap[it.Slot] = true
			}
		}

		var affectedSlots []uint16
		for slot := range affectedMap {
			affectedSlots = append(affectedSlots, slot)
		}
		sort.Slice(affectedSlots, func(i, j int) bool { return affectedSlots[i] < affectedSlots[j] })

		var vaultRows [][protocol.CurrentItemRecordSize]byte
		for _, slot := range affectedSlots {
			if it := newVault.ItemAt(slot); it != nil {
				vaultRows = append(vaultRows, it.Row())
			} else {
				vaultRows = append(vaultRows, protocol.EmptyOrdinaryItem(slot))
			}
		}

		if len(vaultRows) > 0 {
			vaultUpdate, e := protocol.InventorySpaceUpdate(space, vaultRows)
			if e != nil {
				return nil, true, e
			}
			plan = append(plan, outboundPacket{"vault_slots_updated", 0, 14, vaultUpdate})
		}
	}

	return plan, true, nil
}
