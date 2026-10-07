package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/hex"
	"fmt"
)

func (s *Store) MigrateQuestObjectives(ctx context.Context) error {
	return s.execMigration(ctx, "0024_quest_objectives.sql")
}

// CompleteQuestObjective marks one accepted objective satisfied. The caller
// must have verified the source condition itself; this only owns the durable
// transition and refuses a quest that is not this character's accepted quest
// under the same source and progress model.
func (s *Store) CompleteQuestObjective(ctx context.Context, account, characterID int64, qid uint16, version, model string) (bool, error) {
	if qid == 0 || qid == 65535 || len(version) != 64 || model == "" {
		return false, fmt.Errorf("invalid quest objective completion")
	}
	changed, err := s.queries.CompleteQuestObjective(ctx, sqlcgen.CompleteQuestObjectiveParams{AccountID: account, CharacterID: characterID, QuestID: int32(qid), ConfigVersion: version, ProgressModel: model})
	if err != nil {
		return false, err
	}
	return changed == 1, nil
}

// CompleteQuestUseObjective requires a committed item-use event created after
// this quest was accepted. A retried item action can repair an interrupted progress
// refresh without letting an item used before acceptance satisfy the quest.
func (s *Store) CompleteQuestUseObjective(ctx context.Context, account, characterID int64, qid uint16, version, model, eventKey string, template uint32) (bool, error) {
	if qid == 0 || qid == 65535 || len(version) != 64 || model == "" || eventKey == "" || template == 0 {
		return false, fmt.Errorf("invalid item-use quest completion")
	}
	changed, err := s.queries.CompleteQuestUseObjective(ctx, sqlcgen.CompleteQuestUseObjectiveParams{AccountID: account, CharacterID: characterID, QuestID: int32(qid), ConfigVersion: version, ProgressModel: model, EventKey: eventKey, Template: fmt.Sprint(template)})
	if err != nil {
		return false, err
	}
	return changed == 1, nil
}

// A server-owned run supplies the map and matching source objective IDs.
// A duplicate clear must not complete a newly accepted/reaccepted quest.
func (s *Store) RecordQuestMapClear(ctx context.Context, account, characterID int64, run string, mapID uint32, version, model string, matching []uint16) (bool, error) {
	seed, err := hex.DecodeString(run)
	if err != nil || len(seed) != 16 || mapID == 0 || len(version) != 64 || model == "" {
		return false, fmt.Errorf("invalid map clear evidence")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	id, err := queries.LockCharacterOwnerIncludingDeleted(ctx, sqlcgen.LockCharacterOwnerIncludingDeletedParams{AccountID: account, CharacterID: characterID})
	if err != nil {
		return false, err
	}
	changed, err := queries.RecordQuestMapClear(ctx, sqlcgen.RecordQuestMapClearParams{CharacterID: id, RunID: run, MapID: int64(mapID), SourceVersion: version})
	if err != nil {
		return false, err
	}
	if changed == 0 {
		return false, tx.commit(ctx)
	}
	for _, qid := range matching {
		if qid == 0 || qid == 65535 {
			return false, fmt.Errorf("invalid quest identity")
		}
		err = queries.CompleteQuestMapObjective(ctx, sqlcgen.CompleteQuestMapObjectiveParams{CharacterID: id, QuestID: int32(qid), ConfigVersion: version, ProgressModel: model})
		if err != nil {
			return false, err
		}
	}
	return true, tx.commit(ctx)
}
