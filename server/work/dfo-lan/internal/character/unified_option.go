package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"fmt"
)

// SaveSkillLocks merges one CMD2377 skill lock frame (subtype 0x13) into the
// character's stored set.
//
// Slot positions are never persisted: the client only sends the slots that
// differ from its last baseline, so the server keeps the id set and rebuilds the
// compact pages from it. That is also why a frame whose first position is a page
// boundary rebuilds that page - the client re-states the page it owns instead of
// sending an incremental delta.
func (s *Service) SaveSkillLocks(ctx context.Context, role storage.Character, key string, opt protocol.UnifiedOption) ([]uint16, bool, error) {
	if s.Store == nil {
		return nil, false, fmt.Errorf("skill lock storage unavailable")
	}
	if opt.Subtype != protocol.UnifiedOptionSkillLock {
		return nil, false, fmt.Errorf("unsupported unified option subtype")
	}
	if role.ID == 0 || role.AccountID == 0 {
		return nil, false, fmt.Errorf("skill lock requires an owned selected character")
	}
	return s.Store.CommitSkillLocks(ctx, role.AccountID, role.ID, key, "skill-lock-v1", func(current []uint16) ([]uint16, error) {
		return protocol.MergeSkillLocks(current, opt.Entries), nil
	})
}

// SkillLockBlock encodes the character option block that restores the locks.
//
// BUILD GAP: NOTI2827 carries the whole character option payload (3539 bytes
// according to the forwarded note), and the offset of this block inside that
// payload has no local evidence yet - the server has never sent 2827 and the
// client never sends one back, so no captured frame shows the surrounding
// layout. EncodeSkillLockBlock is therefore only the block half; the caller
// decides where it lands.
func SkillLockBlock(locks []uint16) ([]byte, error) {
	return protocol.EncodeSkillLockBlock(locks)
}
