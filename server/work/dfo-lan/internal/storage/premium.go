package storage

import (
	"context"
	"dfolan/internal/cashshop"
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
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_premiums(
 account_id bigint NOT NULL REFERENCES accounts(id),
 premium_type smallint NOT NULL CHECK(premium_type BETWEEN 1 AND 255),
 end_time bigint NOT NULL CHECK(end_time>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,premium_type));
 CREATE INDEX IF NOT EXISTS account_premiums_expiry ON account_premiums(account_id,end_time);`)
	return err
}

// ActivePremiums returns the account timers consumed by the character-select
// payload. Expired rows remain auditable in PostgreSQL but are not advertised.
func (s *Store) ActivePremiums(ctx context.Context, account int64, now time.Time) ([]CashPremium, error) {
	if s == nil || s.DB == nil {
		return nil, nil
	}
	rows, err := s.DB.Query(ctx, `SELECT premium_type,end_time FROM account_premiums WHERE account_id=$1 AND end_time>$2 ORDER BY premium_type`, account, now.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CashPremium{}
	for rows.Next() {
		var typ int16
		var end int64
		if err := rows.Scan(&typ, &end); err != nil {
			return nil, err
		}
		remaining := end - now.Unix()
		if typ > 0 && typ <= 255 && remaining > 0 {
			out = append(out, CashPremium{Type: uint8(typ), EndTime: end, RemainingSecond: remaining})
		}
	}
	return out, rows.Err()
}

// HasActivePremium checks if a specific premium contract is currently active for an account.
func (s *Store) HasActivePremium(ctx context.Context, account int64, premiumType uint8, now time.Time) (bool, error) {
	if s == nil || s.DB == nil {
		return false, nil
	}
	var exists bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account_premiums WHERE account_id=$1 AND premium_type=$2 AND end_time>$3)`, account, premiumType, now.Unix()).Scan(&exists)
	return exists, err
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
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("storage unavailable")
	}
	if durationSecond <= 0 {
		return 0, fmt.Errorf("invalid premium duration")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	now := time.Now().Unix()
	var oldEnd int64
	err = tx.QueryRow(ctx, `SELECT end_time FROM account_premiums WHERE account_id=$1 AND premium_type=$2 FOR UPDATE`, account, premiumType).Scan(&oldEnd)
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
	_, err = tx.Exec(ctx, `INSERT INTO account_premiums(account_id,premium_type,end_time,updated_at) VALUES($1,$2,$3,now()) ON CONFLICT(account_id,premium_type) DO UPDATE SET end_time=EXCLUDED.end_time,updated_at=now()`, account, premiumType, end)
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
