package database

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// Character, both personal vaults and shared vault use the existing fixed lock
// order. Truth level itself fences duplicate stage requests across reconnects.
func (s *Store) UpgradeOathTruth(ctx context.Context, account, id int64, version string, rules *catalog.OathTruthRules, level byte, payment uint32) (Character, bool, error) {
	if rules == nil || len(rules.Source) != 64 || len(version) != 64 || level == 0 || int(level) > rules.MaxLevel {
		return Character{}, false, fmt.Errorf("truth source or target invalid")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return Character{}, false, err
	}
	defer tx.rollback(ctx)
	role, err := lockCharacter(ctx, tx, account, id)
	if err != nil {
		return role, false, err
	}
	if role.ConfigVersion != version {
		return role, false, fmt.Errorf("truth character source mismatch")
	}
	truth, err := inventory.ReadOathTruth(role.State)
	if err != nil {
		return role, false, err
	}
	if err = inventory.ValidateOathTruth(truth, rules); err != nil {
		return role, false, err
	}
	if truth.Level >= level {
		return role, false, tx.commit(ctx)
	}
	q := tx.queries()
	var personal [2]VaultState
	var present [2]bool
	for i := range personal {
		personal[i], err = lockPersonalVault(ctx, q, id, i == 1)
		if err != nil && !isNoRows(err) {
			return role, false, err
		}
		present[i] = err == nil
	}
	if err = q.EnsureAccountVault(ctx, account); err != nil {
		return role, false, err
	}
	shared, err := lockSharedVault(ctx, q, account)
	if err != nil {
		return role, false, err
	}
	state, shared, personal, err := inventory.UpgradeOathTruthWithVaults(role.State, shared, personal, rules, level, payment)
	if err != nil {
		return role, false, err
	}
	if err = q.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); err != nil {
		return role, false, err
	}
	for i := range personal {
		if present[i] {
			if err = savePersonalVaultItems(ctx, q, id, personal[i].Items, i == 1); err != nil {
				return role, false, err
			}
		}
	}
	if err = q.SaveAccountVault(ctx, sqlcgen.SaveAccountVaultParams{AccountID: account, Slots: int32(shared.Slots), Gold: int64(shared.Gold), Items: shared.Items}); err != nil {
		return role, false, err
	}
	key := fmt.Sprintf("oath-truth:%d", level)
	outcome, _ := json.Marshal(map[string]any{"level": level, "payment": payment, "source": rules.RawSHA256})
	if err = q.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: key, ConfigVersion: version, Model: "oath-truth-v1", Outcome: outcome}); err != nil {
		return role, false, err
	}
	if err = tx.commit(ctx); err != nil {
		return role, false, err
	}
	role.State = state
	return role, true, nil
}
func (s *Store) SelectOathTruth(ctx context.Context, account, id int64, version string, rules *catalog.OathTruthRules, slot, option byte) (Character, error) {
	if rules == nil {
		return Character{}, fmt.Errorf("truth rules missing")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return Character{}, err
	}
	defer tx.rollback(ctx)
	role, err := lockCharacter(ctx, tx, account, id)
	if err != nil {
		return role, err
	}
	if role.ConfigVersion != version {
		return role, fmt.Errorf("truth source mismatch")
	}
	state, err := inventory.SelectOathTruth(role.State, rules, slot, option)
	if err != nil {
		return role, err
	}
	if err = tx.queries().UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); err != nil {
		return role, err
	}
	if err = tx.commit(ctx); err != nil {
		return role, err
	}
	role.State = state
	return role, nil
}
func (s *Store) OathTruth(ctx context.Context, account, id int64) (inventory.OathTruthState, error) {
	raw, err := s.queries.ShareActiveCharacterState(ctx, sqlcgen.ShareActiveCharacterStateParams{AccountID: account, CharacterID: id})
	if err != nil {
		return inventory.OathTruthState{}, err
	}
	return inventory.ReadOathTruth(raw)
}

func (s *Store) OathTruthVaults(ctx context.Context, account, id int64) ([2]VaultState, error) {
	var out [2]VaultState
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.rollback(ctx)
	if _, err = lockCharacter(ctx, tx, account, id); err != nil {
		return out, err
	}
	for i := range out {
		out[i], err = lockPersonalVault(ctx, tx.queries(), id, i == 1)
		if err != nil && !isNoRows(err) {
			return out, err
		}
	}
	return out, tx.commit(ctx)
}
