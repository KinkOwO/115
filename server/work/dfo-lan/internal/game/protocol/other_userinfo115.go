package protocol

import (
	"encoding/binary"
	"fmt"
)

// OtherUserInfo115 is the mode-3 view for an already introduced synthetic
// town actor. It has no account, guild, social history or account collections.
// 145637a20 -> 145641040; native read vectors are kept separately from self
// mode 1. The item block uses the same 1452c1540 reader as TagEquipment.
type OtherUserInfo115 struct {
	ActorServerID    uint16
	Context          [2]byte
	Experience       uint64
	Stats            PackedEntryStats
	Worn             []DetailedWorn
	ExpandEquipFlags byte
	Fame             uint32
	SkillTrees       [2][]EntrySkill
}

func OtherUserInfoReply115(s OtherUserInfo115) ([]byte, error) {
	if s.ActorServerID == 0 || s.ActorServerID == 65535 {
		return nil, fmt.Errorf("invalid inspected actor")
	}
	stats, err := s.Stats.Bytes()
	if err != nil {
		return nil, err
	}
	equipment, err := TagEquipment(s.Worn)
	if err != nil {
		return nil, err
	}
	p := append(add16([]byte{3}, 1), s.Context[:]...)
	// 14564109f reads the complete 0x110 header. Synthetic actors have no
	// account metadata: all flags, inline labels and account counters absent.
	// +0x19 -> 14e66f379 -> 145f05f60 is the worn equipment fame value.
	header := make([]byte, 0x110)
	binary.LittleEndian.PutUint32(header[0x19:], s.Fame)
	p = append(p, header...)
	p = add16(p, s.ActorServerID)
	p = add32(add32(p, uint32(s.Experience)), uint32(s.Experience>>32))
	p = append(add32(p, uint32(len(stats))), stats...)
	p = append(p, s.ExpandEquipFlags)
	p = append(p, equipment...)
	p = add32(add32(p, 0), 0)
	p = append(p, 0xff) // 14563ccb0: no selected tree override
	for _, tree := range s.SkillTrees {
		if len(tree) > 255 {
			return nil, fmt.Errorf("too many inspected skills")
		}
		p = append(p, byte(len(tree)))
		seen := make(map[uint16]bool)
		for _, skill := range tree {
			if skill.ID == 0 || skill.Level == 0 || seen[skill.ID] {
				return nil, fmt.Errorf("invalid inspected skill")
			}
			seen[skill.ID] = true
			p = append(add16(p, skill.ID), skill.Level)
		}
		p = append(p, make([]byte, 3*6+5*7)...)
	}
	p = append(p, 0)                   // creature status flag
	p = append(p, make([]byte, 17)...) // 145639180: empty social profile
	p = add32(p, 0)
	// 14563a690: u16, u32, length-prefixed label, two flags (no guild).
	p = append(p, make([]byte, 12)...)
	p = add32(p, 0)                    // additional skill IDs
	p = append(p, 0)                   // 1456395a0 empty appearance collection
	p = append(p, make([]byte, 12)...) // 14564193d..94f three terminal u32
	return p, nil
}
