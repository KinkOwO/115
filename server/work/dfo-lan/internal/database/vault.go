package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
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
func lockPersonalVault(ctx context.Context, q *sqlcgen.Queries, id int64, secondary bool) (VaultState, error) {
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

func savePersonalVaultItems(ctx context.Context, q *sqlcgen.Queries, id int64, items json.RawMessage, secondary bool) error {
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
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return role, vault, e
	}
	defer tx.Rollback(ctx)

	role, e = lockCharacter(ctx, tx, account, id)
	if e != nil {
		return role, vault, e
	}

	queries := s.queries.WithTx(tx)
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

	e = sqlcgen.New(tx).UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: newRoleState})
	if e != nil {
		return role, vault, e
	}
	e = savePersonalVaultItems(ctx, queries, id, newVaultItems, secondary)
	if e != nil {
		return role, vault, e
	}

	if e = tx.Commit(ctx); e != nil {
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
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return role, first, second, err
	}
	defer tx.Rollback(ctx)
	role, err = lockCharacter(ctx, tx, account, id)
	if err != nil {
		return role, first, second, err
	}
	queries := s.queries.WithTx(tx)
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
	if err = sqlcgen.New(tx).UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); err != nil {
		return role, first, second, err
	}
	if err = savePersonalVaultItems(ctx, queries, id, items1, false); err != nil {
		return role, first, second, err
	}
	if err = savePersonalVaultItems(ctx, queries, id, items2, true); err != nil {
		return role, first, second, err
	}
	if err = tx.Commit(ctx); err != nil {
		return role, first, second, err
	}
	role.State, first.Items, second.Items = state, items1, items2
	return role, first, second, nil
}
