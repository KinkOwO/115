package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"errors"
	"fmt"
)

// BoostStorySkipModel 既是 character_quests.progress_model，也是 character_events
// 的幂等键：走过 662 直升胶囊的角色，主线（源里 [grade] [epic]）按一次事务批量标为
// 完成。与奥德赛毕业用的是同一条执行语句（CompleteGraduationQuests：插入已完成行，
// 冲突时只把 accepted 行改成 completed），所以已接受/已完成的行都不会被降级。
//
// v2（2026-10-06）：计划边界由 `min < [goal level]` 放宽成 `min <= [goal level]`，
// 把 115 级当前章主线也扫进来（业主实机反馈「还剩 115 级的任务」）。已经带 v1 收据的
// 角色不能被收据挡住，否则那批存档永远修不好（§0 铁律 4）：检测到 v1 时按 v2 重跑一次，
// 执行语句本身对已完成行是安全的（ON CONFLICT … WHERE status='accepted'）。
const (
	BoostStorySkipModel       = "boost-story-skip-v2"
	BoostStorySkipLegacyModel = "boost-story-skip-v1"
)

// CommitBoostStorySkip runs the boost main-line pass at most once per model.
// The event row is the marker, so a failed transaction leaves nothing behind and
// the next town entry retries; a replay costs two indexed SELECTs.
func (s *Store) CommitBoostStorySkip(ctx context.Context, account, characterID int64, version string, apply func(Character) ([]uint16, error)) (int, bool, error) {
	if len(version) != 64 || apply == nil {
		return 0, false, errors.New("invalid boost story skip")
	}
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer tx.rollback(ctx)
	role, err := lockCharacter(ctx, tx, account, characterID)
	if err != nil {
		return 0, false, err
	}
	if role.ConfigVersion != version {
		return 0, false, fmt.Errorf("boost story skip source mismatch")
	}
	queries := tx.queries()
	if _, err = queries.CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: characterID, EventKey: BoostStorySkipModel}); err == nil {
		return 0, false, tx.commit(ctx)
	} else if !isNoRows(err) {
		return 0, false, err
	}
	compensated := false
	if _, err = queries.CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: characterID, EventKey: BoostStorySkipLegacyModel}); err == nil {
		compensated = true
	} else if !isNoRows(err) {
		return 0, false, err
	}
	ids, err := apply(role)
	if err != nil {
		return 0, false, err
	}
	for i, id := range ids {
		if id == 0 || id == 65535 || (i > 0 && ids[i-1] >= id) {
			return 0, false, fmt.Errorf("boost skip quests must be sorted, unique and valid")
		}
	}
	if len(ids) > 0 {
		if err = queries.CompleteGraduationQuests(ctx, sqlcgen.CompleteGraduationQuestsParams{
			CharacterID: characterID, QuestIds: questIDsToInt32(ids), ConfigVersion: version, ProgressModel: BoostStorySkipModel,
		}); err != nil {
			return 0, false, err
		}
	}
	outcome := map[string]any{"quests": ids, "count": len(ids), "source": version}
	if compensated {
		outcome["compensated_from"] = BoostStorySkipLegacyModel
	}
	proof, err := json.Marshal(outcome)
	if err != nil {
		return 0, false, err
	}
	if err = queries.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{
		CharacterID: characterID, EventKey: BoostStorySkipModel, ConfigVersion: version, Model: BoostStorySkipModel, Outcome: proof,
	}); err != nil {
		return 0, false, err
	}
	if err = tx.commit(ctx); err != nil {
		return 0, false, err
	}
	return len(ids), true, nil
}
