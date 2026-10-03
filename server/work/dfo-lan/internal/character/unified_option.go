package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

// 账号没有保存此选项时返回0，让协议层保留原有默认显示。
func growthEffectFlags(options map[uint16]uint16) byte {
	value, ok := options[protocol.GrowthEffectOption]
	if !ok {
		return 0
	}
	flags, _ := protocol.GrowthEffectFlags(value)
	return flags
}

func (s *Service) roleGrowthEffectFlags(role Character) (byte, error) {
	if s.Store == nil {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	options, err := s.Store.AccountUnifiedOptions(ctx, role.AccountID)
	if err != nil {
		return 0, err
	}
	return growthEffectFlags(options), nil
}

// 1403F64D0读取角色设置分组5的第98项；1403DFE70仅将1视为显示光环。
// 原版etc/unifiedoption/unifiedoption.ctp的CAEE默认值为1；未保存或
// 0xffff沿用这个默认值，不覆盖玩家主动关闭的设置。
func auraEffectVisible(options map[uint16]uint16) bool {
	value, ok := options[98]
	return !ok || value == 0xffff || value == 1
}

// SaveSkillLocks merges one CMD2377 skill lock frame (subtype 0x13) into the
// character's stored set.
//
// Slot positions are never persisted: the client only sends the slots that
// differ from its last baseline, so the server keeps the id set and rebuilds the
// compact pages from it. That is also why a frame whose first position is a page
// boundary rebuilds that page - the client re-states the page it owns instead of
// sending an incremental delta.
func (s *Service) SaveSkillLocks(ctx context.Context, role Character, key string, opt protocol.UnifiedOption) ([]uint16, bool, error) {
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
