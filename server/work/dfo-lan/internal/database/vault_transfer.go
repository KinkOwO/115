package database

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database/sqlcgen"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// CommitVaultTransfer locks character before vault, matching character writes.
// The callback validates and pre-encodes replies before either state is saved.
// Replay returns current states; it never reinstalls an earlier bag snapshot.
func (s *Store) CommitVaultTransfer(ctx context.Context, account, id int64, source, vaultVersion, key string, request []byte,
	apply func(Character, VaultState) (json.RawMessage, json.RawMessage, error)) (Character, VaultState, bool, error) {
	var role Character
	var v VaultState
	fail := func(e error) (Character, VaultState, bool, error) { return Character{}, VaultState{}, false, e }
	a, e := hex.DecodeString(source)
	b, f := hex.DecodeString(vaultVersion)
	if e != nil || f != nil || len(a) != 32 || len(b) != 32 || account <= 0 || id <= 0 || len(key) == 0 || len(key) > 200 || len(request) == 0 || apply == nil {
		return fail(fmt.Errorf("invalid vault transfer"))
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return fail(e)
	}
	defer tx.rollback(ctx)
	role, e = lockCharacter(ctx, tx, account, id)
	if e != nil {
		return fail(e)
	}
	queries := tx.queries()
	v, e = lockPersonalVault(ctx, queries, id, false)
	if e != nil {
		return fail(e)
	}
	if role.ConfigVersion != source || v.ConfigVersion != vaultVersion {
		return fail(fmt.Errorf("vault transfer source mismatch"))
	}
	model := fmt.Sprintf("vault-stack-v1:%x", sha256.Sum256(request))
	prior, e := tx.queries().CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: id, EventKey: key})
	if e == nil {
		if prior != model {
			return fail(fmt.Errorf("vault transfer replay conflict"))
		}
		if e = tx.commit(ctx); e != nil {
			return fail(e)
		}
		return role, v, false, nil
	}
	if !isNoRows(e) {
		return fail(e)
	}
	state, items, e := apply(role, v)
	if e != nil {
		return fail(e)
	}
	var obj map[string]json.RawMessage
	var rows []json.RawMessage
	if json.Unmarshal(state, &obj) != nil || obj == nil || json.Unmarshal(items, &rows) != nil || rows == nil {
		return fail(fmt.Errorf("invalid vault transfer state"))
	}
	if e = tx.queries().UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); e != nil {
		return fail(e)
	}
	if e = savePersonalVaultItems(ctx, queries, id, items, false); e != nil {
		return fail(e)
	}
	if e = queries.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: key, ConfigVersion: source, Model: model, Outcome: json.RawMessage(`{}`)}); e != nil {
		return fail(e)
	}
	if e = tx.commit(ctx); e != nil {
		return fail(e)
	}
	role.State = state
	v.Items = items
	return role, v, true, nil
}
