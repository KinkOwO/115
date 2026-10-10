package character

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
)

// 14563eee3 extracts growtype from the low nibble and stage from bits4..6.
// Branchless professions keep advancement 0 after awakening. Eligibility is
// checked by ApplyAwakening against the profession's growtype-0 source grants.
func (s State) WireAdvancement() (byte, error) {
	if s.Advancement > 15 || s.Awakening > 3 {
		return 0, fmt.Errorf("invalid advancement/stage")
	}
	return s.Advancement | s.Awakening<<4, nil
}

// awakeningColumns is the number of growtype columns the awakening matrix is
// laid out with (matrix length = 3 stages × columns).
//
// Some current-source skills carry no [growtype maximum level] at all and only
// an awakening matrix — skill/swordman/nachal.skl is that shape:
//
//	[skill class] 4
//	[maximum level] 50
//	[awakening maximum level] 0 0 0 0 0 40  0 0 0 0 0 40  0 0 0 0 0 40
//
// Requiring len(caps) == 3*len(base) made every one of them impossible to learn
// (live, 2026-09-21: they surface as "unsupported skill advancement/level").
// The column count is therefore derived from the matrix itself when the base row
// is absent; a base row that is present but inconsistent still refuses, so a
// malformed block never becomes learnable by accident.
func (d LearningDefinition) awakeningColumns() int {
	base, caps := d.Ints("[growtype maximum level]"), d.Ints("[awakening maximum level]")
	switch {
	case len(base) > 0 && len(caps) == 3*len(base):
		return len(base)
	case len(base) == 0 && len(caps) > 0 && len(caps)%3 == 0:
		return len(caps) / 3
	}
	return 0
}

// Awakening caps are stage-major matrices, including the unadvanced column.
// Growtype 0 is a real column, not a placeholder: a profession without a
// change-of-job branch (demonic swordman, creator mage) keeps growtype 0 and
// carries its awakening stages there, and the source matrices put their caps in
// that column. Every other profession's growtype 0 column is all zero, so
// opening adv 0 adds nothing for them.
func (d LearningDefinition) ForAwakening(adv, stage int) bool {
	caps := d.Ints("[awakening maximum level]")
	cols := d.awakeningColumns()
	if cols == 0 || stage < 1 || stage > 3 || adv < 0 || adv >= cols {
		return false
	}
	return caps[(stage-1)*cols+adv] > 0
}

// Preserve the shared definition while selecting this character's source cap.
func (d LearningDefinition) forState(state State) LearningDefinition {
	if !d.ForAwakening(int(state.Advancement), int(state.Awakening)) {
		return d
	}
	cols := d.awakeningColumns()
	copyFields := map[string][]pvf.Token{}
	for k, v := range d.Fields {
		copyFields[k] = v
	}
	base := append([]pvf.Token(nil), d.Fields["[growtype maximum level]"]...)
	// A skill that only carries an awakening matrix has no base row to write
	// into: build the missing columns as zeros, so every other growtype stays
	// refused and only this character's (stage, growtype) cap is selected.
	for len(base) < cols {
		base = append(base, pvf.Token{})
	}
	base[state.Advancement].Value = int32(d.Ints("[awakening maximum level]")[(int(state.Awakening)-1)*cols+int(state.Advancement)])
	copyFields["[growtype maximum level]"] = base
	copyFields["[skill fitness growtype]"] = []pvf.Token{{Type: 0, Value: int32(state.Advancement)}}
	d.Fields = copyFields
	return d
}

func (d LearningDefinition) costForLevel(state State, level int, target int, known map[uint16]byte) (int, error) {
	awakened := d.ForAwakening(int(state.Advancement), int(state.Awakening))
	d = d.forState(state)
	if cost := d.Ints("[purchase cost]"); awakened && len(cost) == 2 {
		index := 0
		if target > 1 {
			index = 1
		}
		d.Fields["[purchase cost]"] = []pvf.Token{{Type: 0, Value: int32(cost[index])}}
	}
	return d.Cost(level, int(state.Advancement), target, known)
}

func (d LearningDefinition) costForState(state State, target int, known map[uint16]byte) (int, error) {
	return d.costForLevel(state, int(state.Level), target, known)
}

func (s *Service) ApplyAwakening(role Character, stage byte) (json.RawMessage, error) {
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	if _, err := state.WireAdvancement(); err != nil {
		return nil, err
	}
	// The sole [growtype 1] block of demonic swordman and creator mage
	// carries [awakening 1..3]. Absence of branches alone is insufficient:
	// require source grants in column 0 so an incomplete catalog stays refused.
	// CMD2177 stage 1 at advancement 0 was captured in the 2026-09-26 handoff.
	branchlessAwakening := false
	if prof, ok := s.Catalog.Professions[role.Profession]; ok {
		branchlessAwakening = len(prof.AdvancementGrowth) == 0 && len(prof.AwakeningSkills[0]) > 0
	}
	if stage < 1 || stage > 3 || stage > state.Awakening+1 || (state.Advancement == 0 && !branchlessAwakening) {
		return nil, fmt.Errorf("awakening must progress sequentially")
	}
	if stage <= state.Awakening {
		return role.State, nil
	}
	if state.Level < [4]byte{0, 50, 75, 100}[stage] {
		return nil, fmt.Errorf("awakening level requirement not met")
	}
	prof, ok := s.Catalog.Professions[role.Profession]
	if !ok || prof.RawSHA256 != state.SourceSHA256 || role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return nil, fmt.Errorf("awakening source mismatch")
	}
	grants := prof.AwakeningSkills[state.Advancement][stage]
	if len(grants) == 0 || len(grants)%2 != 0 || s.Learning == nil {
		return nil, fmt.Errorf("source awakening skills missing")
	}
	for i := 0; i < len(grants); i += 2 {
		id, rank := grants[i], grants[i+1]
		d, ok, sourceErr := s.Learning.Definition(role.Profession, uint16(id))
		if sourceErr != nil {
			return nil, sourceErr
		}
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
	// The third awakening materializes the Skill Evolve/Enhance pool: a fixed
	// 5-point Evolve ledger and the fixed slot layout (3 Enhance rows + 5
	// Evolve rows). Without this the panel reads 0 and a later ordinary Learn
	// response (id29 with an empty persisted block) clears it again.
	if stage == 3 {
		// 三觉给的 VP 点与槽位**两页都要** —— 技能页 2 是独立的第二套加点，
		// 但它同样在三觉后拥有进化/突破（业主 2026-10-09 20:32："第二页没有 vp 了"）。
		// 原来只写 TechniquePoints[0]，第二页的点数永远是 0 ⇒ 面板全空。
		for i := range state.TechniquePoints {
			state.TechniquePoints[i] = 5
		}
		for i := range state.SkillVariations {
			fillVariationSlots(&state.SkillVariations[i])
		}
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &fields); err != nil {
		return nil, err
	}
	fields["awakening"], _ = json.Marshal(state.Awakening)
	fields["learned_skills"], _ = json.Marshal(state.LearnedSkills)
	if stage == 3 {
		fields["technique_points"], _ = json.Marshal(state.TechniquePoints)
		fields["skill_variations"], _ = json.Marshal(state.SkillVariations)
	}
	return json.Marshal(fields)
}
