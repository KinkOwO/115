package character

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// 14563eee3 extracts growtype from the low nibble and stage from bits4..6.
func (s State) WireAdvancement() (byte, error) {
	if s.Advancement > 15 || s.Awakening > 3 || s.Awakening != 0 && s.Advancement == 0 {
		return 0, fmt.Errorf("invalid advancement/stage")
	}
	return s.Advancement | s.Awakening<<4, nil
}

// Awakening caps are stage-major matrices, including the unadvanced column.
func (d LearningDefinition) ForAwakening(adv, stage int) bool {
	base, caps := d.Ints("[growtype maximum level]"), d.Ints("[awakening maximum level]")
	if stage < 1 || stage > 3 || adv < 1 || adv >= len(base) || len(caps) != 3*len(base) {
		return false
	}
	return caps[(stage-1)*len(base)+adv] > 0
}

// Preserve the shared definition while selecting this character's source cap.
func (d LearningDefinition) forState(state State) LearningDefinition {
	if !d.ForAwakening(int(state.Advancement), int(state.Awakening)) {
		return d
	}
	copyFields := map[string][]pvf.Token{}
	for k, v := range d.Fields {
		copyFields[k] = v
	}
	base := append([]pvf.Token(nil), d.Fields["[growtype maximum level]"]...)
	base[state.Advancement].Value = int32(d.Ints("[awakening maximum level]")[(int(state.Awakening)-1)*len(base)+int(state.Advancement)])
	copyFields["[growtype maximum level]"] = base
	copyFields["[skill fitness growtype]"] = []pvf.Token{{Type: 0, Value: int32(state.Advancement)}}
	d.Fields = copyFields
	return d
}

func (d LearningDefinition) costForState(state State, target int, known map[uint16]byte) (int, error) {
	awakened := d.ForAwakening(int(state.Advancement), int(state.Awakening))
	d = d.forState(state)
	if cost := d.Ints("[purchase cost]"); awakened && len(cost) == 2 {
		index := 0
		if target > 1 {
			index = 1
		}
		d.Fields["[purchase cost]"] = []pvf.Token{{Type: 0, Value: int32(cost[index])}}
	}
	return d.Cost(int(state.Level), int(state.Advancement), target, known)
}

func (s *Service) ApplyAwakening(role storage.Character, stage byte) (json.RawMessage, error) {
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	if _, err := state.WireAdvancement(); err != nil {
		return nil, err
	}
	if stage < 1 || stage > 3 || stage > state.Awakening+1 || state.Advancement == 0 {
		return nil, fmt.Errorf("awakening must progress sequentially")
	}
	if stage <= state.Awakening {
		return role.State, nil
	}
	if state.Level < [4]byte{0, 50, 75, 100}[stage] {
		return nil, fmt.Errorf("awakening level requirement not met")
	}
	prof, ok := s.Catalog.Professions[role.Profession]
	if !ok || prof.RawSHA256 != state.SourceSHA256 || role.ConfigVersion != s.Catalog.Source.Checksum {
		return nil, fmt.Errorf("awakening source mismatch")
	}
	grants := prof.AwakeningSkills[state.Advancement][stage]
	if len(grants) == 0 || len(grants)%2 != 0 || s.Learning == nil {
		return nil, fmt.Errorf("source awakening skills missing")
	}
	for i := 0; i < len(grants); i += 2 {
		id, rank := grants[i], grants[i+1]
		d, ok := s.Learning.index[role.Profession][uint16(id)]
		// Membership in the profession's .chr awakening block authorizes this
		// grant; awakened skills deliberately have zero base-growtype caps.
		if id <= 0 || id > 65535 || rank <= 0 || rank > 255 || !ok {
			return nil, fmt.Errorf("invalid awakening skill grant")
		}
		// Source grants may name an 85-level skill during second awakening.
		// Instantiate it unlearned until the normal level gate is satisfied.
		levels := d.Ints("[required level]")
		if len(levels) != 1 {
			return nil, fmt.Errorf("awakening skill level missing")
		}
		if int(state.Level) < levels[0] {
			continue
		}
		for tree := range state.LearnedSkills {
			if state.LearnedSkills[tree] == nil {
				state.LearnedSkills[tree] = map[uint16]byte{}
			}
			if state.LearnedSkills[tree][uint16(id)] < byte(rank) {
				state.LearnedSkills[tree][uint16(id)] = byte(rank)
			}
		}
	}
	state.Awakening = stage
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &fields); err != nil {
		return nil, err
	}
	fields["awakening"], _ = json.Marshal(state.Awakening)
	fields["learned_skills"], _ = json.Marshal(state.LearnedSkills)
	return json.Marshal(fields)
}
