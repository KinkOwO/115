package storage

import (
	"context"
	"time"
)

const (
	PremiumConqueror uint8 = 22
	PremiumTactician uint8 = 27
	PremiumGabriel   uint8 = 73
	PremiumGrowth    uint8 = 79
	PremiumCube      uint8 = 92
	PremiumNeoBasic  uint8 = 117
	PremiumNeoPlus   uint8 = 118
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
