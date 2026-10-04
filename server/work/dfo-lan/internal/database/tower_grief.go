package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
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
	return s.execMigration(ctx, "0034_tower_grief.sql")
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
		row, err := s.queries.ReadTowerGriefProgress(ctx, account)
		progress = TowerGriefProgress{HighestCleared: uint16(row.HighestCleared), ClearedDay: row.ClearedDay, LastRun: row.LastRunID}
		return err
	}
	if err := read(); err == nil {
		return progress, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return progress, err
	}
	states, err := s.queries.AccountCharacterStates(ctx, account)
	if err != nil {
		return progress, err
	}
	highest, err := legacyTowerGriefFloor(states, floors)
	if err != nil {
		return progress, err
	}
	if err = s.queries.EnsureTowerGriefProgress(ctx, sqlcgen.EnsureTowerGriefProgressParams{AccountID: account, HighestCleared: int32(highest)}); err != nil {
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
	row, err := s.queries.AdvanceTowerGrief(ctx, sqlcgen.AdvanceTowerGriefParams{AccountID: account, Floor: int32(floor), Day: day, RunID: run})
	progress = TowerGriefProgress{HighestCleared: uint16(row.HighestCleared), ClearedDay: row.ClearedDay, LastRun: row.LastRunID}
	if err == nil {
		return progress, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return progress, err
	}
	prior, err := s.queries.ReadTowerGriefProgress(ctx, account)
	progress = TowerGriefProgress{HighestCleared: uint16(prior.HighestCleared), ClearedDay: prior.ClearedDay, LastRun: prior.LastRunID}
	if err == nil && progress.HighestCleared == floor && progress.LastRun == run {
		return progress, nil
	}
	if err != nil {
		return progress, err
	}
	return progress, fmt.Errorf("Tower of Grief clear is not the next available floor")
}
