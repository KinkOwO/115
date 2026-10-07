package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const (
	// OdysseyGraduationEvent is the receipt of the current graduation rule.
	// A character carrying an older receipt re-runs the plan as a compensation
	// pass (see compensated below), so a source-scope fix reaches live saves.
	OdysseyGraduationEvent = "odyssey-graduation-v3"
	// compensatedPriorEvent is the previous rule version's receipt key.
	compensatedPriorEvent = "odyssey-graduation-v2"
)

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
	// 老收据（v2）说明该角色已按上一版规则毕业过：这一轮只补缺失行，
	// 不动玩家正在进行的任务（其交任务与奖励保持原样），也不重发奖励。
	priorCompensated, priorErr := queries.CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: id, EventKey: compensatedPriorEvent})
	compensated := false
	switch {
	case priorErr == nil:
		if priorCompensated != compensatedPriorEvent {
			return role, false, fmt.Errorf("Odyssey graduation prior model mismatch")
		}
		compensated = true
	case isNoRows(priorErr):
	default:
		return role, false, priorErr
	}
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
		if compensated {
			_, err = queries.ClearQuests(ctx, sqlcgen.ClearQuestsParams{ConfigVersion: version, QuestIds: questIDsToInt32(quests), CharacterID: id, AccountID: account})
		} else {
			err = queries.CompleteGraduationQuests(ctx, sqlcgen.CompleteGraduationQuestsParams{CharacterID: id, QuestIds: questIDsToInt32(quests), ConfigVersion: version, ProgressModel: OdysseyGraduationEvent})
		}
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
