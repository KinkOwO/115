package storage

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"time"
)

var ErrIspinsWeeklyCleared = errors.New("伊斯大陆本周已完成，请等待周二重置或切换单机无限模式")

const ispinsWeeklyModel = "ispins-weekly-clear-v1"

// US boundary: Tuesday09:00UTC. Uses existing events, never changes player JSON/schema.
func IspinsWeekStart(now time.Time) time.Time {
	t := now.UTC().Add(-9 * time.Hour)
	day := time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, time.UTC)
	return day.AddDate(0, 0, -(int(t.Weekday())+7-int(time.Tuesday))%7)
}
func (s *Store) IspinsWeeklyUsed(ctx context.Context, account, id int64, now time.Time) (bool, error) {
	var used bool
	e := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM character_events e JOIN characters c ON c.id=e.character_id WHERE c.id=$1 AND c.account_id=$2 AND c.deleted_at IS NULL AND e.model=$3 AND e.outcome->>'week'=$4)`, id, account, ispinsWeeklyModel, IspinsWeekStart(now).Format(time.RFC3339)).Scan(&used)
	return used, e
}

// Both modes record clears. Unlimited ignores the weekly admission cap.
func (s *Store) RecordIspinsWeeklyClear(ctx context.Context, account, id int64, version, run string, now time.Time, limited bool) error {
	raw, e := hex.DecodeString(run)
	if e != nil || len(raw) != 16 {
		return fmt.Errorf("invalid Ispins run identity")
	}
	week := IspinsWeekStart(now).Format(time.RFC3339)
	key := "ispins-clear:" + run
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var saved string
	if e = tx.QueryRow(ctx, `SELECT config_version FROM characters WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL FOR UPDATE`, id, account).Scan(&saved); e != nil {
		return e
	}
	if saved != version {
		return fmt.Errorf("Ispins clear save identity mismatch")
	}
	var model string
	e = tx.QueryRow(ctx, `SELECT model FROM character_events WHERE character_id=$1 AND event_key=$2`, id, key).Scan(&model)
	if e == nil {
		if model != ispinsWeeklyModel {
			return fmt.Errorf("Ispins receipt model mismatch")
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	if limited {
		var used bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM character_events WHERE character_id=$1 AND model=$2 AND outcome->>'week'=$3)`, id, ispinsWeeklyModel, week).Scan(&used); e != nil {
			return e
		}
		if used {
			return ErrIspinsWeeklyCleared
		}
	}
	if _, e = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,$4,jsonb_build_object('week',$5::text,'run',$6::text))`, id, key, version, ispinsWeeklyModel, week, run); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
