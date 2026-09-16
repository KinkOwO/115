package character

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// EntryAddition uses persisted source attributes and initial skills in native
// wire units. Equipment and advancement-specific skill learning are separate.
func (s *Service) EntryAddition(role storage.Character) ([]byte, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	if state.SourceSHA256 == "" {
		return nil, fmt.Errorf("missing source character data")
	}
	v := state.Attributes
	var failure error
	scaled := func(name string, multiplier, max float64) uint32 {
		n := float64(v[name]) * multiplier
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > max {
			failure = fmt.Errorf("attribute %s out of native bounds", name)
			return 0
		}
		return uint32(math.Round(n))
	}
	signed := func(name string, multiplier float64) int16 {
		n := float64(v[name]) * multiplier
		if math.IsNaN(n) || math.IsInf(n, 0) || n < math.MinInt16 || n > math.MaxInt16 {
			failure = fmt.Errorf("attribute %s out of signed native bounds", name)
			return 0
		}
		return int16(math.Round(n))
	}
	stats := protocol.PackedEntryStats{
		HP: scaled("[hp max]", 10, math.MaxUint32), MP: scaled("[mp max]", 10, math.MaxUint32),
		Core: [4]uint16{uint16(scaled("[physical attack]", 10, math.MaxUint16)), uint16(scaled("[physical defense]", 10, math.MaxUint16)), uint16(scaled("[magical attack]", 10, math.MaxUint16)), uint16(scaled("[magical defense]", 10, math.MaxUint16))},
		// Native 145d2e760 divides these wire values by ten before applying
		// actor attributes. Source parser 147557e90 scales all input by ten,
		// including MP regeneration, whose effective runtime unit differs.
		Element:       [4]int16{signed("[fire resistance]", 10), signed("[water resistance]", 10), signed("[dark resistance]", 10), signed("[light resistance]", 10)},
		Inventory:     int32(scaled("[inventory limit]", 10, math.MaxInt32)),
		Regeneration:  [2]int16{signed("[hp regen speed]", 10), signed("[mp regen speed]", 10)},
		Movement:      scaled("[move speed]", 10, math.MaxUint32),
		AttackCasting: [2]uint16{uint16(scaled("[attack speed]", 10, math.MaxUint16)), uint16(scaled("[cast speed]", 10, math.MaxUint16))},
		RecoveryJump:  [2]int16{signed("[hit recovery]", 10), signed("[jump power]", 10)},
		Weight:        int32(scaled("[weight]", 10, math.MaxInt32)), BasePercent: 100,
	}
	if failure != nil {
		return nil, failure
	}
	var trees [2][]protocol.EntrySkill
	for i := range trees {
		known, e := knownSkills(state, i)
		if e != nil {
			return nil, e
		}
		ids := skillOrder(state, known)
		for _, id := range ids {
			trees[i] = append(trees[i], protocol.EntrySkill{ID: uint16(id), Level: known[uint16(id)]})
		}
	}
	return protocol.UserInfoAdditionProbe(protocol.EntryAdditionProbe{ActorServerID: role.WireID, Experience: state.Experience, Stats: stats, SkillTrees: trees})
}

// The exact .chr loader at 147559d80 stores (ID, first value) in the
// initial-skill vector and (ID, second value) in a separate vector. All 17
// imported professions use second value=1 in their initial section. Preserve
// that supported shape; do not interpret arbitrary growth/PvP conditions.
func initialSkills(s State) ([]protocol.EntrySkill, error) {
	if s.Advancement != 0 || len(s.InitialSkills)%3 != 0 {
		return nil, fmt.Errorf("unsupported initial skill state")
	}
	var out []protocol.EntrySkill
	for i := 0; i < len(s.InitialSkills); i += 3 {
		id, level, condition := s.InitialSkills[i], s.InitialSkills[i+1], s.InitialSkills[i+2]
		if id <= 0 || id > 65535 || level <= 0 || level > 255 || condition != 1 {
			return nil, fmt.Errorf("unsupported source initial skill tuple")
		}
		out = append(out, protocol.EntrySkill{ID: uint16(id), Level: byte(level)})
	}
	return out, nil
}

func (s *Service) EntrySkills(role storage.Character) ([]byte, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	var trees [2]protocol.SkillTree
	for i := range trees {
		rows, e := s.skillRows(role, state, i)
		if e != nil {
			return nil, e
		}
		if s.Learning != nil {
			// Instantiate own-job skill definitions for the learning window.
			// Zero ranks are unlearned and never occupy a shortcut or grant use.
			seen := map[uint16]bool{}
			occupied := map[uint16]bool{}
			for _, r := range rows {
				seen[r.ID] = true
				occupied[r.Slot] = true
			}
			var missing []int
			for id, d := range s.Learning.index[role.Profession] {
				if !seen[id] && d.ForAdvancement(int(state.Advancement)) {
					missing = append(missing, int(id))
				}
			}
			sort.Ints(missing)
			for _, id := range missing {
				slot := uint16(14)
				for slot < 255 && occupied[slot] {
					slot++
				}
				if slot == 255 {
					return nil, fmt.Errorf("unlearned base-profession palette exceeds current wire slots")
				}
				occupied[slot] = true
				rows = append(rows, protocol.LearnedSkill{ID: uint16(id), Slot: slot})
			}
		}
		trees[i] = protocol.SkillTree{SP: state.SkillPoints[i], TP: state.TechniquePoints[i], Skills: rows}
	}
	return protocol.SkillInfoTrees(state.Level, trees)
}
