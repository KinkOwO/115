package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"fmt"
)

type VaultState = inventory.VaultState

func (s *Store) MigrateVault(ctx context.Context) error {
	return s.execMigration(ctx, "0026_vaults.sql")
}

// UpgradeSecondaryVaultCapacity only raises capacity; existing items and
// purchased slots are never rewritten or reduced. Legacy cargo rows are used
// when present, otherwise an 8-slot Safe II advances to the 24-slot baseline.
func (s *Store) UpgradeSecondaryVaultCapacity(ctx context.Context) error {
	return s.execMigration(ctx, "0028_secondary_vault_upgrade.sql")
}

// 固定容器选择命名查询；缺省保持金库 1，旧调用和旧存档均不迁移。
func secondaryPersonalVault(space []byte) (bool, error) {
	if len(space) == 0 || (len(space) == 1 && space[0] == 2) {
		return false, nil
	}
	if len(space) == 1 && space[0] == 45 {
		return true, nil
	}
	return false, errors.New("个人金库容器无效")
}

// The two personal containers share a domain shape but have separate SQL.
// Keep that finite branch here rather than interpolating a table name.
func lockPersonalVault(ctx context.Context, q querySet, id int64, secondary bool) (VaultState, error) {
	var row sqlcgen.LockPrimaryVaultRow
	var err error
	if secondary {
		r, e := q.LockSecondaryVault(ctx, id)
		row, err = sqlcgen.LockPrimaryVaultRow(r), e
	} else {
		row, err = q.LockPrimaryVault(ctx, id)
	}
	return VaultState{Slots: uint16(row.Slots), Items: row.Items, ConfigVersion: row.ConfigVersion}, err
}

func savePersonalVaultItems(ctx context.Context, q querySet, id int64, items json.RawMessage, secondary bool) error {
	if secondary {
		return q.SaveSecondaryVaultItems(ctx, sqlcgen.SaveSecondaryVaultItemsParams{CharacterID: id, Items: items})
	}
	return q.SavePrimaryVaultItems(ctx, sqlcgen.SavePrimaryVaultItemsParams{CharacterID: id, Items: items})
}

// Initialization inserts only absent state; reconnect never resets stored items.
func (s *Store) LoadVault(ctx context.Context, account, id int64, initial uint16, version string, space ...byte) (VaultState, error) {
	var v VaultState
	secondary, e := secondaryPersonalVault(space)
	if e != nil {
		return v, e
	}
	if initial == 0 || len(version) != 64 {
		return v, errors.New("invalid vault source")
	}
	if secondary {
		e = s.queries.EnsureSecondaryVault(ctx, sqlcgen.EnsureSecondaryVaultParams{AccountID: account, CharacterID: id, Slots: int32(initial), ConfigVersion: version})
	} else {
		e = s.queries.EnsurePrimaryVault(ctx, sqlcgen.EnsurePrimaryVaultParams{AccountID: account, CharacterID: id, Slots: int32(initial), ConfigVersion: version})
	}
	if e != nil {
		return v, e
	}
	var row sqlcgen.OwnedPrimaryVaultRow
	if secondary {
		r, err := s.queries.OwnedSecondaryVault(ctx, sqlcgen.OwnedSecondaryVaultParams{AccountID: account, CharacterID: id})
		row, e = sqlcgen.OwnedPrimaryVaultRow(r), err
	} else {
		row, e = s.queries.OwnedPrimaryVault(ctx, sqlcgen.OwnedPrimaryVaultParams{AccountID: account, CharacterID: id})
	}
	v = VaultState{Slots: uint16(row.Slots), Items: row.Items, ConfigVersion: row.ConfigVersion}
	return v, storageError(e)
}

