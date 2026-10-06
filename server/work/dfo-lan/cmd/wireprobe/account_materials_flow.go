package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// Account material storage ("soul storage") orchestration. The 115 client
// pins seventeen templates to list 35 at 363..379 and two radiant souls
// to list 42 at 0..1
// (docs/protocol/next43-account-material-database.md). The server owns the
// counts per account; any of these templates entering a character bag is
// swept into the account storage inside one transaction.

// sweepAccountMaterials moves every account-shared material stack out of the
// role's ordinary bag into the account database. It is a pure state-derived
// migration: once swept, a retry finds nothing left to move.
func sweepAccountMaterials(ctx context.Context, store *database.Store, role database.Character) (database.Character, inventory.AccountMaterials, error) {
	materials := inventory.NewAccountMaterials()
	if store == nil || role.ID == 0 {
		return role, materials, fmt.Errorf("account material storage unavailable")
	}
	saved, counts, e := store.CommitAccountMaterialSweep(ctx, role.AccountID, role.ID, role.ConfigVersion,
		func(current database.Character, raw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			m, e := inventory.ReadAccountMaterials(raw)
			if e != nil {
				return nil, nil, e
			}
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			swept, deltas, e := inventory.SweepAccountMaterials(b)
			if e != nil {
				return nil, nil, e
			}
			if len(deltas) == 0 {
				normalized, err := m.Save()
				return current.State, normalized, err
			}
			if m, e = m.ApplyDeltas(deltas); e != nil {
				return nil, nil, e
			}
			state, e := inventory.SaveBag(current.State, swept)
			if e != nil {
				return nil, nil, e
			}
			updated, e := m.Save()
			if e != nil {
				return nil, nil, e
			}
			return state, updated, nil
		})
	if e != nil {
		return role, materials, e
	}
	materials, e = inventory.ReadAccountMaterials(counts)
	if e != nil {
		return role, materials, e
	}
	saved.WireID = role.WireID
	return saved, materials, nil
}

// accountMaterialSnapshot builds the NOTI13 list35 storage snapshot the
// client expects ahead of the ordinary list0 snapshot. The list0 reader
// harvests slots 363..379 into the account storage pipeline
// (sub_145ADC2A0), so the storage snapshot must arrive first and the bag
// snapshot must follow it.
func accountMaterialSnapshot(m inventory.AccountMaterials) ([]byte, error) {
	return protocol.InventoryRestoreSpace(inventory.AccountMaterialSpace, m.Rows(inventory.AccountMaterialSpace))
}

func radiantSoulSnapshot(m inventory.AccountMaterials) ([]byte, error) {
	return protocol.InventoryRestoreSpace(inventory.RadiantSoulSpace, m.Rows(inventory.RadiantSoulSpace))
}

// accountMaterialRefreshPackets sends list35, list42, then the full list0
// bag snapshot that triggers the client-side harvest.
//
// BUG5（第二十二轮）：副本内（inDungeon）跳过 22KB 的全量背包快照——
// 军团本战斗中技能消耗材料（无色小晶块等，CMD18）每次触发这三连发，
// 22KB 的 InventoryRestore 解析期间客户端打断当前施法（觉醒必断，
// 135312 会话实证每次 CMD18 后三连发）。副本内只发 list35/list42 两个
// 账号库快照（约 2.9KB），背包显示回城后由任意全量刷新校正；官服副本内
// 同命令也只有一条 32B 轻量应答。城镇路径维持三连发不变。
func accountMaterialRefreshPackets(m inventory.AccountMaterials, role database.Character, inDungeon bool) ([]outboundPacket, error) {
	storageBody, e := accountMaterialSnapshot(m)
	if e != nil {
		return nil, e
	}
	soulBody, e := radiantSoulSnapshot(m)
	if e != nil {
		return nil, e
	}
	if inDungeon {
		return []outboundPacket{
			{"account_materials_restored", 0, 13, storageBody},
			{"radiant_souls_restored", 0, 13, soulBody},
		}, nil
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	bagBody, e := protocol.InventoryRestore(b.Rows(), b.Expansion)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{
		{"account_materials_restored", 0, 13, storageBody},
		{"radiant_souls_restored", 0, 13, soulBody},
		{"inventory_restored", 0, 13, bagBody},
	}, nil
}

// storageDestinationSlot remaps a bag slot to the fixed account storage slot
// when the awarded template belongs to list 35, so
// ACK/scene packets point at where the stack actually lives.
func storageDestinationSlot(template uint32, slot uint16) uint16 {
	if space, fixed, ok := inventory.AccountMaterialTarget(template); ok && space == inventory.AccountMaterialSpace {
		return fixed
	}
	return slot
}
