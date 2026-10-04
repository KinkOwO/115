package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"errors"
	"fmt"
)

func (s *Store) Quests(ctx context.Context, account, id int64) ([]QuestState, error) {
	owned, e := s.queries.CharacterOwned(ctx, sqlcgen.CharacterOwnedParams{AccountID: account, CharacterID: id})
	if e != nil {
		return nil, e
	}
	if !owned {
		return nil, errors.New("character is not owned")
	}
	rows, e := s.queries.Quests(ctx, id)
	if e != nil {
		return nil, e
	}
	out := []QuestState{}
	for _, row := range rows {
		out = append(out, QuestState{ID: uint16(row.QuestID), Status: row.Status, Progress: uint32(row.Progress), ConfigVersion: row.ConfigVersion, ProgressModel: row.ProgressModel})
	}
	return out, nil
}

// CompletedQuestIDs 批量读取本账号各角色的指定已完成任务，不将账号内其他角色的进度串给当前角色。
func (s *Store) CompletedQuestIDs(ctx context.Context, account int64, version string, ids []uint16) (map[int64][]uint16, error) {
	out := make(map[int64][]uint16)
	if len(ids) == 0 {
		return out, nil
	}
	questIDs := make([]int32, len(ids))
	for i, id := range ids {
		questIDs[i] = int32(id)
	}
	rows, err := s.queries.CompletedQuestIDs(ctx, sqlcgen.CompletedQuestIDsParams{AccountID: account, ConfigVersion: version, QuestIds: questIDs})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.CharacterID] = append(out[row.CharacterID], uint16(row.QuestID))
	}
	return out, nil
}

// Repair only the accepted, zero-valued records produced by the initial
// incomplete codec. A persisted before/after audit and compare-and-swap keep
// this operation idempotent and prevent overwriting newer objective progress.
func (s *Store) RepairLegacyQuest(ctx context.Context, account, id int64, before QuestState, initial uint32, model string) error {
	if before.Status != "accepted" || before.Progress != 0 || before.ProgressModel != "legacy-zero" || initial == 0 || model == "legacy-zero" || model == "" {
		return errors.New("not an eligible legacy quest")
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	after := before
	after.Progress = initial
	after.ProgressModel = model
	oldJSON, _ := json.Marshal(before)
	newJSON, _ := json.Marshal(after)
	queries := s.queries.WithTx(tx)
	changed, e := queries.RepairLegacyQuest(ctx, sqlcgen.RepairLegacyQuestParams{AccountID: account, CharacterID: id, QuestID: int32(before.ID), Progress: int64(initial), ProgressModel: model, ConfigVersion: before.ConfigVersion})
	if e != nil {
		return e
	}
	if changed != 1 {
		return fmt.Errorf("legacy quest changed; repair refused")
	}
	e = queries.RecordLegacyQuestRepair(ctx, sqlcgen.RecordLegacyQuestRepairParams{CharacterID: id, QuestID: int32(before.ID), BeforeState: oldJSON, AfterState: newJSON})
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
