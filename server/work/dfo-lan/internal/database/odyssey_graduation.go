package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const OdysseyGraduationEvent = "odyssey-graduation-v2"

// CommitOdysseyGraduation commits mode, quests and receipt under the same
// character lock. A v1 receipt must not suppress migration of an older save.
func (s *Store) CommitOdysseyGraduation(ctx context.Context, account, id int64, version string, apply func(Character, bool) (json.RawMessage, json.RawMessage, []uint16, error)) (Character, bool, error) {
	var role Character
	checksum, err := hex.DecodeString(version)
	if err != nil || len(checksum) != 32 || apply == nil {
		return role, false, fmt.Errorf("invalid Odyssey graduation event")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return role, false, err
	}
	defer tx.rollback(ctx)
	role, err = lockCharacter(ctx, tx, account, id)
	if err != nil {
		return role, false, err
	}
	if role.ConfigVersion != version {
		return role, false, fmt.Errorf("Odyssey graduation source mismatch")
	}
	prior, err := tx.queries().CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: id, EventKey: OdysseyGraduationEvent})
	if err == nil {
		if prior != OdysseyGraduationEvent {
			return role, false, fmt.Errorf("Odyssey graduation model mismatch")
		}
		return role, false, tx.commit(ctx)
	}
	if !isNoRows(err) {
		return role, false, err
	}
	queries := tx.queries()
	paid, err := queries.OdysseyGraduationAlreadyPaid(ctx, id)
	if err != nil {
		return role, false, err
	}
	state, proof, quests, err := apply(role, paid)
	if err != nil {
		return role, false, err
	}
	if !json.Valid(state) || !json.Valid(proof) {
		return role, false, fmt.Errorf("invalid Odyssey graduation JSON")
	}
	for i, q := range quests {
		if q == 0 || q == 65535 || (i > 0 && quests[i-1] >= q) {
			return role, false, fmt.Errorf("graduation quests must be sorted, unique and valid")
		}
	}
	if len(quests) > 0 {
		err = queries.CompleteGraduationQuests(ctx, sqlcgen.CompleteGraduationQuestsParams{CharacterID: id, QuestIds: questIDsToInt32(quests), ConfigVersion: version, ProgressModel: OdysseyGraduationEvent})
		if err != nil {
			return role, false, err
		}
	}
	if err = tx.queries().UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); err != nil {
		return role, false, err
	}
	if err = queries.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: OdysseyGraduationEvent, ConfigVersion: version, Model: OdysseyGraduationEvent, Outcome: proof}); err != nil {
		return role, false, err
	}
	if err = tx.commit(ctx); err != nil {
		return role, false, err
	}
	role.State = state
	return role, true, nil
}
