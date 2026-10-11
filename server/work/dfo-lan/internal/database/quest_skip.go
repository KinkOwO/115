package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"dfolan/internal/quest"
	"encoding/json"
	"fmt"
)

var _ quest.SkipStore = (*Store)(nil)

// CommitMainlineSkip owns the complete press: decide on the latest locked
// character/quest state, then commit quests, rewards, key items and receipts
// together. A callback failure writes nothing and can safely be retried.
func (s *Store) CommitMainlineSkip(ctx context.Context, account, id int64, version, rewardModel string, apply func(Character, []quest.QuestState, json.RawMessage, bool) (quest.SkipBatch, error)) (quest.SkipCommit, error) {
	var out quest.SkipCommit
	if len(version) != 64 || rewardModel == "" || apply == nil {
		return out, fmt.Errorf("invalid mainline skip request")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.rollback(ctx)
	q := tx.queries()
	current, err := lockCharacter(ctx, tx, account, id)
	if err != nil {
		return out, err
	}
	if current.ConfigVersion != version {
		return out, fmt.Errorf("quest character source mismatch")
	}
	saved, err := q.Quests(ctx, id)
	if err != nil {
		return out, err
	}
	states := make([]quest.QuestState, 0, len(saved))
	for _, v := range saved {
		states = append(states, quest.QuestState{ID: uint16(v.QuestID), Status: v.Status, ConfigVersion: v.ConfigVersion, Progress: uint32(v.Progress), ProgressModel: v.ProgressModel})
	}
	var ticket json.RawMessage
	priorTicket, err := q.StoredCharacterEvent(ctx, sqlcgen.StoredCharacterEventParams{CharacterID: id, EventKey: "quest-skip-ticket-items"})
	if err != nil && !isNoRows(err) {
		return out, err
	}
	if err == nil {
		if priorTicket.Model != "quest-skip-ticket-items-v1" {
			return out, fmt.Errorf("quest key receipt model mismatch")
		}
		ticket = priorTicket.Outcome
	}
	priorSweep, err := q.HasMainlineSweep(ctx, id)
	if err != nil {
		return out, err
	}
	batch, err := apply(current, states, ticket, priorSweep)
	if err != nil {
		return out, err
	}
	if !json.Valid(batch.State) {
		return out, fmt.Errorf("invalid skip character state")
	}
	if len(batch.Rows) > 0 && (batch.EventKey == "" || len(batch.EventKey) > 200 || !json.Valid(batch.Outcome)) {
		return out, fmt.Errorf("invalid skip audit")
	}
	ids := []int32{}
	completed := map[uint16]bool{}
	for _, v := range states {
		if v.Status == "completed" {
			completed[v.ID] = true
		}
	}
	seen := map[uint16]bool{}
	for _, v := range batch.Rows {
		if v.Quest == 0 || v.Quest == 65535 || v.Model == "" || v.Model == "legacy-zero" || seen[v.Quest] || completed[v.Quest] || !json.Valid(v.Receipt) {
			return out, fmt.Errorf("invalid or previously completed skip row %d", v.Quest)
		}
		seen[v.Quest] = true
		ids = append(ids, int32(v.Quest))
	}
	if len(ids) > 0 {
		for _, row := range batch.Rows {
			if err = q.UpsertMainlineSkippedQuest(ctx, sqlcgen.UpsertMainlineSkippedQuestParams{CharacterID: id, QuestID: int32(row.Quest), ConfigVersion: version, ProgressModel: row.Model}); err != nil {
				return out, err
			}
			if err = q.RecordQuestReward(ctx, sqlcgen.RecordQuestRewardParams{CharacterID: id, QuestID: int32(row.Quest), SourceVersion: version, Model: rewardModel, Receipt: row.Receipt}); err != nil {
				return out, err
			}
		}
		if err = q.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: batch.EventKey, ConfigVersion: version, Model: "gm-quest-sweep-v1", Outcome: batch.Outcome}); err != nil {
			return out, err
		}
	}
	if len(batch.TicketOutcome) > 0 {
		if !json.Valid(batch.TicketOutcome) || (len(ids) == 0 && !priorSweep) {
			return out, fmt.Errorf("invalid key receipt without a sweep")
		}
		if err = q.SaveMainlineTicketOutcome(ctx, sqlcgen.SaveMainlineTicketOutcomeParams{CharacterID: id, ConfigVersion: version, Outcome: batch.TicketOutcome}); err != nil {
			return out, err
		}

	}
	if len(ids) > 0 || len(batch.TicketOutcome) > 0 {
		if err = q.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: batch.State}); err != nil {
			return out, err
		}
	}
	if err = tx.commit(ctx); err != nil {
		return out, err
	}
	current.State = batch.State
	out.Character, out.Settled, out.EventApplied = current, batch.Rows, len(ids) > 0
	return out, nil
}
