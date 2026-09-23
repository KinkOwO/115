package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func accountVaultRefusal(err error) []byte {
	var refused *inventory.AccountVaultError
	if errors.As(err, &refused) {
		return protocol.Refusal(refused.Code)
	}
	return protocol.Refusal(22)
}

// 成功应答先让客户端按当前档位执行一次开通/升级，再恢复权威快照。
// 反序会导致 CMD306/0x14529A4A0 把已经同步的容量再升一档。
func accountVaultUpgradePackets(role storage.Character, raw json.RawMessage, vault storage.AccountVaultState, rules inventory.AccountVaultRules, opcode uint16, applied bool) ([]outboundPacket, error) {
	var packets []outboundPacket
	if applied {
		packets = append(packets, outboundPacket{"账号金库升级成功", 1, opcode, []byte{1}})
	}
	body, err := inventory.AccountVaultPayload(vault, rules)
	if err != nil {
		return nil, err
	}
	packets = append(packets, outboundPacket{"账号金库已同步", 0, 13, body})
	materials, err := inventory.ReadAccountMaterials(raw)
	if err != nil {
		return nil, err
	}
	rows := materials.Rows()
	// 开通恰好耗尽 100 个晶块时，也明确同步零数量，不能省略后残留旧显示。
	if materials.Count(3262) == 0 {
		slot, _ := inventory.AccountMaterialSlot(3262)
		rows = append(rows, protocol.OrdinaryItem(slot, 3262, 0))
	}
	body, err = protocol.InventoryRestoreSpace(inventory.AccountMaterialSpace, rows)
	if err != nil {
		return nil, err
	}
	packets = append(packets, outboundPacket{"账号金库费用材料已同步", 0, 13, body})
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	body, err = protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return nil, err
	}
	return append(packets, outboundPacket{"账号金库费用背包已同步", 0, 13, body}), nil
}

func (w *worldSession) upgradeAccountVault(ctx context.Context, opcode uint16, plain, raw, keys []byte, prefix string) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.vault == nil || w.vault.Store == nil || w.vault.Rules.Account == nil || w.activeDungeon != nil {
		return nil, fmt.Errorf("账号金库尚未就绪或角色正在副本中")
	}
	// 0x1469DC8DE/0x1469DC8FA 均只有命令头，实机 CMD305 也确认为空包。
	if (opcode != 305 && opcode != 306) || len(plain) != 0 || prefix == "" {
		return nil, fmt.Errorf("账号金库开通或升级请求无效")
	}
	rules := *w.vault.Rules.Account
	key := fmt.Sprintf("account-vault:%s:%x", prefix, sha256.Sum256(raw))
	saved, materials, vault, applied, err := w.vault.Store.CommitAccountVault(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, opcode,
		func(role storage.Character, materials json.RawMessage, vault storage.AccountVaultState) (json.RawMessage, json.RawMessage, storage.AccountVaultState, error) {
			state, counts, next, err := inventory.UpgradeAccountVault(role, materials, vault, rules, opcode == 305)
			if err != nil {
				return nil, nil, vault, err
			}
			role.State = state
			packets, err := accountVaultUpgradePackets(role, counts, next, rules, opcode, true)
			if err == nil {
				_, err = preparePackets(keys, packets)
			}
			return state, counts, next, err
		})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return accountVaultUpgradePackets(saved, materials, vault, rules, opcode, applied)
}

func accountVaultMovePackets(role storage.Character, vault storage.AccountVaultState, rules inventory.AccountVaultRules) ([]outboundPacket, error) {
	body, err := inventory.AccountVaultPayload(vault, rules)
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{{"账号金库存取快照", 0, 13, body}}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	body, err = protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return nil, err
	}
	return append(packets, outboundPacket{"账号金库存取背包", 0, 13, body}), nil
}

func (w *worldSession) moveAccountVault(service *inventory.WearService, r protocol.ItemMoveRequest, key string) ([]outboundPacket, error) {
	if w.vault == nil || w.vault.Store == nil || w.vault.Rules.Account == nil || w.activeDungeon != nil {
		return nil, fmt.Errorf("当前不能操作账号金库")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count uint32
	var changed [][protocol.CurrentItemRecordSize]byte
	saved, _, vault, applied, err := w.vault.Store.CommitAccountVault(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, 19,
		func(role storage.Character, materials json.RawMessage, vault storage.AccountVaultState) (json.RawMessage, json.RawMessage, storage.AccountVaultState, error) {
			old, err := inventory.ReadExtendedVault(storage.VaultState{Slots: vault.Slots, Items: vault.Items})
			if err != nil {
				return nil, nil, vault, err
			}
			state, next, moved, err := inventory.MoveAccountVault(role, vault, service.BagRules, w.vault.Catalog, service.Catalog, r, uint32(time.Now().Unix()))
			if err != nil {
				return nil, nil, vault, err
			}
			count = moved
			role.State = state
			if _, err = accountVaultMovePackets(role, next, *w.vault.Rules.Account); err != nil {
				return nil, nil, vault, err
			}
			updated, err := inventory.ReadExtendedVault(storage.VaultState{Slots: next.Slots, Items: next.Items})
			if err != nil {
				return nil, nil, vault, err
			}
			changed = updated.Rows()
			for _, item := range old.Items {
				if updated.ItemAt(item.Slot) == nil {
					changed = append(changed, protocol.EmptyOrdinaryItem(item.Slot))
				}
			}
			if _, err = protocol.InventorySpaceUpdate(12, changed); err != nil {
				return nil, nil, vault, err
			}
			return state, materials, next, nil
		})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	packets, err := accountVaultMovePackets(saved, vault, *w.vault.Rules.Account)
	if err != nil {
		return nil, err
	}
	if applied {
		packets = append([]outboundPacket{{"账号金库存取成功", 1, 19, protocol.ItemMoveSuccess(r, count)}}, packets...)
		body, err := protocol.InventorySpaceUpdate(12, changed)
		if err != nil {
			return nil, err
		}
		packets = append(packets, outboundPacket{"账号金库格子刷新", 0, 14, body})
	}
	return packets, nil
}
