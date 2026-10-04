package database

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"dfolan/internal/database/sqlcgen"

	"github.com/jackc/pgx/v5"
)

type BlackPurgatoryQuota struct {
	Daily, Weekly byte
}

// 通关次数与奖单在同一角色锁、同一事务内提交，掉线不能返还已生成奖励的次数。
func (s *Store) FreezeBlackPurgatoryReward(ctx context.Context, account, id int64, version, run, model string,
	create func() (json.RawMessage, error),
) (json.RawMessage, error) {
	decoded, err := hex.DecodeString(run)
	if err != nil || len(decoded) != 16 || model != "black-purgatory-card-v1" || create == nil {
		return nil, fmt.Errorf("黑鸦奖励事务参数无效")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := sqlcgen.New(tx)
	current, err := q.LockCharacterVersion(ctx, sqlcgen.LockCharacterVersionParams{AccountID: account, CharacterID: id})
	if err != nil {
		return nil, err
	}
	if current != version {
		return nil, fmt.Errorf("黑鸦奖励角色配置不一致")
	}
	prior, err := q.StoredCharacterEvent(ctx, sqlcgen.StoredCharacterEventParams{CharacterID: id, EventKey: "cardplan:" + run})
	if err == nil {
		if prior.Model != model {
			return nil, fmt.Errorf("黑鸦奖励回执类型不一致")
		}
		return prior.Outcome, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	stage, err := q.BlackPurgatoryEntryStage(ctx, sqlcgen.BlackPurgatoryEntryStageParams{CharacterID: id, EventKey: "black-purgatory-entry:" + run})
	if err != nil {
		return nil, err
	}
	if stage != "entered" && stage != "cleared" {
		return nil, fmt.Errorf("黑鸦奖励缺少有效挑战回执")
	}
	raw, err := create()
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("黑鸦奖单格式无效")
	}
	if err = q.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: "cardplan:" + run, ConfigVersion: version, Model: model, Outcome: raw}); err != nil {
		return nil, err
	}
	if err = q.SetCharacterEventStage(ctx, sqlcgen.SetCharacterEventStageParams{CharacterID: id, EventKey: "black-purgatory-entry:" + run, Stage: "cleared"}); err != nil {
		return nil, err
	}
	return raw, tx.Commit(ctx)
}

// BlackPurgatoryQuota 复用角色事件账本，不改写角色JSON或已有物品存档。
// 原版dungeonincountinfo.etc：四种黑鸦模式共享每日1次、每周2次。
// 入场占用，未通关退出返还；通关后保留消耗。RunID保证重复加载不重复扣次。
func (s *Store) BlackPurgatoryQuota(ctx context.Context, account, id int64, run, action string, day, week time.Time) (BlackPurgatoryQuota, error) {
	var out BlackPurgatoryQuota
	if day.IsZero() || week.IsZero() || week.After(day) {
		return out, fmt.Errorf("黑鸦次数周期无效")
	}
	switch action {
	case "read", "recover":
		if run != "" {
			return out, fmt.Errorf("黑鸦次数读取不能指定挑战")
		}
	case "enter", "refund", "clear":
		decoded, err := hex.DecodeString(run)
		if err != nil || len(decoded) != 16 {
			return out, fmt.Errorf("黑鸦挑战标识无效")
		}
	default:
		return out, fmt.Errorf("黑鸦次数操作无效")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	q := sqlcgen.New(tx)
	version, err := q.LockCharacterVersion(ctx, sqlcgen.LockCharacterVersionParams{AccountID: account, CharacterID: id})
	if err != nil {
		return out, err
	}
	const model = "black-purgatory-entry-v1"
	key := "black-purgatory-entry:" + run
	stage := ""
	if run != "" {
		var prior sqlcgen.CharacterEventStageRow
		prior, err = q.CharacterEventStage(ctx, sqlcgen.CharacterEventStageParams{CharacterID: id, EventKey: key})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		if err == nil && prior.Model != model {
			return out, fmt.Errorf("黑鸦挑战回执类型不匹配")
		}
		stage = prior.Stage
	}
	if action == "recover" {
		// 新登录不恢复旧副本会话，返还因掉线或停服遗留的未通关占用。
		err = q.RefundPendingBlackPurgatoryEntries(ctx, id)
	} else if action == "refund" && stage == "entered" {
		err = q.SetCharacterEventStage(ctx, sqlcgen.SetCharacterEventStageParams{CharacterID: id, EventKey: key, Stage: "refunded"})
	} else if action == "clear" {
		if stage != "entered" && stage != "cleared" {
			return out, fmt.Errorf("黑鸦通关缺少有效入场回执")
		}
		err = q.SetCharacterEventStage(ctx, sqlcgen.SetCharacterEventStageParams{CharacterID: id, EventKey: key, Stage: "cleared"})
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	counts, err := q.BlackPurgatoryEntryCounts(ctx, sqlcgen.BlackPurgatoryEntryCountsParams{CharacterID: id, DayStart: day, WeekStart: week})
	if err != nil {
		return out, err
	}
	out = BlackPurgatoryQuota{Daily: byte(max(1-counts.Daily, 0)), Weekly: byte(max(2-counts.Weekly, 0))}
	if action == "enter" {
		if stage == "refunded" || stage == "cleared" {
			return out, fmt.Errorf("黑鸦挑战已结束，不能复用旧入场回执")
		}
		if stage == "" {
			if out.Daily == 0 || out.Weekly == 0 {
				return out, fmt.Errorf("黑鸦之境剩余入场次数不足")
			}
			err = q.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: key, ConfigVersion: version, Model: model, Outcome: json.RawMessage(`{"stage":"entered"}`)})
			if err != nil {
				return out, err
			}
			out.Daily--
			out.Weekly--
		}
	}
	return out, tx.Commit(ctx)
}
