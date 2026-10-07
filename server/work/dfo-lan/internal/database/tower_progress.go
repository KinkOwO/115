package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
	"regexp"
	"time"

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
	return s.execMigration(ctx, "0035_tower_progress.sql")
}

func storedTowerProgress(row sqlcgen.ReadTowerProgressRow) TowerProgress {
	return TowerProgress{HighestCleared: uint16(row.HighestCleared), EntryDay: row.EntryDay, EntriesToday: uint16(row.EntriesToday), LastRun: row.LastRunID}
}

func (s *Store) ReadTowerProgress(ctx context.Context, account int64, policy TowerPolicy, legacyFloor uint16) (TowerProgress, error) {
	var out TowerProgress
	if account <= 0 || policy.validate() != nil || legacyFloor > policy.TopFloor {
		return out, fmt.Errorf("invalid tower progress request")
	}
	if err := s.queries.EnsureTowerProgress(ctx, sqlcgen.EnsureTowerProgressParams{AccountID: account, TowerKey: policy.Key, HighestCleared: int32(legacyFloor)}); err != nil {
		return out, err
	}
	row, err := s.queries.ReadTowerProgress(ctx, sqlcgen.ReadTowerProgressParams{AccountID: account, TowerKey: policy.Key})
	return storedTowerProgress(row), storageError(err)
}

// ReserveTowerEntry records each admission at entry, regardless of clear result.
// WIP testing: daily entry limits are temporarily disabled for every tower.
func (s *Store) ReserveTowerEntry(ctx context.Context, account int64, policy TowerPolicy, floor uint16, now time.Time) (TowerProgress, error) {
	var out TowerProgress
	if account <= 0 || policy.validate() != nil || floor == 0 || floor > policy.TopFloor {
		return out, fmt.Errorf("invalid tower entry")
	}
	day := policy.ServiceDay(now)
	row, err := s.queries.ReserveTowerEntry(ctx, sqlcgen.ReserveTowerEntryParams{AccountID: account, TowerKey: policy.Key, Day: day, Floor: int32(floor)})
	out = storedTowerProgress(sqlcgen.ReadTowerProgressRow(row))
	if isNoRows(err) {
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
	row, err := s.queries.AdvanceTowerFloor(ctx, sqlcgen.AdvanceTowerFloorParams{AccountID: account, TowerKey: policy.Key, Floor: int32(floor), RunID: run})
	out = storedTowerProgress(sqlcgen.ReadTowerProgressRow(row))
	if err == nil {
		return out, nil
	}
	if !isNoRows(err) {
		return out, err
	}
	prior, err := s.queries.ReadTowerProgress(ctx, sqlcgen.ReadTowerProgressParams{AccountID: account, TowerKey: policy.Key})
	out = storedTowerProgress(prior)
	if err == nil && out.HighestCleared == floor && out.LastRun == run {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	return out, fmt.Errorf("%s tower clear is not the next floor", policy.Key)
}
