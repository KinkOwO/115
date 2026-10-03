package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type TowerGriefProgress struct {
	HighestCleared uint16
	ClearedDay     string
	LastRun        string
}

var TowerGriefPolicy = TowerPolicy{Key: "grief", TopFloor: 100, DailyEntries: 1, ResetHourUTC: 9}

// MigrateTowerGriefProgress adds account-scoped progress without changing any
// existing character state. Old clear records are imported on first read.
func (s *Store) MigrateTowerGriefProgress(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_tower_grief_progress (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 highest_cleared smallint NOT NULL DEFAULT 0 CHECK(highest_cleared BETWEEN 0 AND 100),
 cleared_day date,
 last_run_id text NOT NULL DEFAULT ''
 );`)
	return err
}

// TowerGriefServiceDay changes at the original tower's 09:00 UTC reset.
func TowerGriefServiceDay(now time.Time) string {
	return now.UTC().Add(-9 * time.Hour).Format("2006-01-02")
}

// TowerGriefProgress seeds an account from already saved character clear
// records when upgrading an existing database. The source-verified floor map
// is provided by the caller, so no arithmetic dungeon-ID guess is needed.
func (s *Store) TowerGriefProgress(ctx context.Context, account int64, floors [101]uint32) (TowerGriefProgress, error) {
	var progress TowerGriefProgress
	if account <= 0 {
		return progress, fmt.Errorf("invalid account")
	}
	for floor := 1; floor <= 100; floor++ {
		if floors[floor] == 0 {
			return progress, fmt.Errorf("incomplete Tower of Grief floor map")
		}
	}
	read := func() error {
		return s.DB.QueryRow(ctx, `SELECT highest_cleared, COALESCE(cleared_day::text,''), last_run_id
 FROM account_tower_grief_progress WHERE account_id=$1`, account).
			Scan(&progress.HighestCleared, &progress.ClearedDay, &progress.LastRun)
	}
	if err := read(); err == nil {
		return progress, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return progress, err
	}
	rows, err := s.DB.Query(ctx, `SELECT state FROM characters WHERE account_id=$1`, account)
	if err != nil {
		return progress, err
	}
	var states []json.RawMessage
	for rows.Next() {
		var state json.RawMessage
		if err = rows.Scan(&state); err != nil {
			rows.Close()
			return progress, err
		}
		states = append(states, state)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return progress, err
	}
	highest, err := legacyTowerGriefFloor(states, floors)
	if err != nil {
		return progress, err
	}
	if _, err = s.DB.Exec(ctx, `INSERT INTO account_tower_grief_progress(account_id,highest_cleared)
 VALUES($1,$2) ON CONFLICT(account_id) DO NOTHING`, account, highest); err != nil {
		return progress, err
	}
	return progress, read()
}

func legacyTowerGriefFloor(states []json.RawMessage, floors [101]uint32) (uint16, error) {
	cleared := make(map[string]bool)
	for _, state := range states {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(state, &fields); err != nil {
			return 0, err
		}
		if len(fields["dungeon_best_times"]) == 0 {
			continue
		}
		var records map[string]uint32
		if err := json.Unmarshal(fields["dungeon_best_times"], &records); err != nil {
			return 0, err
		}
		for key, elapsed := range records {
			if elapsed != 0 {
				cleared[key] = true
			}
		}
	}
	var highest uint16
	for floor := 1; floor <= 100; floor++ {
		key := fmt.Sprintf("%d:normal:solo", floors[floor])
		if !cleared[key] {
			break
		}
		highest = uint16(floor)
	}
	return highest, nil
}

// AdvanceTowerGrief only commits the next floor once per service day. A retry
// of the same cleared run is idempotent after the character clear was saved.
func (s *Store) AdvanceTowerGrief(ctx context.Context, account int64, floor uint16, run string, now time.Time) (TowerGriefProgress, error) {
	var progress TowerGriefProgress
	if account <= 0 || floor == 0 || floor > 100 || len(run) != 32 {
		return progress, fmt.Errorf("invalid Tower of Grief clear")
	}
	day := TowerGriefServiceDay(now)
	err := s.DB.QueryRow(ctx, `UPDATE account_tower_grief_progress
 SET highest_cleared=$2, cleared_day=$3::date, last_run_id=$4
 WHERE account_id=$1 AND highest_cleared=$2-1
 AND (cleared_day IS NULL OR cleared_day<>$3::date)
 RETURNING highest_cleared, cleared_day::text, last_run_id`, account, floor, day, run).
		Scan(&progress.HighestCleared, &progress.ClearedDay, &progress.LastRun)
	if err == nil {
		return progress, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return progress, err
	}
	err = s.DB.QueryRow(ctx, `SELECT highest_cleared, COALESCE(cleared_day::text,''), last_run_id
 FROM account_tower_grief_progress WHERE account_id=$1`, account).
		Scan(&progress.HighestCleared, &progress.ClearedDay, &progress.LastRun)
	if err == nil && progress.HighestCleared == floor && progress.LastRun == run {
		return progress, nil
	}
	if err != nil {
		return progress, err
	}
	return progress, fmt.Errorf("Tower of Grief clear is not the next available floor")
}
