package storage

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *Store) MigrateQuestRewards(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_quest_rewards (
 character_id bigint NOT NULL, quest_id integer NOT NULL, source_version text NOT NULL,
 model text NOT NULL, receipt jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,quest_id), FOREIGN KEY(character_id,quest_id) REFERENCES character_quests(character_id,quest_id));`)
	return e
}

type QuestRewardCommit struct {
	Character Character
	Receipt   json.RawMessage
	Applied   bool
}

func (s *Store) CommitQuestReward(ctx context.Context, account, id int64, qid uint16, version, progressModel, rewardModel string, apply func(Character) (json.RawMessage, json.RawMessage, error)) (QuestRewardCommit, error) {
	var out QuestRewardCommit
	if qid == 0 || qid == 65535 || len(version) != 64 || progressModel == "" || rewardModel == "" || apply == nil {
		return out, fmt.Errorf("invalid quest reward request")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	r := &out.Character
	e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 FOR UPDATE`, account, id).Scan(&r.ID, &r.AccountID, &r.WireID, &r.Name, &r.Profession, &r.Request, &r.ConfigVersion, &r.State, &r.CreatedAt)
	if e != nil {
		return out, e
	}
	if r.ConfigVersion != version {
		return out, fmt.Errorf("quest character source mismatch")
	}
	var q QuestState
	e = tx.QueryRow(ctx, `SELECT status,progress,config_version,progress_model FROM character_quests WHERE character_id=$1 AND quest_id=$2 FOR UPDATE`, id, qid).Scan(&q.Status, &q.Progress, &q.ConfigVersion, &q.ProgressModel)
	if e != nil {
		return out, e
	}
	if q.ConfigVersion != version || q.ProgressModel != progressModel || q.Progress != 0 {
		return out, fmt.Errorf("quest objective not ready under current source")
	}
	if q.Status == "completed" {
		e = tx.QueryRow(ctx, `SELECT receipt FROM character_quest_rewards WHERE character_id=$1 AND quest_id=$2 AND source_version=$3`, id, qid, version).Scan(&out.Receipt)
		if e != nil {
			return out, e
		}
		return out, tx.Commit(ctx)
	}
	if q.Status != "accepted" {
		return out, fmt.Errorf("quest is not accepted")
	}
	state, receipt, e := apply(*r)
	if e != nil {
		return out, e
	}
	if !json.Valid(state) || !json.Valid(receipt) {
		return out, fmt.Errorf("invalid quest reward JSON")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO character_quest_rewards(character_id,quest_id,source_version,model,receipt) VALUES($1,$2,$3,$4,$5)`, id, qid, version, rewardModel, receipt); e != nil {
		return out, e
	}
	if e = s.commitAdventureExperience(ctx, tx, *r, state); e != nil {
		return out, e
	}
	if _, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state); e != nil {
		return out, e
	}
	if _, e = tx.Exec(ctx, `UPDATE character_quests SET status='completed',completed_at=now() WHERE character_id=$1 AND quest_id=$2`, id, qid); e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	r.State = state
	out.Receipt = receipt
	out.Applied = true
	s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	return out, nil
}
