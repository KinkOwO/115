package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

type QuestState struct {
	ID            uint16 `json:"id"`
	Status        string `json:"status"`
	Progress      uint32 `json:"progress"`
	ConfigVersion string `json:"config_version"`
	ProgressModel string `json:"progress_model"`
}

func (s *Store) MigrateQuests(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_quests (
 character_id bigint NOT NULL REFERENCES characters(id), quest_id integer NOT NULL CHECK(quest_id BETWEEN 1 AND 65535),
 status text NOT NULL CHECK(status IN ('accepted','completed')), progress bigint NOT NULL DEFAULT 0 CHECK(progress BETWEEN 0 AND 4294967295),
 config_version text NOT NULL, accepted_at timestamptz NOT NULL DEFAULT now(), completed_at timestamptz,
 PRIMARY KEY(character_id,quest_id));
 ALTER TABLE character_quests ADD COLUMN IF NOT EXISTS progress_model text NOT NULL DEFAULT 'legacy-zero';
 CREATE TABLE IF NOT EXISTS character_quest_repairs (
   character_id bigint NOT NULL, quest_id integer NOT NULL, reason text NOT NULL,
   before_state jsonb NOT NULL, after_state jsonb NOT NULL, repaired_at timestamptz NOT NULL DEFAULT now(),
   PRIMARY KEY(character_id,quest_id,reason));`)
	return e
}
func (s *Store) AcceptQuest(ctx context.Context, account, characterID int64, qid uint16, version string, minLevel, maxLevel uint32, prerequisites []uint32, initial uint32, model string) (QuestState, error) {
	out := QuestState{ID: qid, Status: "accepted", ConfigVersion: version, Progress: initial, ProgressModel: model}
	if qid == 0 || qid == 65535 || len(version) != 64 || model == "" || model == "legacy-zero" {
		return out, errors.New("invalid quest")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	var raw []byte
	if e = tx.QueryRow(ctx, `SELECT state FROM characters WHERE id=$2 AND account_id=$1 FOR UPDATE`, account, characterID).Scan(&raw); e != nil {
		return out, e
	}
	var state struct {
		Level uint32 `json:"level"`
	}
	if e = json.Unmarshal(raw, &state); e != nil {
		return out, e
	}
	if state.Level < minLevel || state.Level > maxLevel {
		return out, errors.New("quest level condition not met")
	}
	e = tx.QueryRow(ctx, `SELECT status,progress,config_version,progress_model FROM character_quests WHERE character_id=$1 AND quest_id=$2`, characterID, qid).Scan(&out.Status, &out.Progress, &out.ConfigVersion, &out.ProgressModel)
	if e == nil {
		if out.Status != "accepted" || out.ConfigVersion != version || out.ProgressModel != model {
			return out, errors.New("quest is completed or requires configuration migration")
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	for _, pre := range prerequisites {
		var done bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM character_quests WHERE character_id=$1 AND quest_id=$2 AND status='completed')`, characterID, pre).Scan(&done); e != nil {
			return out, e
		}
		if !done {
			return out, fmt.Errorf("prerequisite %d incomplete", pre)
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO character_quests(character_id,quest_id,status,config_version,progress,progress_model) VALUES($1,$2,'accepted',$3,$4,$5)`, characterID, qid, version, initial, model)
	if e != nil {
		return out, e
	}
	return out, tx.Commit(ctx)
}
func (s *Store) AbandonQuest(ctx context.Context, account, characterID int64, qid uint16) error {
	tag, e := s.DB.Exec(ctx, `DELETE FROM character_quests q USING characters c WHERE q.character_id=c.id AND c.id=$2 AND c.account_id=$1 AND q.quest_id=$3 AND q.status='accepted'`, account, characterID, qid)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return errors.New("quest is not active for this character")
	}
	return nil
}
