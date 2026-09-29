package storage

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var current string
	if err = tx.QueryRow(ctx, `SELECT config_version FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&current); err != nil {
		return nil, err
	}
	if current != version {
		return nil, fmt.Errorf("黑鸦奖励角色配置不一致")
	}
	var priorModel string
	var raw json.RawMessage
	err = tx.QueryRow(ctx, `SELECT model,outcome FROM character_events WHERE character_id=$1 AND event_key=$2`, id, "cardplan:"+run).Scan(&priorModel, &raw)
	if err == nil {
		if priorModel != model {
			return nil, fmt.Errorf("黑鸦奖励回执类型不一致")
		}
		return raw, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var stage string
	err = tx.QueryRow(ctx, `SELECT outcome->>'stage' FROM character_events WHERE character_id=$1 AND event_key=$2 AND model='black-purgatory-entry-v1'`, id, "black-purgatory-entry:"+run).Scan(&stage)
	if err != nil {
		return nil, err
	}
	if stage != "entered" && stage != "cleared" {
		return nil, fmt.Errorf("黑鸦奖励缺少有效挑战回执")
	}
	raw, err = create()
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("黑鸦奖单格式无效")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,$4,$5)`, id, "cardplan:"+run, version, model, raw); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE character_events SET outcome=jsonb_set(outcome,'{stage}','"cleared"') WHERE character_id=$1 AND event_key=$2`, id, "black-purgatory-entry:"+run); err != nil {
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
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var version string
	if err = tx.QueryRow(ctx, `SELECT config_version FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&version); err != nil {
		return out, err
	}
	const model = "black-purgatory-entry-v1"
	key := "black-purgatory-entry:" + run
	stage := ""
	if run != "" {
		var priorModel string
		err = tx.QueryRow(ctx, `SELECT model,outcome->>'stage' FROM character_events WHERE character_id=$1 AND event_key=$2`, id, key).Scan(&priorModel, &stage)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		if err == nil && priorModel != model {
			return out, fmt.Errorf("黑鸦挑战回执类型不匹配")
		}
	}
	if action == "recover" {
		// 新登录不恢复旧副本会话，返还因掉线或停服遗留的未通关占用。
		_, err = tx.Exec(ctx, `UPDATE character_events SET outcome=jsonb_set(outcome,'{stage}','"refunded"') WHERE character_id=$1 AND model=$2 AND outcome->>'stage'='entered'`, id, model)
	} else if action == "refund" && stage == "entered" {
		_, err = tx.Exec(ctx, `UPDATE character_events SET outcome=jsonb_set(outcome,'{stage}','"refunded"') WHERE character_id=$1 AND event_key=$2`, id, key)
	} else if action == "clear" {
		if stage != "entered" && stage != "cleared" {
			return out, fmt.Errorf("黑鸦通关缺少有效入场回执")
		}
		_, err = tx.Exec(ctx, `UPDATE character_events SET outcome=jsonb_set(outcome,'{stage}','"cleared"') WHERE character_id=$1 AND event_key=$2`, id, key)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	var daily, weekly int
	err = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE created_at >= $3),count(*) FROM character_events
 WHERE character_id=$1 AND model=$2 AND created_at >= $4 AND outcome->>'stage' IN ('entered','cleared')`, id, model, day, week).Scan(&daily, &weekly)
	if err != nil {
		return out, err
	}
	out = BlackPurgatoryQuota{Daily: byte(max(1-daily, 0)), Weekly: byte(max(2-weekly, 0))}
	if action == "enter" {
		if stage == "refunded" || stage == "cleared" {
			return out, fmt.Errorf("黑鸦挑战已结束，不能复用旧入场回执")
		}
		if stage == "" {
			if out.Daily == 0 || out.Weekly == 0 {
				return out, fmt.Errorf("黑鸦之境剩余入场次数不足")
			}
			_, err = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome)
 VALUES($1,$2,$3,$4,'{"stage":"entered"}')`, id, key, version, model)
			if err != nil {
				return out, err
			}
			out.Daily--
			out.Weekly--
		}
	}
	return out, tx.Commit(ctx)
}
