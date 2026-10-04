package database

import (
	"context"
	"dfolan/internal/cashshop"
	"dfolan/internal/database/sqlcgen"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	PremiumConqueror = cashshop.PremiumConqueror
	PremiumTactician = cashshop.PremiumTactician
	PremiumGabriel   = cashshop.PremiumGabriel
	PremiumGrowth    = cashshop.PremiumGrowth
	PremiumCube      = cashshop.PremiumCube
	PremiumNeoBasic  = cashshop.PremiumNeoBasic
	PremiumNeoPlus   = cashshop.PremiumNeoPlus
)

func (s *Store) MigratePremiums(ctx context.Context) error {
	return s.execMigration(ctx, "0022_premiums.sql")
}

// ActivePremiums returns the account timers consumed by the character-select
// payload. Expired rows remain auditable in PostgreSQL but are not advertised.
func (s *Store) ActivePremiums(ctx context.Context, account int64, now time.Time) ([]CashPremium, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.queries.ActivePremiums(ctx, sqlcgen.ActivePremiumsParams{AccountID: account, NowEpoch: now.Unix()})
	if err != nil {
		return nil, err
	}
	out := []CashPremium{}
	for _, row := range rows {
		typ, end := row.PremiumType, row.EndTime
		remaining := end - now.Unix()
		if typ > 0 && typ <= 255 && remaining > 0 {
			out = append(out, CashPremium{Type: uint8(typ), EndTime: end, RemainingSecond: remaining})
		}
	}
	return out, nil
}

// HasActivePremium checks if a specific premium contract is currently active for an account.
func (s *Store) HasActivePremium(ctx context.Context, account int64, premiumType uint8, now time.Time) (bool, error) {
	if s == nil || s.db == nil {
		return false, nil
	}
	return s.queries.HasActivePremium(ctx, sqlcgen.HasActivePremiumParams{AccountID: account, PremiumType: int16(premiumType), NowEpoch: now.Unix()})
}

// ActivePremiumSet returns a set of currently active premium types for fast lookups.
func (s *Store) ActivePremiumSet(ctx context.Context, account int64, now time.Time) (map[uint8]bool, error) {
	premiums, err := s.ActivePremiums(ctx, account, now)
	if err != nil {
		return nil, err
	}
	set := make(map[uint8]bool, len(premiums))
	for _, p := range premiums {
		set[p.Type] = true
	}
	return set, nil
}

// ActivatePremium updates the account contract timer atomically and returns the new end_time.
func (s *Store) ActivatePremium(ctx context.Context, account int64, premiumType uint8, durationSecond int64) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("storage unavailable")
	}
	if durationSecond <= 0 {
		return 0, fmt.Errorf("invalid premium duration")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	now := time.Now().Unix()
	q := s.queries.WithTx(tx)
	oldEnd, err := q.LockPremiumExpiry(ctx, sqlcgen.LockPremiumExpiryParams{AccountID: account, PremiumType: int16(premiumType)})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	base := now
	if oldEnd > base {
		base = oldEnd
	}
	end := base + durationSecond
	if end <= base {
		return 0, fmt.Errorf("premium expiry overflow")
	}
	err = q.SavePremiumExpiry(ctx, sqlcgen.SavePremiumExpiryParams{AccountID: account, PremiumType: int16(premiumType), EndTime: end})
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return end, nil
}

// HasGrowthPremium implements the character consumer's growth capability.
func (s *Store) HasGrowthPremium(ctx context.Context, account int64, now time.Time) (bool, error) {
	return s.HasActivePremium(ctx, account, PremiumGrowth, now)
}

// HasTacticianPremium implements the character consumer's skill-cost capability.
func (s *Store) HasTacticianPremium(ctx context.Context, account int64, now time.Time) (bool, error) {
	return s.HasActivePremium(ctx, account, PremiumTactician, now)
}
