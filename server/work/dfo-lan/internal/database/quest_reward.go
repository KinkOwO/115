package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"fmt"
)

func (s *Store) MigrateQuestRewards(ctx context.Context) error {
	return s.execMigration(ctx, "0025_quest_rewards.sql")
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
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	r := &out.Character
	row, e := queries.LockOwnedCharacterIncludingDeleted(ctx, sqlcgen.LockOwnedCharacterIncludingDeletedParams{AccountID: account, CharacterID: id})
	if e != nil {
		return out, e
	}
	*r = storedCharacter(sqlcgen.CharactersRow(row))
	if r.ConfigVersion != version {
		return out, fmt.Errorf("quest character source mismatch")
	}
	q, e := queries.LockQuest(ctx, sqlcgen.LockQuestParams{CharacterID: id, QuestID: int32(qid)})
	if e != nil {
		return out, e
	}
	if q.ConfigVersion != version || q.ProgressModel != progressModel || q.Progress != 0 {
		return out, fmt.Errorf("quest objective not ready under current source")
	}
	if q.Status == "completed" {
		out.Receipt, e = queries.QuestRewardReceipt(ctx, sqlcgen.QuestRewardReceiptParams{CharacterID: id, QuestID: int32(qid), SourceVersion: version})
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
	if e = queries.RecordQuestReward(ctx, sqlcgen.RecordQuestRewardParams{CharacterID: id, QuestID: int32(qid), SourceVersion: version, Model: rewardModel, Receipt: receipt}); e != nil {
		return out, e
	}
	if e = s.commitAdventureExperience(ctx, tx, *r, state); e != nil {
		return out, e
	}
	if e = queries.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); e != nil {
		return out, e
	}
	if e = queries.CompleteRewardedQuest(ctx, sqlcgen.CompleteRewardedQuestParams{CharacterID: id, QuestID: int32(qid)}); e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	r.State = state
	out.Receipt = receipt
	out.Applied = true
	return out, nil
}