// personalVaultSlotsValid 判定一层个人金库的容量是否落在客户端认可的档位上。
//
// 依据（两处都只认这一条口径，这里不另立第三张表）：
//   - internal/game/protocol/vault.go 的 PersonalVaultUpgradeNotice 只接受
//     8..264 且 (slots-8)%16==0，否则连 NOTI66 扩容通知都发不出去；
//   - configs/pvf-vault-policy.json 的 verified_slots 正是 8,24,...,264 这一串档位。
//
// 这个判断同时管“目标容量”和“建行时的 initial”：Ensure*Vault 会把 initial 原样写进新行，
// 放进来的野值会在数据库 CHECK 上炸，或者建出一个玩法口径之外的容量。
func personalVaultSlotsValid(slots uint16) bool {
	return slots >= 8 && slots <= 264 && (slots-8)%16 == 0
}

// GrantVaultSlots 把一层**个人**金库的容量提升到 slots（只升不降、天然幂等：重复调用是同一固定点）。
//
// 为什么需要它：金库容量在既有代码里只有两条写路径 —— 商城购买
// （internal/database/cash_purchase.go 的 `SavePrimaryVaultSlots`/`SaveSecondaryVaultSlots`，被 Cera 订单事务私有包装）
// 与 LoadVault 的 Ensure（只按 initial 建行）。奖励脚本要给新角色"直接开到大金库"，没有公开入口。
//
// 边界：个人金库口径 slots 必须 8..264 且 (slots-8)%16==0（依据 internal/game/protocol/vault.go 的校验与
// configs/pvf-vault-policy.json 的 verified_slots 上限）；越界返回带数值的错误。secondary=true 表示金库2
// （space 45，表 character_secondary_vaults），false 表示金库1（space 2，表 character_vaults）。
// 建号场景下这两个行可能**还不存在**（唯一建档时机是登录）⇒ 必须先 Ensure 再读现值，只有目标更大才写。
//
// 不做的事：不碰角色状态、背包、金币和 items —— 只动这两张金库表的 slots 一列。
func (s *Store) GrantVaultSlots(ctx context.Context, account, id int64, version string, initial, slots uint16, secondary bool) error {
	if account <= 0 || id <= 0 {
		return fmt.Errorf("个人金库扩容的账号或角色无效: account=%d id=%d", account, id)
	}
	if !personalVaultSlotsValid(initial) {
		return fmt.Errorf("个人金库初始容量 %d 不是合法档位（需 8..264 且 (slots-8)%%16==0）", initial)
	}
	if !personalVaultSlotsValid(slots) {
		return fmt.Errorf("个人金库目标容量 %d 不是合法档位（需 8..264 且 (slots-8)%%16==0）", slots)
	}
	// LoadVault 的既有门禁同宽：config_version 是 64 字符来源哈希，直接写进新行的 NOT NULL 列。
	if len(version) != 64 {
		return fmt.Errorf("个人金库配置版本长度无效: 期望 64，实得 %d", len(version))
	}
	// 校验全部放在事务之前，非法目标一行都不写（调用失败也不该留下部分状态）。
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	// 先 Ensure：行缺失时按 initial 建行（与登录 LoadVault 的建档同形，配置版本取自同一 version）；
	// 行已存在时 ON CONFLICT DO NOTHING 不动它，所以这里不会把玩家已买的容量改回 initial。
	if secondary {
		err = queries.EnsureSecondaryVault(ctx, sqlcgen.EnsureSecondaryVaultParams{AccountID: account, CharacterID: id, Slots: int32(initial), ConfigVersion: version})
	} else {
		err = queries.EnsurePrimaryVault(ctx, sqlcgen.EnsurePrimaryVaultParams{AccountID: account, CharacterID: id, Slots: int32(initial), ConfigVersion: version})
	}
	if err != nil {
		return err
	}
	vault, err := lockPersonalVault(ctx, queries, id, secondary)
	if err != nil {
		// Ensure 的 INSERT ... SELECT 要求角色存在、属于该账号且未删除；读不到行就是这个条件没成立。
		if isNoRows(err) {
			return fmt.Errorf("个人金库所属角色不存在或不属于该账号: account=%d id=%d", account, id)
		}
		return err
	}
	// 只升不降：现值不小于目标就是同一固定点 —— 不写 slots、不算失败，Ensure 的结果照样提交，
	// 这样"首次调用时目标本来就不大于 initial"也留下一条和登录建档完全一致的行。
	if vault.Slots >= slots {
		return tx.commit(ctx)
	}
	// 目标更大才写，且只写 slots 一列（Save*VaultSlots 不碰 items/gold）。
	if secondary {
		err = queries.SaveSecondaryVaultSlots(ctx, sqlcgen.SaveSecondaryVaultSlotsParams{CharacterID: id, Slots: int32(slots)})
	} else {
		err = queries.SavePrimaryVaultSlots(ctx, sqlcgen.SavePrimaryVaultSlotsParams{CharacterID: id, Slots: int32(slots)})
	}
	if err != nil {
		return err
	}
	return tx.commit(ctx)
}

