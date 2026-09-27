package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

// TowerProgress keeps each tower's account progression and daily admissions
// separate. A tower's source rules supply its entry limit and reset hour.
type TowerProgress struct {
	HighestCleared uint16
	EntryDay       string
	EntriesToday   uint16
	LastRun        string
}

type TowerPolicy struct {
	Key          string
	TopFloor     uint16
	DailyEntries uint16
	ResetHourUTC uint8
}

var towerKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func (p TowerPolicy) validate() error {
	if !towerKeyPattern.MatchString(p.Key) || p.TopFloor == 0 || p.DailyEntries == 0 || p.ResetHourUTC > 23 {
		return fmt.Errorf("invalid tower policy")
	}
	return nil
}

func (p TowerPolicy) ServiceDay(now time.Time) string {
	return now.UTC().Add(-time.Duration(p.ResetHourUTC) * time.Hour).Format("2006-01-02")
}

// MigrateTowerProgress copies the earlier Grief save without changing it or
// touching character state. The old table remains available for recovery.
func (s *Store) MigrateTowerProgress(ctx context.Context) error {
	if err := s.MigrateTowerGriefProgress(ctx); err != nil {
		return err
	}
	if _, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_tower_progress (
 account_id bigint NOT NULL REFERENCES accounts(id),
 tower_key text NOT NULL,
 highest_cleared smallint NOT NULL DEFAULT 0 CHECK(highest_cleared >= 0),
 entry_day date,
 entries_today smallint NOT NULL DEFAULT 0 CHECK(entries_today >= 0),
 last_run_id text NOT NULL DEFAULT '',
 PRIMARY KEY(account_id,tower_key)
 );`); err != nil {
		return err
	}
	_, err := s.DB.Exec(ctx, `INSERT INTO account_tower_progress(account_id,tower_key,highest_cleared,entry_day,entries_today,last_run_id)
 SELECT account_id,'grief',highest_cleared,cleared_day,
 CASE WHEN cleared_day IS NULL THEN 0 ELSE 1 END,last_run_id
 FROM account_tower_grief_progress
 ON CONFLICT(account_id,tower_key) DO NOTHING`)
	return err
}

func (s *Store) ReadTowerProgress(ctx context.Context, account int64, policy TowerPolicy, legacyFloor uint16) (TowerProgress, error) {
	var out TowerProgress
	if account <= 0 || policy.validate() != nil || legacyFloor > policy.TopFloor {
		return out, fmt.Errorf("invalid tower progress request")
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO account_tower_progress(account_id,tower_key,highest_cleared)
 VALUES($1,$2,$3) ON CONFLICT(account_id,tower_key) DO NOTHING`, account, policy.Key, legacyFloor); err != nil {
		return out, err
	}
	err := s.DB.QueryRow(ctx, `SELECT highest_cleared,COALESCE(entry_day::text,''),entries_today,last_run_id
 FROM account_tower_progress WHERE account_id=$1 AND tower_key=$2`, account, policy.Key).
		Scan(&out.HighestCleared, &out.EntryDay, &out.EntriesToday, &out.LastRun)
	return out, err
}

// ReserveTowerEntry records each admission at entry, regardless of clear result.
// WIP testing: daily entry limits are temporarily disabled for every tower.
func (s *Store) ReserveTowerEntry(ctx context.Context, account int64, policy TowerPolicy, floor uint16, now time.Time) (TowerProgress, error) {
	var out TowerProgress
	if account <= 0 || policy.validate() != nil || floor == 0 || floor > policy.TopFloor {
		return out, fmt.Errorf("invalid tower entry")
	}
	day := policy.ServiceDay(now)
	err := s.DB.QueryRow(ctx, `UPDATE account_tower_progress
 SET entry_day=$3::date,
 entries_today=CASE WHEN entry_day=$3::date THEN entries_today+1 ELSE 1 END
 WHERE account_id=$1 AND tower_key=$2 AND highest_cleared+1=$4

 RETURNING highest_cleared,entry_day::text,entries_today,last_run_id`, account, policy.Key, day, floor).
		Scan(&out.HighestCleared, &out.EntryDay, &out.EntriesToday, &out.LastRun)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, fmt.Errorf("%s tower entry unavailable", policy.Key)
	}
	return out, err
}

// AdvanceTowerFloor is idempotent for a saved run and never changes the day's
// admission count.
func (s *Store) AdvanceTowerFloor(ctx context.Context, account int64, policy TowerPolicy, floor uint16, run string) (TowerProgress, error) {
	var out TowerProgress
	if account <= 0 || policy.validate() != nil || floor == 0 || floor > policy.TopFloor || len(run) != 32 {
		return out, fmt.Errorf("invalid tower clear")
	}
	err := s.DB.QueryRow(ctx, `UPDATE account_tower_progress
 SET highest_cleared=$3,last_run_id=$4
 WHERE account_id=$1 AND tower_key=$2 AND highest_cleared=$3-1 AND entries_today>0
 RETURNING highest_cleared,COALESCE(entry_day::text,''),entries_today,last_run_id`, account, policy.Key, floor, run).
		Scan(&out.HighestCleared, &out.EntryDay, &out.EntriesToday, &out.LastRun)
	if err == nil {
		return out, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	err = s.DB.QueryRow(ctx, `SELECT highest_cleared,COALESCE(entry_day::text,''),entries_today,last_run_id
 FROM account_tower_progress WHERE account_id=$1 AND tower_key=$2`, account, policy.Key).
		Scan(&out.HighestCleared, &out.EntryDay, &out.EntriesToday, &out.LastRun)
	if err == nil && out.HighestCleared == floor && out.LastRun == run {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	return out, fmt.Errorf("%s tower clear is not the next floor", policy.Key)
}
