package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"sort"
)

// Entry holds one quest's static, source-derived properties. Everything here
// is decided by the imported catalog alone; nothing depends on a character.
type Entry struct {
	ID                 uint32
	Kind               string
	Model              string
	Initial            uint32
	Implemented        bool
	RewardUsable       bool
	GrowUsable         bool
	MinimumLevel       uint32
	MaximumLevel       uint32
	Jobs               []string
	Prerequisites      []uint32
	PrerequisiteGroups [][]uint32
	GrowTypes          []int32
	NPC                uint32
	NPCReach           NPCReachObjective
	Range              RangeObjective
	Seek               SeekObjective
	HuntDungeon        uint32
	HuntEnemy          uint32
}

// Index precomputes those properties once per catalog load. The hot paths —
// the available list, a run's map-clear settlement and the world proximity
// check — otherwise re-walk all 2844 quests and re-scan each quest's own
// script cells on every level-up, clear and submit.
type Index struct {
	Source     string
	Ordered    []uint32
	Entries    map[uint32]*Entry
	ByClearMap map[uint32][]uint16
	Positional []uint32
}

// slotExpansion returns the equipment-slot index a quest unlocks.
//
// Source quests 649 / 650 / 2636 unlock the extended equipment slots and carry
// [reward type] = [slot expansion] with a single [reward int data] number that
// is the slot index, not an item tuple:
//
//	649  (level 60)  epic_60_nightassault_second_2.qst        0  support
//	650  (level 65)  epic_65_pirateonthetrain_magic_stone_2   1  magic stone
//	2636 (level 90)  system_90_earring_02.qst                 2  earring
//
// The integer therefore must never reach ItemRewards, which reads a bare [0] as
// "item id 0 with no count" and reports an invalid reward row.
func slotExpansion(d catalog.QuestDefinition) (byte, bool) {
	t := cells(d.Script.Cells, "[reward type]")
	if len(t) != 1 || t[0].Type != 6 || t[0].Text != "[slot expansion]" {
		return 0, false
	}
	ids := cells(d.Script.Cells, "[reward int data]")
	if len(ids) != 1 || ids[0].Type != 0 || ids[0].Value < 0 || ids[0].Value > 3 {
		return 0, false
	}
	return byte(ids[0].Value), true
}

// The unlock bits the armoury rebuilds its padlocks from, saved in the bag as
// expand_equip_flags. These are NOT the reward scalars: the scalar is a slot
// index and the earring's index 2 corresponds to bit 4, so OR-ing the scalar
// directly would set bit 2 and leave the earring locked.
//
// The bits themselves live with the bag they are OR-ed into
// (internal/inventory); the names here keep the quest-reward vocabulary.
const (
	ExpandSupport    = inventory.ExpandSupport    // reward scalar 0, equipment slot 22
	ExpandMagicStone = inventory.ExpandMagicStone // reward scalar 1, equipment slot 23
	ExpandEarring    = inventory.ExpandEarring    // reward scalar 2, equipment slot 25
)

// slotUnlockMask maps a [slot expansion] reward scalar to its unlock bit.
func slotUnlockMask(slot byte) (byte, bool) {
	switch slot {
	case 0:
		return ExpandSupport, true
	case 1:
		return ExpandMagicStone, true
	case 2:
		return ExpandEarring, true
	}
	return 0, false
}

// rewardUsable reports whether Finish can settle this quest's reward today.
// Quests it rejects must also stay out of the available list: a character who
// accepts one can never submit it, which reads in game as a quest that will
// not complete no matter how often it is handed in.
//
// [slot expansion] was added 2026-09-22. Until then only [item] and [none] were
// accepted, so the three extended-slot quests above were filtered out of the
// available list: the character could never accept them, the armoury stayed
// locked (live: the support and magic-stone cells of a level-84 Odyssey
// character still showed padlocks), and the client reported CMD 390
// EXPAND_EQUIPSLOT_FLAG_UPDATE with an all-zero flag body.
func rewardUsable(d catalog.QuestDefinition) bool {
	if t := cells(d.Script.Cells, "[reward type]"); len(t) > 0 {
		if len(t) != 1 || t[0].Type != 6 {
			return false
		}
		switch t[0].Text {
		case "[item]", "[none]":
		case "[slot expansion]":
			if _, ok := slotExpansion(d); !ok {
				return false
			}
		default:
			return false
		}
	}
	return len(cells(d.Script.Cells, "[reward select int data]")) == 0 &&
		len(cells(d.Script.Cells, "[reward selection int data]")) == 0
}

func BuildIndex(c catalog.QuestCatalog) *Index {
	x := &Index{Source: c.Source.Checksum, Entries: map[uint32]*Entry{}, ByClearMap: map[uint32][]uint16{}}
	for id, d := range c.Quests {
		initial, model, err := InitialProgress(d)
		groups := d.PrerequisiteGroups
		if len(groups) == 0 && len(d.Prerequisites) > 0 {
			groups = [][]uint32{d.Prerequisites}
		}
		e := &Entry{
			ID: id, Kind: d.Kind, Model: model, Initial: initial, Implemented: err == nil,
			RewardUsable: rewardUsable(d), GrowUsable: true,
			MinimumLevel: d.MinimumLevel, MaximumLevel: d.MaximumLevel,
			Jobs: d.Jobs, Prerequisites: d.Prerequisites, PrerequisiteGroups: groups,
		}
		for _, g := range cells(d.Script.Cells, "[grow type]") {
			if g.Type != 0 {
				e.GrowUsable = false
				break
			}
			e.GrowTypes = append(e.GrowTypes, g.Value)
		}
		switch {
		case !e.Implemented:
		case model == SingleClearMap:
			m := uint32(d.ObjectiveCells[0].Value)
			if id < 65535 && id != 0 {
				x.ByClearMap[m] = append(x.ByClearMap[m], uint16(id))
			}
		case model == SingleMeetNPC:
			e.NPC = uint32(d.ObjectiveCells[0].Value)
			x.Positional = append(x.Positional, id)
		case model == SingleReachRange:
			e.Range, _ = ReachRange(d)
			x.Positional = append(x.Positional, id)
		case model == ReachNPC:
			e.NPCReach, _ = ReachNPCObjective(d)
			e.NPC = e.NPCReach.NPC
			x.Positional = append(x.Positional, id)
		case model == SeekAndMeetNPC:
			e.Seek, _ = SeekMeet(d)
			e.NPC = e.Seek.NPC
			x.Positional = append(x.Positional, id)
		case model == SingleHuntEnemy:
			e.HuntDungeon, e.HuntEnemy, _ = HuntEnemyObjective(d)
		}
		x.Entries[id] = e
		x.Ordered = append(x.Ordered, id)
	}
	sort.Slice(x.Ordered, func(i, j int) bool { return x.Ordered[i] < x.Ordered[j] })
	sort.Slice(x.Positional, func(i, j int) bool { return x.Positional[i] < x.Positional[j] })
	for m := range x.ByClearMap {
		sort.Slice(x.ByClearMap[m], func(i, j int) bool { return x.ByClearMap[m][i] < x.ByClearMap[m][j] })
	}
	return x
}

// Index returns the service's lazily built quest index.
func (s *Service) Index() *Index {
	if s.index == nil || s.index.Source != s.Catalog.Source.Checksum {
		s.index = BuildIndex(s.Catalog)
	}
	return s.index
}
