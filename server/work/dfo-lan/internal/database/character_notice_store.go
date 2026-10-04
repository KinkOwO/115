package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
	"math"
)

// MigrateCharacterNotices creates the per-character read-notice ledger. Tree 1
// backs NOTI402 (INFORM_NOTICE), tree 2 backs NOTI426 (INFORM_NOTICE_2ND);
// the third-awakening teaching frame rides tree 2 with notice id 62. The
// ledger is per-character by design: the client asks each character to read
// the guide once, so switching characters must re-pop it.
func (s *Store) MigrateCharacterNotices(ctx context.Context) error {
	return s.execMigration(ctx, "0003_character_notices.sql")
}

// MarkCharacterNotice records or removes one read notice. seen=false deletes
// the row so the popup can come back when the client unchecks
// "Don't show this again" (value 0).
func (s *Store) MarkCharacterNotice(ctx context.Context, account, character int64, tree byte, notice uint16, seen bool) error {
	if tree > 2 {
		return fmt.Errorf("invalid notice tree")
	}
	// The persisted smallint cannot represent the upper half of uint16. Reject
	// before conversion, preserving the old driver's out-of-range behavior.
	if notice > math.MaxInt16 {
		return fmt.Errorf("notice id out of storage range")
	}
	if seen {
		return s.queries.MarkCharacterNotice(ctx, sqlcgen.MarkCharacterNoticeParams{
			AccountID: account, CharacterID: character, Tree: int16(tree), NoticeID: int16(notice),
		})
	}
	return s.queries.UnmarkCharacterNotice(ctx, sqlcgen.UnmarkCharacterNoticeParams{
		AccountID: account, CharacterID: character, Tree: int16(tree), NoticeID: int16(notice),
	})
}

// CharacterNoticeSeen returns the read notice ids of one tree, ascending.
func (s *Store) CharacterNoticeSeen(ctx context.Context, account, character int64, tree byte) ([]uint16, error) {
	if tree > 2 {
		return nil, fmt.Errorf("invalid notice tree")
	}
	ids, e := s.queries.CharacterNoticeSeen(ctx, sqlcgen.CharacterNoticeSeenParams{
		AccountID: account, CharacterID: character, Tree: int16(tree),
	})
	if e != nil {
		return nil, e
	}
	out := make([]uint16, len(ids))
	for i, id := range ids {
		out[i] = uint16(id)
	}
	return out, nil
}
