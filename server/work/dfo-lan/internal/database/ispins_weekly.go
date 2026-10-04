package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
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
	return s.queries.IspinsWeeklyUsed(ctx, sqlcgen.IspinsWeeklyUsedParams{AccountID: account, CharacterID: id, Model: ispinsWeeklyModel, Week: IspinsWeekStart(now).Format(time.RFC3339)})
}

// Both modes record clears. Unlimited ignores the weekly admission cap.
func (s *Store) RecordIspinsWeeklyClear(ctx context.Context, account, id int64, version, run string, now time.Time, limited bool) error {
	raw, e := hex.DecodeString(run)
	if e != nil || len(raw) != 16 {
		return fmt.Errorf("invalid Ispins run identity")
	}
	week := IspinsWeekStart(now).Format(time.RFC3339)
	key := "ispins-clear:" + run
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
	saved, e := queries.LockCharacterVersion(ctx, sqlcgen.LockCharacterVersionParams{AccountID: account, CharacterID: id})
	if e != nil {
		return e
	}
	if saved != version {
		return fmt.Errorf("Ispins clear save identity mismatch")
	}
	model, e := sqlcgen.New(tx).CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: id, EventKey: key})
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
		used, err := queries.LockedIspinsWeeklyUsed(ctx, sqlcgen.LockedIspinsWeeklyUsedParams{CharacterID: id, Model: ispinsWeeklyModel, Week: week})
		if err != nil {
			return err
		}
		if used {
			return ErrIspinsWeeklyCleared
		}
	}
	if e = queries.RecordIspinsWeeklyClear(ctx, sqlcgen.RecordIspinsWeeklyClearParams{CharacterID: id, EventKey: key, ConfigVersion: version, Model: ispinsWeeklyModel, Week: week, Run: run}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
