package storage

import (
	"context"
	"time"
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
