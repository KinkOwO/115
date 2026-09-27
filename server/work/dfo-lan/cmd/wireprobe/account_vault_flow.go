package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
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
	rows := materials.Rows(inventory.AccountMaterialSpace)
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
	body, err = radiantSoulSnapshot(materials)
	if err != nil {
		return nil, err
	}
	packets = append(packets, outboundPacket{"账号金库光辉灵魂已同步", 0, 13, body})
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

func (w *worldSession) moveAccountVault(service *inventory.WearService, r protocol.ItemMoveRequest, key string) ([]outboundPacket, error) {
	if w.vault == nil || w.vault.Store == nil || w.vault.Rules.Account == nil || w.activeDungeon != nil {
		return nil, fmt.Errorf("当前不能操作账号金库")
	}
	if r.SourceList == 2 || r.SourceList == 45 || r.DestinationList == 2 || r.DestinationList == 45 {
		return w.moveAccountVaultCross(service, r, key)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var count uint32
	saved, _, savedVault, _, err := w.vault.Store.CommitAccountVault(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, 19,
		func(role storage.Character, materials json.RawMessage, vault storage.AccountVaultState) (json.RawMessage, json.RawMessage, storage.AccountVaultState, error) {
			state, next, moved, err := inventory.MoveAccountVault(role, vault, service.BagRules, w.vault.Catalog, service.Catalog, r, *w.vault.Rules.Account)
			if err != nil {
				return nil, nil, vault, err
			}
			count = moved
			role.State = state
			if _, err = inventory.AccountVaultPayload(next, *w.vault.Rules.Account); err != nil {
				return nil, nil, vault, err
			}
			newBag, err := inventory.ReadBag(state)
			if err != nil {
				return nil, nil, vault, err
			}
			if _, err = protocol.InventoryRestore(newBag.Rows(), newBag.Expansion); err != nil {
				return nil, nil, vault, err
			}
			return state, materials, next, nil
		})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	packets := []outboundPacket{{"账号金库存取成功", 1, 19, protocol.ItemMoveSuccess(r, count)}}
	vaultBody, err := inventory.AccountVaultPayload(savedVault, *w.vault.Rules.Account)
	if err != nil {
		return nil, err
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	bagBody, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return nil, err
	}
	packets = append(packets, outboundPacket{"账号金库完整刷新", 0, 13, vaultBody}, outboundPacket{"账号金库背包完整刷新", 0, 13, bagBody})
	return packets, nil
}

func (w *worldSession) sortAccountVaultCmd() ([]outboundPacket, error) {
	if w == nil || w.vault == nil || w.vault.Rules.Account == nil || w.activeDungeon != nil {
		return nil, fmt.Errorf("账号金库排序不可用")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	v, err := w.vault.Store.CommitAccountVaultSort(ctx, w.account, w.role.ID, inventory.SortAccountVaultItems)
	if err != nil {
		return nil, err
	}
	body, err := inventory.AccountVaultPayload(v, *w.vault.Rules.Account)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"account_vault_sorted", 1, 20, []byte{}}, {"account_vault_list", 0, 13, body}}, nil
}

