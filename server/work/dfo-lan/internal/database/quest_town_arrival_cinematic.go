package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
	"time"
)

// CompleteTownArrivalCinematic owns the accepted-lifetime check and 1->0
// transition in the same SQLite writer transaction. Zero is a valid replay,
// but a run started before this acceptance cannot confirm even a replay.
func (s *Store) CompleteTownArrivalCinematic(ctx context.Context, account, characterID int64, id uint16, version, model string, started time.Time) (bool, error) {
	if id == 0 || id == 65535 || len(version) != 64 || model == "" || started.IsZero() {
		return false, fmt.Errorf("invalid arrival cinematic completion")
	}
	handle, err := s.engine.begin(ctx)
	if err != nil {
		return false, err
	}
	defer handle.rollback(ctx)
	tx, ok := handle.(*sqliteTx)
	if !ok {
		return false, fmt.Errorf("arrival cinematic requires SQLite owner")
	}
	var progress uint32
	err = tx.tx.QueryRowContext(ctx, `SELECT q.progress FROM character_quests q JOIN characters c ON c.id=q.character_id
 WHERE c.account_id=?1 AND c.id=?2 AND c.deleted_at IS NULL AND c.config_version=?4
 AND q.quest_id=?3 AND q.status='accepted' AND q.config_version=?4 AND q.progress_model=?5
 AND q.progress IN (0,1) AND q.accepted_at<=?6`, account, characterID, id, version, model, started.UnixMicro()).Scan(&progress)
	if err != nil {
		return false, fmt.Errorf("arrival cinematic accepted lifetime: %w", err)
	}
	if progress == 0 {
		return false, handle.commit(ctx)
	}
	changed, err := handle.queries().CompleteQuestObjective(ctx, sqlcgen.CompleteQuestObjectiveParams{AccountID: account, CharacterID: characterID, QuestID: int32(id), ConfigVersion: version, ProgressModel: model})
	if err != nil {
		return false, err
	}
	if changed != 1 {
		return false, fmt.Errorf("arrival cinematic objective not committed")
	}
	return true, handle.commit(ctx)
}

// MigrateTownArrivalCinematic only adapts an owned accepted quest under the
// current source. Old zero is retained only with matching source map, run,
// dungeon and acceptance receipts; completed quests and rewards are untouched.
func (s *Store) MigrateTownArrivalCinematic(ctx context.Context, account, characterID int64, id uint16, version, oldModel, model string, dungeon uint32, maps []uint32) error {
	if id == 0 || id == 65535 || len(version) != 64 || oldModel == "" || model == "" || dungeon == 0 {
		return fmt.Errorf("invalid arrival cinematic migration")
	}
	handle, err := s.engine.begin(ctx)
	if err != nil {
		return err
	}
	defer handle.rollback(ctx)
	tx, ok := handle.(*sqliteTx)
	if !ok {
		return fmt.Errorf("arrival cinematic requires SQLite owner")
	}
	proof := false
	for _, mapID := range maps {
		var found bool
		err = tx.tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM character_quests q
 JOIN characters c ON c.id=q.character_id
 JOIN character_map_clears m ON m.character_id=q.character_id AND m.map_id=?7 AND m.source_version=q.config_version AND m.cleared_at>=q.accepted_at
 JOIN character_events e ON e.character_id=q.character_id AND e.event_key='clear:'||m.run_id AND e.config_version=q.config_version AND e.created_at>q.accepted_at
 WHERE c.account_id=?1 AND c.id=?2 AND c.deleted_at IS NULL AND c.config_version=?4
 AND q.quest_id=?3 AND q.status='accepted' AND q.config_version=?4 AND q.progress_model=?5
 AND json_extract(e.outcome,'$.Run')=m.run_id AND json_extract(e.outcome,'$.dungeon_id')=?6)`, account, characterID, id, version, oldModel, dungeon, mapID).Scan(&found)
		if err != nil {
			return err
		}
		proof = proof || found
	}
	progress := 1
	if proof {
		progress = 0
	}
	_, err = tx.tx.ExecContext(ctx, `UPDATE character_quests AS q SET progress=?7,progress_model=?6 FROM characters c
 WHERE c.id=q.character_id AND c.account_id=?1 AND c.id=?2 AND c.deleted_at IS NULL AND c.config_version=?4
 AND q.quest_id=?3 AND q.status='accepted' AND q.config_version=?4 AND q.progress_model=?5`, account, characterID, id, version, oldModel, model, progress)
	if err != nil {
		return err
	}
	return handle.commit(ctx)
}
