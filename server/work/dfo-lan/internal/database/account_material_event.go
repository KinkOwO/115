package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CommitAccountMaterialEvent applies an account-scoped material change exactly
// once.
//
// CommitAccountMaterialSweep cannot serve a spend: it is safe to retry only
// because it is state-derived - once swept there is nothing left to move - while
// replaying a spend would deduct the same cubes twice. This variant therefore
// carries the same receipt CommitCharacterEvent uses, keyed by the acting
// character, and locks the account material row alongside the character so two
// characters on one account cannot interleave.
//
// The returned bool reports whether the change was applied (false on an
// idempotent replay). The returned counts are always the post-commit storage
// content, so a replay can still answer the client authoritatively.
// CommitAccountMaterialEventTx 与 CommitAccountMaterialEvent 相同，但把事务句柄交给 apply。
//
// 与 CommitCharacterEventTx 同一动机：NPC 商店的**材料支付**路径也要在同一事务里
// 校验并记录限购（多数限购商品是 account/accumulate 的材料货）。
func (s *Store) CommitAccountMaterialEventTx(ctx context.Context, account, id int64, version, key, model string,
	apply func(*Tx, Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (Character, json.RawMessage, bool, error) {
	if apply == nil {
		return Character{}, nil, false, fmt.Errorf("account material event 缺少处理函数")
	}
	return s.commitAccountMaterialEvent(ctx, account, id, version, key, model, apply, nil)
}

func (s *Store) CommitAccountMaterialEvent(ctx context.Context, account, id int64, version, key, model string, apply func(Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (Character, json.RawMessage, bool, error) {
	return s.commitAccountMaterialEvent(ctx, account, id, version, key, model, nil, apply)
}

func (s *Store) commitAccountMaterialEvent(ctx context.Context, account, id int64, version, key, model string,
	txApply func(*Tx, Character, json.RawMessage) (json.RawMessage, json.RawMessage, error),
	apply func(Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (Character, json.RawMessage, bool, error) {
	var role Character
	decoded, e := hex.DecodeString(version)
	if e != nil || len(decoded) != 32 || key == "" || len(key) > 200 || model == "" || len(model) > 100 || (apply == nil && txApply == nil) {
		return role, nil, false, fmt.Errorf("invalid account material event")
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return role, nil, false, e
	}
	defer tx.rollback(ctx)
	// Callback operations may share account state across characters.
	// Lock accounts before characters, matching roster/archival order.
	if txApply != nil {
		if _, e = tx.queries().LockAccountState(ctx, account); e != nil {
			return role, nil, false, e
		}
	}
	role, e = lockCharacter(ctx, tx, account, id)
	if e != nil {
		return role, nil, false, e
	}
	if role.ConfigVersion != version {
		return role, nil, false, fmt.Errorf("account material event source mismatch")
	}
	if e = tx.queries().InitializeAccountMaterials(ctx, account); e != nil {
		return role, nil, false, e
	}
	counts, e := tx.queries().LockAccountMaterials(ctx, account)
	if e != nil {
		return role, nil, false, e
	}
	prior, e := tx.queries().CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: id, EventKey: key})
	if e == nil {
		if prior != model {
			return role, nil, false, fmt.Errorf("account material event model mismatch")
		}
		// Replayed request: the deduction already happened. Hand back the state
		// as it stands so the caller can still acknowledge with real counts.
		return role, counts, false, tx.commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return role, nil, false, e
	}
	var state, updated json.RawMessage
	if txApply != nil {
		state, updated, e = txApply(newTx(tx.queries(), account, id), role, counts)
	} else {
		state, updated, e = apply(role, counts)
	}
	if e != nil {
		return role, nil, false, e
	}
	if !json.Valid(state) || !json.Valid(updated) {
		return role, nil, false, fmt.Errorf("invalid account material JSON")
	}
	if e = tx.queries().RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: key, ConfigVersion: version, Model: model, Outcome: updated}); e != nil {
		return role, nil, false, e
	}
	if e = tx.queries().UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); e != nil {
		return role, nil, false, e
	}
	if e = tx.queries().SaveAccountMaterials(ctx, sqlcgen.SaveAccountMaterialsParams{AccountID: account, Counts: updated}); e != nil {
		return role, nil, false, e
	}
	if e = tx.commit(ctx); e != nil {
		return role, nil, false, e
	}
	role.State = state
	return role, updated, true, nil
}