func (w *worldSession) moveAccountVaultCross(service *inventory.WearService, r protocol.ItemMoveRequest, key string) ([]outboundPacket, error) {
	space := r.SourceList
	if space == 12 {
		space = r.DestinationList
	}
	if space != 2 && space != 45 {
		return nil, fmt.Errorf("个人金库容器无效")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var moved uint32
	saved, shared, personal, _, err := w.vault.Store.CommitAccountVaultCrossMove(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, space, func(role storage.Character, a storage.AccountVaultState, p storage.VaultState) (json.RawMessage, storage.AccountVaultState, json.RawMessage, error) {
		if p.ConfigVersion != w.vault.Rules.SourceSHA256 {
			return nil, a, nil, fmt.Errorf("个人金库存档版本不匹配")
		}
		if a.Slots == 0 {
			return nil, a, nil, fmt.Errorf("账号金库尚未开通")
		}
		av, e := inventory.ReadAccountVault(a)
		if e != nil {
			return nil, a, nil, e
		}
		pv, e := inventory.ReadExtendedVault(p)
		if e != nil {
			return nil, a, nil, e
		}
		at := func(list byte, slot uint16) uint32 {
			v := av
			if list != 12 {
				v = pv
			}
			if item := v.ItemAt(slot); item != nil {
				return item.Template
			}
			return 0
		}
		if at(r.SourceList, r.SourceSlot) != r.SourceItem || at(r.DestinationList, r.DestinationSlot) != r.DestinationItem {
			return nil, a, nil, fmt.Errorf("账号金库跨库移动的槽位物品已经改变")
		}
		limitTemplate := r.SourceItem
		if limitTemplate == 0 {
			limitTemplate = r.DestinationItem
		}
		av, pv, moved, e = inventory.MoveAccountVaultCross(av, pv, inventory.StackLimitForTemplate(w.vault.Catalog, service.BagRules, limitTemplate), r, w.vault.Catalog, service.Catalog)
		if e != nil {
			return nil, a, nil, e
		}
		a.Items, e = inventory.SaveVault(av)
		if e != nil {
			return nil, a, nil, e
		}
		items, e := inventory.SaveVault(pv)
		if e != nil {
			return nil, a, nil, e
		}
		if _, e = inventory.AccountVaultPayload(a, *w.vault.Rules.Account); e != nil {
			return nil, a, nil, e
		}
		if _, e = protocol.PersonalVaultSpace(space, pv.Slots, pv.Rows()); e != nil {
			return nil, a, nil, e
		}
		return role.State, a, items, nil
	})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	packets := []outboundPacket{{"账号金库跨库移动成功", 1, 19, protocol.ItemMoveSuccess(r, moved)}}
	// Replays still receive authoritative snapshots; the transaction makes no
	// second write, and the client can clear its pending transfer state.
	body, err := inventory.VaultPayload(personal, space)
	if err != nil {
		return nil, err
	}
	name := "vault_cross_vault_restored"
	if space == 45 {
		name = "vault_cross_cargo_restored"
	}
	packets = append(packets, outboundPacket{name, 0, 13, body})
	body, err = inventory.AccountVaultPayload(shared, *w.vault.Rules.Account)
	if err != nil {
		return nil, err
	}
	return append(packets, outboundPacket{"账号金库存取快照", 0, 13, body}), nil
}

func parseAccountVaultGoldWithdraw(plain []byte) (uint32, error) {
	if len(plain) != 16 || !bytes.Equal(plain[4:], make([]byte, 12)) {
		return 0, fmt.Errorf("金币取出请求体无效")
	}
	return binary.LittleEndian.Uint32(plain[:4]), nil
}

func (w *worldSession) accountVaultGoldPackets(role storage.Character, v storage.AccountVaultState) ([]outboundPacket, error) {
	body, err := inventory.AccountVaultPayload(v, *w.vault.Rules.Account)
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{{"账号金库金币快照", 0, 13, body}}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	body, err = protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return nil, err
	}
	return append(packets, outboundPacket{"账号金库金币背包已同步", 0, 13, body}), nil
}

func (w *worldSession) changeAccountVaultGold(ctx context.Context, opcode uint16, plain, raw, keys []byte, prefix string) ([]outboundPacket, error) {
	if w == nil || w.vault == nil || w.vault.Store == nil || w.vault.Rules.Account == nil || w.activeDungeon != nil || w.role.ID == 0 || prefix == "" {
		return nil, fmt.Errorf("账号金库金币操作不可用")
	}
	var amount uint32
	if opcode == 307 {
		if len(plain) != 8 || !bytes.Equal(plain[4:], make([]byte, 4)) {
			return nil, fmt.Errorf("金币存入请求体无效")
		}
		amount = binary.LittleEndian.Uint32(plain[:4])
	} else if opcode == 308 {
		var err error
		amount, err = parseAccountVaultGoldWithdraw(plain)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("金币命令无效")
	}
	if amount == 0 {
		return nil, fmt.Errorf("金币金额不能为零")
	}
	key := fmt.Sprintf("account-vault-gold:%s:%x", prefix, sha256.Sum256(raw))
	var plan []outboundPacket
	saved, _, _, applied, err := w.vault.Store.CommitAccountVault(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, opcode, func(role storage.Character, materials json.RawMessage, v storage.AccountVaultState) (json.RawMessage, json.RawMessage, storage.AccountVaultState, error) {
		var state json.RawMessage
		var next storage.AccountVaultState
		var e error
		if opcode == 307 {
			state, next, e = inventory.DepositAccountVaultGold(role, v, *w.vault.Rules.Account, amount)
		} else {
			state, next, e = inventory.WithdrawAccountVaultGold(role, v, *w.vault.Rules.Account, amount)
		}
		if e != nil {
			return nil, nil, v, e
		}
		role.State = state
		plan, e = w.accountVaultGoldPackets(role, next)
		if e != nil {
			return nil, nil, v, e
		}
		name := "账号金库金币存入成功"
		if opcode == 308 {
			name = "账号金库金币取出成功"
		}
		plan = append([]outboundPacket{{name, 1, opcode, []byte{1}}}, plan...)
		_, e = preparePackets(keys, plan)
		return state, materials, next, e
	})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	if !applied {
		return []outboundPacket{{"账号金库金币重复请求", 1, opcode, []byte{1}}}, nil
	}
	return plan, nil
}
