package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

func (s *Store) Quests(ctx context.Context, account, id int64) ([]QuestState, error) {
	var owned bool
	if e := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM characters WHERE account_id=$1 AND id=$2)`, account, id).Scan(&owned); e != nil {
		return nil, e
	}
	if !owned {
		return nil, errors.New("character is not owned")
	}
	rows, e := s.DB.Query(ctx, `SELECT quest_id,status,progress,config_version,progress_model FROM character_quests WHERE character_id=$1 ORDER BY quest_id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []QuestState{}
	for rows.Next() {
		var q QuestState
		if e = rows.Scan(&q.ID, &q.Status, &q.Progress, &q.ConfigVersion, &q.ProgressModel); e != nil {
			return nil, e
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// Repair only the accepted, zero-valued records produced by the initial
// incomplete codec. A persisted before/after audit and compare-and-swap keep
// this operation idempotent and prevent overwriting newer objective progress.
func (s *Store) RepairLegacyQuest(ctx context.Context, account, id int64, before QuestState, initial uint32, model string) error {
	if before.Status != "accepted" || before.Progress != 0 || before.ProgressModel != "legacy-zero" || initial == 0 || model == "legacy-zero" || model == "" {
		return errors.New("not an eligible legacy quest")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	after := before
	after.Progress = initial
	after.ProgressModel = model
	oldJSON, _ := json.Marshal(before)
	newJSON, _ := json.Marshal(after)
	tag, e := tx.Exec(ctx, `UPDATE character_quests q SET progress=$4,progress_model=$5 FROM characters c
      WHERE q.character_id=c.id AND c.account_id=$1 AND c.id=$2 AND q.quest_id=$3
      AND q.status='accepted' AND q.progress=0 AND q.progress_model='legacy-zero' AND q.config_version=$6`, account, id, before.ID, initial, model, before.ConfigVersion)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("legacy quest changed; repair refused")
	}
	_, e = tx.Exec(ctx, `INSERT INTO character_quest_repairs(character_id,quest_id,reason,before_state,after_state) VALUES($1,$2,'initial-zero-false-completion',$3,$4)`, id, before.ID, oldJSON, newJSON)
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