// CommitVaultMove performs an atomic character bag + vault state modification
// under row locks on characters and character_vaults.
func (s *Store) CommitVaultMove(ctx context.Context, account, id int64,
	apply func(role Character, vault VaultState) (newRoleState json.RawMessage, newVaultItems json.RawMessage, err error), space ...byte) (Character, VaultState, error) {
	var role Character
	var vault VaultState
	secondary, e := secondaryPersonalVault(space)
	if e != nil {
		return role, vault, e
	}
	if apply == nil {
		return role, vault, errors.New("nil vault move apply function")
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return role, vault, e
	}
	defer tx.rollback(ctx)

	role, e = lockCharacter(ctx, tx, account, id)
	if e != nil {
		return role, vault, e
	}

	queries := tx.queries()
	vault, e = lockPersonalVault(ctx, queries, id, secondary)
	if e != nil {
		return role, vault, e
	}

	newRoleState, newVaultItems, e := apply(role, vault)
	if e != nil {
		return role, vault, e
	}
	if !json.Valid(newRoleState) || !json.Valid(newVaultItems) {
		return role, vault, errors.New("invalid JSON state in vault move")
	}

	e = tx.queries().UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: newRoleState})
	if e != nil {
		return role, vault, e
	}
	e = savePersonalVaultItems(ctx, queries, id, newVaultItems, secondary)
	if e != nil {
		return role, vault, e
	}

	if e = tx.commit(ctx); e != nil {
		return role, vault, e
	}

	role.State = newRoleState
	vault.Items = newVaultItems
	return role, vault, nil
}

// CommitVaultCrossMove locks the character and both personal vault rows in a
// fixed order, so a failed transfer leaves both inventories unchanged.
func (s *Store) CommitVaultCrossMove(ctx context.Context, account, id int64,
	apply func(Character, VaultState, VaultState) (json.RawMessage, json.RawMessage, json.RawMessage, error),
) (Character, VaultState, VaultState, error) {
	var role Character
	var first, second VaultState
	if apply == nil {
		return role, first, second, errors.New("nil cross-vault apply")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return role, first, second, err
	}
	defer tx.rollback(ctx)
	role, err = lockCharacter(ctx, tx, account, id)
	if err != nil {
		return role, first, second, err
	}
	queries := tx.queries()
	first, err = lockPersonalVault(ctx, queries, id, false)
	if err != nil {
		return role, first, second, err
	}
	second, err = lockPersonalVault(ctx, queries, id, true)
	if err != nil {
		return role, first, second, err
	}
	state, items1, items2, err := apply(role, first, second)
	if err != nil {
		return role, first, second, err
	}
	if !json.Valid(state) || !json.Valid(items1) || !json.Valid(items2) {
		return role, first, second, errors.New("invalid cross-vault JSON")
	}
	if err = tx.queries().UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); err != nil {
		return role, first, second, err
	}
	if err = savePersonalVaultItems(ctx, queries, id, items1, false); err != nil {
		return role, first, second, err
	}
	if err = savePersonalVaultItems(ctx, queries, id, items2, true); err != nil {
		return role, first, second, err
	}
	if err = tx.commit(ctx); err != nil {
		return role, first, second, err
	}
	role.State, first.Items, second.Items = state, items1, items2
	return role, first, second, nil
}
