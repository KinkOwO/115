package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

// PackedEntryStats stores the native wire units, not display units. Mapping
// source attributes into these units is a separate character/rules concern.
// Current native conversion: 147555f80, exactly 91 bytes -> 40 protected words.
type PackedEntryStats struct {
	HP, MP        uint32
	Core          [4]uint16
	Element       [4]int16
	Status        [19]int16
	Inventory     int32
	Regeneration  [2]int16
	Movement      uint32
	AttackCasting [2]uint16
	RecoveryJump  [2]int16
	Weight        int32
	BasePercent   byte
	Extra         float32
}

func (s PackedEntryStats) Bytes() ([]byte, error) {
	if s.HP == 0 || s.MP == 0 || s.BasePercent == 0 || math.IsNaN(float64(s.Extra)) || math.IsInf(float64(s.Extra), 0) {
		return nil, fmt.Errorf("incomplete or non-finite entry stats")
	}
	p := add32(add32(nil, s.HP), s.MP)
	for _, v := range s.Core {
		p = add16(p, v)
	}
	for _, v := range s.Element {
		p = add16(p, uint16(v))
	}
	for _, v := range s.Status {
		p = add16(p, uint16(v))
	}
	p = add32(p, uint32(s.Inventory))
	for _, v := range s.Regeneration {
		p = add16(p, uint16(v))
	}
	p = add32(p, s.Movement)
	for _, v := range s.AttackCasting {
		p = add16(p, v)
	}
	for _, v := range s.RecoveryJump {
		p = add16(p, uint16(v))
	}
	p = add32(p, uint32(s.Weight))
	p = add32(append(p, s.BasePercent), math.Float32bits(s.Extra))
	return p, nil
}

type EntrySkill struct {
	ID    uint16
	Level byte
}

// SkillTreeLocked is the "skill type" selector wire value that the client reads
// as "the second skill page was never unlocked". 86JP
// DfoServer.Game.Skills.SkillTreeExpansionState calls the same value
// LockedWireValue; 0/1 mean unlocked and currently selected page.
const SkillTreeLocked = 0xff

// SkillTreeWireIndex 把 1-based 的技能类型选择（0=未解锁 / 1=类型1 / 2=类型2）
// 投影成客户端原生选择字节（0xff / 0 / 1）。任何越界值都退回锁定，
// 保证存档损坏或字段漏设时不会意外解锁第二页。
func SkillTreeWireIndex(selection byte) byte {
	if selection >= 1 && selection <= 2 {
		return selection - 1
	}
	return SkillTreeLocked
}

// EntryAdditionProbe is the minimum current mode 1 layout with explicitly
// absent optional equipment/collections. It must follow a matching mode 0.
// Source base stats are connected to the detailed probe. Unknown fixed-prefix
// fields are experimental zeros; optional skill/equipment data remains pending.
type EntryAdditionProbe struct {
	Fame           uint32
	AdventureLevel uint32
	ActorServerID  uint16
	Context        [2]byte
	Experience     uint64
	Stats          PackedEntryStats
	SkillTrees     [2][]EntrySkill
	Worn           []DetailedWorn
	// ExpandEquipFlags is the saved extended-slot unlock byte (support 1,
	// magic stone 2, earring 16). Native 14563d692 reads it straight into the
	// character's +0x198 and 145cf28f0 gates equipment slots 22/23/25 on those
	// bits, so the armoury cannot be opened through any other packet - the
	// NOTI328 refresh only re-reads a state it already has. The type is the
	// width guard: the native field is one byte and a wider saved value is
	// rejected while projecting rather than truncated here.
	ExpandEquipFlags byte
	// SkillTreeType 决定客户端显示哪一页技能：0 = 第二技能页从未解锁
	// （零值，所以老存档以及任何忘记赋值的调用方都保持锁定），
	// 1 = 技能类型 1（第一页），2 = 技能类型 2（第二页）。
	// 下发时经 SkillTreeWireIndex 投影成原生选择字节：0 -> SkillTreeLocked，
	// 1 -> 0，2 -> 1。86JP DfoServer.Game.Skills.SkillTreeExpansionState 用的是
	// 同一组 wire 值。这个字节过去被硬编码成 SkillTreeLocked，所以任何角色都开
	// 不了第二页。
	SkillTreeType byte
}

func UserInfoAdditionProbe(s EntryAdditionProbe) ([]byte, error) {
	if s.ActorServerID == 0 || s.ActorServerID == 65535 {
		return nil, fmt.Errorf("invalid addition actor identity")
	}
	stats, e := s.Stats.Bytes()
	if e != nil {
		return nil, e
	}
	p := append(add16([]byte{1}, 1), s.Context[:]...)
	p = append(p, make([]byte, 250)...)
	// 14563d472 读取到 14e66f260；14563e1e8 从 +0x13 取名望并调用相同的 setter。
	binary.LittleEndian.PutUint32(p[5+0x13:], s.Fame)
	p = add16(p, s.ActorServerID)
	p = add32(add32(p, uint32(s.Experience)), uint32(s.Experience>>32))
	p = append(add32(p, uint32(len(stats))), stats...)
	// 14563d692: the character's extended-slot unlock byte. This slot used to
	// be written as a fixed zero, which is why a quest could settle without
	// the armoury ever opening. Tests pin it at payload offset 360.
	p = append(p, s.ExpandEquipFlags)
	// 14563d6cb -> 1452c1540 always consumes an equipment block, even
	// when empty: u8 rows, u32 scalar, u8 collection count, u64 flags.
	// Omitting these 14 bytes shifts switching inventory and skill trees.
	equipment, e := DetailedEquipment(s.Worn)
	if e != nil {
		return nil, e
	}
	p = append(p, equipment...)
	p = append(add16(p, 0), 0) // 14563c1f0: switching-inventory ID + count
	p = add32(add32(p, 0), 0)  // 14563d6dd / 14563d717
	// 原生"技能类型"选择字节，见 EntryAdditionProbe.SkillTreeType。
	p = append(p, SkillTreeWireIndex(s.SkillTreeType))
	// 14563d9aa increments the outer tree index; both self and other branches
	// consume TWO sets of count + skills + three pairs + five triples.
	for _, tree := range s.SkillTrees {
		if len(tree) > 255 {
			return nil, fmt.Errorf("skill tree exceeds native count")
		}
		p = append(p, byte(len(tree)))
		seen := map[uint16]bool{}
		for _, skill := range tree {
			if skill.ID == 0 || skill.Level == 0 || seen[skill.ID] {
				return nil, fmt.Errorf("invalid or duplicate skill")
			}
			seen[skill.ID] = true
			p = append(add16(p, skill.ID), skill.Level)
		}
		p = append(p, make([]byte, 3*6+5*7)...)
	}
	p = append(p, 0, 0, 0, 0) // creature byte, extended skills count, two skill flags
	// 14563DBF8 读取冒险团等级，145D2C790 保存后由 145C9EA10 按源表重算加成。
	p = add32(add32(p, 0), s.AdventureLevel)
	p = append(p, 0, 0) // 1456395a0 count, 14563dd04 byte
	return p, nil
}
