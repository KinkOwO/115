package storage

import (
	"context"
	"encoding/hex"
	"fmt"
)

func (s *Store) MigrateQuestObjectives(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_map_clears (
 character_id bigint NOT NULL REFERENCES characters(id), run_id text NOT NULL,
 map_id bigint NOT NULL CHECK(map_id>0), source_version text NOT NULL,
 cleared_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id,run_id,map_id));`)
	return err
}

// CompleteQuestObjective marks one accepted objective satisfied. The caller
// must have verified the source condition itself; this only owns the durable
// transition and refuses a quest that is not this character's accepted quest
// under the same source and progress model.
func (s *Store) CompleteQuestObjective(ctx context.Context, account, characterID int64, qid uint16, version, model string) (bool, error) {
	if qid == 0 || qid == 65535 || len(version) != 64 || model == "" {
		return false, fmt.Errorf("invalid quest objective completion")
	}
	tag, err := s.DB.Exec(ctx, `UPDATE character_quests q SET progress=0 FROM characters c
 WHERE c.id=q.character_id AND c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL
 AND q.quest_id=$3 AND q.status='accepted' AND q.config_version=$4 AND q.progress_model=$5 AND q.progress<>0`,
		account, characterID, qid, version, model)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// A server-owned run supplies the map and matching source objective IDs.
// A duplicate clear must not complete a newly accepted/reaccepted quest.
func (s *Store) RecordQuestMapClear(ctx context.Context, account, characterID int64, run string, mapID uint32, version, model string, matching []uint16) (bool, error) {
	seed, err := hex.DecodeString(run)
	if err != nil || len(seed) != 16 || mapID == 0 || len(version) != 64 || model == "" {
		return false, fmt.Errorf("invalid map clear evidence")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var id int64
	if err = tx.QueryRow(ctx, `SELECT id FROM characters WHERE account_id=$1 AND id=$2 FOR UPDATE`, account, characterID).Scan(&id); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO character_map_clears(character_id,run_id,map_id,source_version) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, id, run, mapID, version)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, tx.Commit(ctx)
	}
	for _, qid := range matching {
		if qid == 0 || qid == 65535 {
			return false, fmt.Errorf("invalid quest identity")
		}
		_, err = tx.Exec(ctx, `UPDATE character_quests SET progress=0 WHERE character_id=$1 AND quest_id=$2 AND status='accepted' AND progress=1 AND config_version=$3 AND progress_model=$4`, id, qid, version, model)
		if err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}
