package storage

import (
	"context"
	"fmt"
)

// MigrateCharacterNotices creates the per-character read-notice ledger. Tree 1
// backs NOTI402 (INFORM_NOTICE), tree 2 backs NOTI426 (INFORM_NOTICE_2ND);
// the third-awakening teaching frame rides tree 2 with notice id 62. The
// ledger is per-character by design: the client asks each character to read
// the guide once, so switching characters must re-pop it.
func (s *Store) MigrateCharacterNotices(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_notice_seen (
 account_id bigint NOT NULL REFERENCES accounts(id),
 character_id bigint NOT NULL,
 tree smallint NOT NULL,
 notice_id smallint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id, character_id, tree, notice_id));`)
	return e
}

// MarkCharacterNotice records or removes one read notice. seen=false deletes
// the row so the popup can come back when the client unchecks
// "Don't show this again" (value 0).
func (s *Store) MarkCharacterNotice(ctx context.Context, account, character int64, tree byte, notice uint16, seen bool) error {
	if tree > 2 {
		return fmt.Errorf("invalid notice tree")
	}
	if seen {
		_, e := s.DB.Exec(ctx, `INSERT INTO character_notice_seen(account_id, character_id, tree, notice_id) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, account, character, tree, notice)
		return e
	}
	_, e := s.DB.Exec(ctx, `DELETE FROM character_notice_seen WHERE account_id=$1 AND character_id=$2 AND tree=$3 AND notice_id=$4`, account, character, tree, notice)
	return e
}

// CharacterNoticeSeen returns the read notice ids of one tree, ascending.
func (s *Store) CharacterNoticeSeen(ctx context.Context, account, character int64, tree byte) ([]uint16, error) {
	if tree > 2 {
		return nil, fmt.Errorf("invalid notice tree")
	}
	rows, e := s.DB.Query(ctx, `SELECT notice_id FROM character_notice_seen WHERE account_id=$1 AND character_id=$2 AND tree=$3 ORDER BY notice_id`, account, character, tree)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []uint16{}
	for rows.Next() {
		var n int
		if e = rows.Scan(&n); e != nil {
			return nil, e
		}
		out = append(out, uint16(n))
	}
	return out, rows.Err()
}
