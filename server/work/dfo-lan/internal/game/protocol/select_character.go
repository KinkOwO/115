package protocol

import (
	"encoding/binary"
	"fmt"
)

// Captured current-build request: LE u32 roster position followed by twelve
// zero bytes. The later option meanings are not recovered yet.
func DecodeSelectRequest(p []byte) (uint32, error) {
	if len(p) != 16 {
		return 0, fmt.Errorf("select request must have 16 bytes")
	}
	for _, v := range p[4:] {
		if v != 0 {
			return 0, fmt.Errorf("unsupported select options")
		}
	}
	return binary.LittleEndian.Uint32(p), nil
}

func DecodeUserInfoRequest(p []byte) (uint16, byte, error) {
	if len(p) < 3 {
		return 0, 0, fmt.Errorf("short userinfo request")
	}
	if e := padding(p[3:], 8); e != nil {
		return 0, 0, e
	}
	return binary.LittleEndian.Uint16(p), p[2], nil
}

// SelectProbeState describes an explicit parser experiment. These fields do
// not claim official initial world/tutorial rules or implement town entry.
// Layout: 0x14525a120, nested 0x144f58e10 and 0x146cc6fd0.
type PremiumEntry struct {
	Type            uint8 `json:"type"`
	RemainingSecond int64 `json:"remaining_second"`
}

type SelectProbeState struct {
	ActiveQuests      []ActiveQuest  `json:"-"`
	Premiums          []PremiumEntry `json:"premiums,omitempty"`
	CreatedTime       uint32         `json:"created_time"`
	UnknownPrefix     uint32         `json:"unknown_prefix"`
	ActorServerID     uint16         `json:"actor_server_id"`
	Fatigue           [3]uint16      `json:"fatigue_fields"`
	Cash              uint32         `json:"cash"`
	QuestFields       [4]uint32      `json:"quest_fields"`
	WorldKind         uint32         `json:"world_kind"`
	TutorialFlag      byte           `json:"tutorial_flag"`
	TutorialCompleted []byte         `json:"tutorial_completed"`
	Tail16            [2]uint16      `json:"tail_u16"`
	ArenaBroadcast    byte           `json:"arena_broadcast"`
	FatigueTail       uint16         `json:"fatigue_tail"`
}

func SelectProbeSuccess(s SelectProbeState) ([]byte, error) {
	if len(s.ActiveQuests) > 4096 {
		return nil, fmt.Errorf("too many active quests")
	}
	seen := map[uint16]bool{}
	for _, q := range s.ActiveQuests {
		if q.ID == 0 || q.ID == 65535 || seen[q.ID] {
			return nil, fmt.Errorf("invalid or duplicate active quest")
		}
		seen[q.ID] = true
	}
	if len(s.TutorialCompleted) > 101 {
		return nil, fmt.Errorf("too many tutorial entries")
	}
	for _, v := range s.TutorialCompleted {
		if v >= 101 {
			return nil, fmt.Errorf("tutorial index out of range")
		}
	}
	// Native log arguments at 0x14525a58e..0x14525a5a9 identify the second
	// u32 as createdTime and the following u16 as MyServerId. The first u32
	// is read into an otherwise unused local. Keep roster slot distinct.
	p := add16(add32(add32([]byte{1}, s.UnknownPrefix), s.CreatedTime), s.ActorServerID)
	for _, v := range s.Fatigue {
		p = add16(p, v)
	}
	if len(s.Premiums) > 255 {
		return nil, fmt.Errorf("too many premium entries")
	}
	p = append(p, byte(len(s.Premiums)))
	seenPremium := map[uint8]bool{}
	for _, premium := range s.Premiums {
		if premium.Type == 0 || premium.RemainingSecond <= 0 || seenPremium[premium.Type] {
			return nil, fmt.Errorf("invalid premium entry")
		}
		seenPremium[premium.Type] = true
		p = append(p, premium.Type)
		// 14525a429 将这个 u64 交给 1459b5a10 原样保存。
		// 1456d9bf0 经 145241f80 读取后直接除以 86400 显示天数，
		// 因此必须发送剩余秒数，不能发送数据库的 Unix 到期时间戳。
		for i := 0; i < 8; i++ {
			p = append(p, byte(uint64(premium.RemainingSecond)>>uint(8*i)))
		}
	}
	p = add32(p, s.Cash)
	for i := 0; i < 30; i++ {
		q := ActiveQuest{ID: 0xffff}
		if i < len(s.ActiveQuests) {
			q = s.ActiveQuests[i]
		}
		p = add32(add16(p, q.ID), q.Progress)
	}
	// Same u16 ID/u32 remaining layout after the first 30 slots.
	overflow := 0
	if len(s.ActiveQuests) > 30 {
		overflow = len(s.ActiveQuests) - 30
	}
	p = add32(p, uint32(overflow))
	for i := 30; i < len(s.ActiveQuests); i++ {
		q := s.ActiveQuests[i]
		p = add32(add16(p, q.ID), q.Progress)
	}
	for _, v := range s.QuestFields {
		p = add32(p, v)
	}
	p = add32(p, s.WorldKind)
	p = append(p, s.TutorialFlag, byte(len(s.TutorialCompleted)))
	p = append(p, s.TutorialCompleted...)
	for _, v := range s.Tail16 {
		p = add16(p, v)
	}
	p = append(p, s.ArenaBroadcast)
	p = add16(p, s.FatigueTail)
	p = append(p, 0) // second quest collection count, u8
	p = add32(p, 0)  // third quest collection count, u32
	return p, nil
}
